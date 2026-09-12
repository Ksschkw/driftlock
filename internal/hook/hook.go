package hook

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Ksschkw/driftlock/internal/audit"
	"github.com/Ksschkw/driftlock/internal/cache"
	"github.com/Ksschkw/driftlock/internal/config"
	"github.com/Ksschkw/driftlock/internal/diff"
	"github.com/Ksschkw/driftlock/internal/docman"
	"github.com/Ksschkw/driftlock/internal/git"
	"github.com/Ksschkw/driftlock/internal/llm"
	"github.com/Ksschkw/driftlock/internal/llm/types"
	"github.com/Ksschkw/driftlock/internal/output"
	"github.com/Ksschkw/driftlock/internal/parser"
	"github.com/Ksschkw/driftlock/internal/updater"
)

// Options controls a Driftlock run.
type Options struct {
	// DryRun performs the check without writing any files.
	DryRun bool
	// NoFix blocks on drift without auto-fixing, even when auto_fix is enabled.
	NoFix bool
	// BaseRef, when set, switches to range mode: instead of the staged index,
	// Driftlock compares BaseRef..HeadRef. This is how CI runs against a pull
	// request (base = target branch, head = PR tip).
	BaseRef string
	// HeadRef is the newer ref in range mode; defaults to HEAD.
	HeadRef string
	// Report makes the run informational only: drift is reported but the run
	// never exits non-zero. Ideal for gradual adoption.
	Report bool
	// JSON emits a machine-readable report to stdout instead of colored text.
	JSON bool
}

type checkResult struct {
	ok          bool
	explanation string
	err         error
}

// DocResult is the per-document outcome, used for JSON output.
type DocResult struct {
	Doc         string   `json:"doc"`
	Status      string   `json:"status"` // up_to_date | outdated | llm_error | skipped
	Explanation string   `json:"explanation,omitempty"`
	Changes     []string `json:"changes,omitempty"`
	Fixed       bool     `json:"fixed,omitempty"`
}

// Report is the aggregate machine-readable result.
type Report struct {
	Mode    string      `json:"mode"` // staged | range
	Drift   bool        `json:"drift"`
	Results []DocResult `json:"results"`
	// Unparsed lists mapped source files that produced no structural
	// signatures, so a consumer can distinguish "nothing changed" from
	// "we could not read this file".
	Unparsed []string `json:"unparsed,omitempty"`
}

// applyStrictLLMOverrides forces blocking behaviour when DRIFTLOCK_STRICT_LLM
// is set truthy.
//
// CI is the one place where allowing a commit — or a merge — through on an
// unreachable provider is indefensible: the gate exists precisely to make that
// judgement, and a transient outage would silently convert it into a rubber
// stamp. Local commits keep the friendlier default, because blocking a
// developer's work on a flaky network is a worse trade.
func applyStrictLLMOverrides(cfg *config.Config) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DRIFTLOCK_STRICT_LLM"))) {
	case "1", "true", "yes":
		cfg.Behavior.BlockOnLLMError = true
		cfg.Behavior.BlockOnFalse = true
	}
}

// skipRequested reports whether DRIFTLOCK_SKIP asks Driftlock to stand down.
//
// The project's .env is loaded first, so the flag can be configured there as
// well as in the shell. Previously the check ran BEFORE config loading — which
// is where .env was read — so a `.env` containing `DRIFTLOCK_SKIP=true`
// silently did nothing, and the only way to skip a commit was to export the
// variable inline. LoadDotEnv never clobbers a variable already present in the
// real environment, so a shell value still wins.
func skipRequested() bool {
	if root, err := config.FindProjectRoot(); err == nil {
		config.LoadDotEnv(root)
	}
	return os.Getenv("DRIFTLOCK_SKIP") == "true"
}

// Run executes with default (staged, auto-fixing) options.
func Run(ctx context.Context) error {
	return RunWith(ctx, Options{})
}

// RunWithOptions preserves the original three-argument entry point.
func RunWithOptions(ctx context.Context, dryRun, noFix bool) error {
	return RunWith(ctx, Options{DryRun: dryRun, NoFix: noFix})
}

// RunWith is the main pipeline. It resolves changed source files (from the
// staging index or a ref range), maps them to docs, checks each doc against its
// structural changes via the LLM (consulting the verdict cache first), and
// optionally auto-fixes and blocks.
func RunWith(ctx context.Context, opts Options) error {
	if skipRequested() {
		return nil
	}

	cfg, err := config.LoadProjectConfig()
	if err != nil {
		return err
	}
	applyStrictLLMOverrides(cfg)
	root, err := config.FindProjectRoot()
	if err != nil {
		return err
	}

	rangeMode := opts.BaseRef != ""
	mode := "staged"
	if rangeMode {
		mode = "range"
	}

	// Resolve the changed files and content accessors for the active mode.
	files, oldContentOf, newContentOf, err := resolveSources(opts, rangeMode)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		if !opts.JSON {
			fmt.Fprint(os.Stderr, output.GreenStr("driftlock: No changed files to check.\n"))
		} else {
			printJSON(Report{Mode: mode})
		}
		return nil
	}

	docMap, err := config.ResolveDocMapping(cfg.DocMapping, files, root)
	if err != nil {
		return fmt.Errorf("failed to resolve doc mapping: %w", err)
	}
	if len(docMap) == 0 {
		if !opts.JSON {
			fmt.Fprint(os.Stderr, output.GreenStr("driftlock: Changed files do not match any doc_mapping sources. Nothing to check.\n"))
		} else {
			printJSON(Report{Mode: mode})
		}
		return nil
	}

	unparsed := unparsedSources(docMap, files, newContentOf)
	if len(unparsed) > 0 && (cfg.Behavior.ReportUnparsed || os.Getenv("DRIFTLOCK_DEBUG") != "") {
		for _, src := range unparsed {
			if !opts.JSON {
				fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf(
					"driftlock: no structural signatures found in %s; it may use syntax the extractor does not understand. Use driftlock:ignore or narrow doc_mapping if this is expected.\n", src)))
			}
		}
	}

	provider, err := llm.NewProvider(cfg.LLM, cfg.LLM.Prompts)
	if err != nil {
		return fmt.Errorf("failed to create LLM provider: %w", err)
	}

	// NOTE: saved explicitly before every return/exit — os.Exit skips defers,
	// and the blocked-commit path is exactly when fresh verdicts must persist
	// (the retry after staging docs should hit the cache, not re-bill the LLM).
	verdictCache := cache.Load(root, cfg.Behavior.CacheEnabled())

	var fullDiff string
	if cfg.Behavior.IncludeFullDiff {
		if rangeMode {
			fullDiff, _ = git.RangeDiff(opts.BaseRef, opts.HeadRef)
		} else {
			fullDiff, _ = git.GetStagedDiff()
		}
	}

	// In range mode we can never write fixes back into a commit, so force
	// check-only semantics.
	noFix := opts.NoFix || rangeMode
	dryRun := opts.DryRun || rangeMode

	report := Report{Mode: mode}
	anyStructuralChanges := false
	anyOutOfSync := false
	anyLLMError := false
	var llmFailedDocs []string

	for docPath, sourceFiles := range docMap {
		var allChanges []diff.StructuralChange
		for _, src := range sourceFiles {
			if !contains(files, src) {
				continue
			}
			oldContent := oldContentOf(src)
			newContent := newContentOf(src)
			if newContent == "" && oldContent != "" {
				continue // deletion: nothing structural to document
			}
			allChanges = append(allChanges, diff.ExtractStructuralChanges(src, oldContent, newContent)...)
		}
		if len(allChanges) == 0 {
			continue
		}
		anyStructuralChanges = true

		changedNames := publicNames(allChanges)

		docFullPath := filepath.Join(root, docPath)
		fullDoc, err := readDocForCheck(root, docPath, opts.HeadRef, rangeMode)
		if err != nil {
			if !opts.JSON {
				fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("warning: could not read doc %s: %v\n", docPath, err)))
			}
			continue
		}

		diffText := diff.FormatStructuralChanges(allChanges)
		diffForLLM := diffText
		if cfg.Behavior.IncludeFullDiff && fullDiff != "" {
			diffForLLM = fullDiff
		}

		// Chunk the doc to the sections mentioning changed symbols. The note
		// (symbols not documented anywhere) is Driftlock metadata: it goes to
		// the LLM as context but must never be embedded in doc content, or the
		// auto-fix would write it verbatim into the user's documentation.
		chunkedDoc, chunkNote := docman.ExtractRelevantSections(fullDoc, changedNames)
		wholeDoc := chunkedDoc == ""
		if wholeDoc {
			// No section mentions any changed symbol (e.g. all-new API): fall
			// back to the full document so the fix can append new content.
			chunkedDoc = fullDoc
		}
		checkDoc := chunkedDoc
		// The note travels with the DIFF, not the doc: it is change metadata
		// ("these symbols are documented nowhere"), the check needs it to
		// judge, and the fix needs it to know what to add — but it must never
		// be part of the document content or the fix will write it verbatim
		// into the user's docs.
		diffWithNote := diffForLLM
		if chunkNote != "" {
			diffWithNote += "\n\n" + chunkNote
		}
		if os.Getenv("DRIFTLOCK_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG] %s chunked doc: %d bytes (full: %d)\n", docPath, len(chunkedDoc), len(fullDoc))
		}

		dr := DocResult{Doc: docPath, Changes: summarizeChanges(allChanges)}

		// Consult the cache before spending tokens. The check prompt is part
		// of the key: improving the prompt must invalidate verdicts produced by
		// the old one (a nil Prompts means the built-in default is used).
		checkPrompt := types.DefaultPrompts().Check
		if cfg.LLM.Prompts != nil && cfg.LLM.Prompts.Check != "" {
			checkPrompt = cfg.LLM.Prompts.Check
		}
		cacheKey := cache.Key(cfg.LLM.Model, checkPrompt, diffWithNote, checkDoc)
		var result checkResult
		if cached, ok := verdictCache.Get(cacheKey); ok {
			result = checkResult{ok: cached.OK, explanation: cached.Explanation}
			if os.Getenv("DRIFTLOCK_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "[DEBUG] %s: cache hit\n", docPath)
			}
		} else {
			checkCtx, cancelCheck := context.WithTimeout(ctx, perCheckBudget(cfg))
			result = checkDocWithRetry(checkCtx, provider, cfg.Behavior.MaxRetries, diffWithNote, checkDoc)
			cancelCheck()
			if result.err == nil {
				verdictCache.Set(cacheKey, cache.Entry{OK: result.ok, Explanation: result.explanation})
			}
		}

		if result.err != nil {
			anyLLMError = true
			llmFailedDocs = append(llmFailedDocs, docPath)
			dr.Status = "llm_error"
			dr.Explanation = result.err.Error()
			report.Results = append(report.Results, dr)
			if !opts.JSON {
				fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("driftlock: %s → LLM error: %v\n", docPath, result.err)))
			}
			continue
		}

		cleanExplanation := strings.TrimPrefix(
			strings.TrimPrefix(strings.TrimSpace(result.explanation), "TRUE. "),
			"FALSE. ",
		)
		dr.Explanation = cleanExplanation

		if result.ok {
			dr.Status = "up_to_date"
			if !opts.JSON {
				fmt.Fprint(os.Stderr, output.GreenStr(fmt.Sprintf("driftlock: %s → up to date (%s)\n", docPath, cleanExplanation)))
			}
		} else {
			dr.Status = "outdated"
			anyOutOfSync = true
			if !opts.JSON {
				fmt.Fprint(os.Stderr, output.RedStr(fmt.Sprintf("driftlock: %s → outdated (%s)\n", docPath, cleanExplanation)))
			}
		}

		if !result.ok && cfg.Behavior.AutoFix && !dryRun && !noFix && !opts.Report {
			updatedSections, ferr := provider.Fix(ctx, diffWithNote, chunkedDoc)
			if ferr != nil {
				if !opts.JSON {
					fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("auto-fix failed for %s: %v\n", docPath, ferr)))
				}
			} else {
				newFullDoc, changed := mergeFix(fullDoc, updatedSections)
				if !changed {
					// An empty reply, or headings that match nothing, leaves
					// the document untouched. Reporting success here told the
					// user their docs had been updated when they had not, and
					// rewriting the identical content churned the file mtime.
					if !opts.JSON {
						fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("auto-fix produced no change for %s; the docs still need a manual update.\n", docPath)))
					}
				} else if werr := updater.WriteDoc(docFullPath, newFullDoc); werr != nil {
					if !opts.JSON {
						fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("failed to write updated doc %s: %v\n", docPath, werr)))
					}
				} else {
					dr.Fixed = true
					if !opts.JSON {
						fmt.Fprint(os.Stderr, output.BoldStr(fmt.Sprintf("driftlock: %s has been updated to reflect your changes.\n", docPath)))
					}
				}
			}
		}

		report.Results = append(report.Results, dr)

		hash := audit.ComputeHash(diffText, fullDoc)
		if err := audit.LogHash(root, hash, sourceFiles); err != nil && !opts.JSON {
			fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("warning: audit logging failed: %v\n", err)))
		}
		if cfg.Audit.Solana {
			if err := audit.SendSolanaAudit(ctx, cfg.Audit, hash); err != nil && !opts.JSON {
				fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("warning: Solana audit logging failed: %v\n", err)))
			}
		}
	}

	report.Drift = anyOutOfSync
	report.Unparsed = unparsed

	if opts.JSON {
		printJSON(report)
	} else {
		printTextSummary(anyStructuralChanges, anyOutOfSync, anyLLMError, llmFailedDocs, cfg, dryRun)
	}

	if err := verdictCache.Save(); err != nil && !opts.JSON {
		fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("warning: could not save verdict cache: %v\n", err)))
	}

	// Report mode is purely informational.
	if opts.Report {
		return nil
	}

	// Blocking decisions.
	if anyLLMError && cfg.Behavior.BlockOnLLMError {
		if dryRun {
			// `check` (and range mode) must fail loudly rather than exit;
			// otherwise a provider outage silently passes a pull request.
			return fmt.Errorf("documentation check incomplete: the LLM could not be reached")
		}
		os.Exit(1)
	}
	if anyOutOfSync && cfg.Behavior.BlockOnFalse {
		if dryRun {
			return fmt.Errorf("documentation drift detected")
		}
		if noFix {
			fmt.Fprint(os.Stderr, output.RedStr("\nCommit blocked: documentation out of sync. Review the flagged issues and update docs manually.\n"))
		} else {
			fmt.Fprint(os.Stderr, output.RedStr("\nCommit blocked: documentation out of sync. Review the updated files and stage them.\n"))
		}
		os.Exit(1)
	}
	return nil
}

// llmErrorMessage builds the operator-facing message for documents that could
// not be checked. It always states whether the commit went through, because a
// fail-open gate that is quiet is indistinguishable from a pass — the whole
// point of block_on_llm_error=false is that the developer must know their
// commit was NOT verified.
func llmErrorMessage(failedDocs []string, blocked bool) string {
	outcome := "The commit is proceeding UNCHECKED"
	if blocked {
		outcome = "The commit is blocked"
	}
	docs := "unknown"
	if len(failedDocs) > 0 {
		docs = strings.Join(failedDocs, ", ")
	}
	return fmt.Sprintf(
		"driftlock: %d document(s) could not be checked because the LLM failed (%s). %s. "+
			"Set block_on_llm_error = true to fail instead, raise timeout_seconds for a slow provider, "+
			"or run with DRIFTLOCK_DEBUG=1 to see the provider error.",
		len(failedDocs), docs, outcome)
}

// mergeFix applies the model's rewritten sections to the full document and
// reports whether the result actually differs. An empty reply, or a reply whose
// headings match nothing in the document, is a no-op — and a no-op must never
// be announced as "the docs have been updated".
func mergeFix(fullDoc, updatedSections string) (string, bool) {
	merged := docman.MergeSectionUpdates(fullDoc, updatedSections)
	return merged, merged != fullDoc
}

// unparsedSources returns the mapped source files whose new content produced
// no structural signatures at all. Those are the files where Driftlock's
// silence is ambiguous: either nothing changed, or the extractor could not read
// the file. Reporting them makes the silence meaningful.
func unparsedSources(docMap map[string][]string, files []string, newContentOf func(string) string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, sources := range docMap {
		for _, src := range sources {
			if seen[src] || !contains(files, src) {
				continue
			}
			seen[src] = true
			content := newContentOf(src)
			if !looksLikeCode(content) {
				continue
			}
			if len(parser.ExtractSignatures(src, content)) == 0 {
				out = append(out, src)
			}
		}
	}
	sort.Strings(out)
	return out
}

// looksLikeCode is a deliberately loose heuristic that keeps the unparsed-file
// diagnostic from firing on short or data-only files.
func looksLikeCode(content string) bool {
	if len(content) < 20 || !strings.Contains(content, "(") {
		return false
	}
	return strings.Count(content, "\n") >= 2
}

// readDocForCheck returns the documentation content the check should judge.
//
// In staged mode this is the blob in the index — the exact content the commit
// will contain. Reading the working tree instead was a correctness hole: the
// source side of the diff is HEAD-vs-index, so editing a mapped doc without
// staging it made Driftlock validate the edited copy while the commit landed
// the stale one. Driftlock could therefore bless a commit it had never checked.
//
// In range mode the head ref is authoritative. In both modes an absent blob
// (an untracked or newly created doc) falls back to the working tree.
func readDocForCheck(root, docPath, headRef string, rangeMode bool) (string, error) {
	if rangeMode {
		head := headRef
		if head == "" {
			head = "HEAD"
		}
		if content, err := git.GetFileContentAtRefAt(root, head, docPath); err == nil && content != "" {
			return content, nil
		}
	} else if content, err := git.GetStagedFileContentAt(root, docPath); err == nil && content != "" {
		return content, nil
	}

	data, err := os.ReadFile(filepath.Join(root, docPath))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// resolveSources returns the changed files and closures to read their old/new
// content, for either staged or range mode.
func resolveSources(opts Options, rangeMode bool) ([]string, func(string) string, func(string) string, error) {
	if rangeMode {
		files, err := git.ListChangedFilesInRange(opts.BaseRef, opts.HeadRef)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to list changed files in range: %w", err)
		}
		head := opts.HeadRef
		if head == "" {
			head = "HEAD"
		}
		oldOf := func(src string) string { c, _ := git.GetFileContentAtRef(opts.BaseRef, src); return c }
		newOf := func(src string) string { c, _ := git.GetFileContentAtRef(head, src); return c }
		return files, oldOf, newOf, nil
	}

	files, err := git.ListStagedFiles()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to list staged files: %w", err)
	}
	oldOf := func(src string) string { c, _ := git.GetFileContentAtHEAD(src); return c }
	newOf := func(src string) string { c, _ := git.GetStagedFileContent(src); return c }
	return files, oldOf, newOf, nil
}

func printTextSummary(anyStructuralChanges, anyOutOfSync, anyLLMError bool, failedDocs []string, cfg *config.Config, dryRun bool) {
	// A check that errored must never also print an all-clear, in any mode.
	// The previous version only short-circuited when block_on_llm_error was
	// false or when running non-dry-run, so `driftlock check` in CI reported
	// "All documentation matches" after a provider failure — the worst possible
	// message for a gate.
	if anyLLMError {
		msg := "\n" + llmErrorMessage(failedDocs, cfg.Behavior.BlockOnLLMError) + "\n"
		if cfg.Behavior.BlockOnLLMError {
			fmt.Fprint(os.Stderr, output.RedStr(msg))
		} else {
			fmt.Fprint(os.Stderr, output.YellowStr(msg))
		}
		return
	}
	if anyOutOfSync {
		return // per-doc lines already printed
	}
	if anyStructuralChanges {
		fmt.Fprint(os.Stderr, output.GreenStr("\ndriftlock: All documentation matches the latest structural changes.\n"))
	} else {
		fmt.Fprint(os.Stderr, output.GreenStr("\ndriftlock: No structural changes in mapped sources; documentation check skipped.\n"))
	}
}

func printJSON(r Report) {
	b, _ := json.MarshalIndent(r, "", "  ")
	fmt.Fprintln(os.Stdout, string(b))
}

// publicNames extracts public-API symbol names from a set of changes for use in
// documentation chunking.
func publicNames(changes []diff.StructuralChange) []string {
	var names []string
	for _, ch := range changes {
		sig := ch.NewSig
		if sig == "" {
			sig = ch.OldSig
		}
		if name := extractNameFromSignature(sig); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func summarizeChanges(changes []diff.StructuralChange) []string {
	var out []string
	for _, c := range changes {
		sig := c.NewSig
		if sig == "" {
			sig = c.OldSig
		}
		out = append(out, c.Change+": "+sig)
	}
	return out
}

// FixAll forces regeneration of all mapped documentation for staged files.
func FixAll(ctx context.Context) error {
	cfg, err := config.LoadProjectConfig()
	if err != nil {
		return err
	}
	files, err := git.ListStagedFiles()
	if err != nil {
		return err
	}
	root, err := config.FindProjectRoot()
	if err != nil {
		return err
	}
	docMap, err := config.ResolveDocMapping(cfg.DocMapping, files, root)
	if err != nil {
		return err
	}
	provider, err := llm.NewProvider(cfg.LLM, cfg.LLM.Prompts)
	if err != nil {
		return err
	}
	for docPath, sourceFiles := range docMap {
		var allChanges []diff.StructuralChange
		for _, src := range sourceFiles {
			if !contains(files, src) {
				continue
			}
			oldContent, _ := git.GetFileContentAtHEAD(src)
			newContent, _ := git.GetStagedFileContent(src)
			if newContent == "" && oldContent != "" {
				continue
			}
			allChanges = append(allChanges, diff.ExtractStructuralChanges(src, oldContent, newContent)...)
		}
		if len(allChanges) == 0 {
			continue
		}
		docFullPath := filepath.Join(root, docPath)
		docContent, _ := os.ReadFile(docFullPath)
		diffText := diff.FormatStructuralChanges(allChanges)
		newDoc, err := provider.Fix(ctx, diffText, string(docContent))
		if err != nil {
			fmt.Fprint(os.Stderr, output.RedStr(fmt.Sprintf("fix failed for %s: %v\n", docPath, err)))
			continue
		}
		if err := updater.WriteDoc(docFullPath, newDoc); err != nil {
			fmt.Fprint(os.Stderr, output.RedStr(fmt.Sprintf("failed to write updated doc %s: %v\n", docPath, err)))
			continue
		}
		fmt.Print(output.GreenStr(fmt.Sprintf("Updated %s\n", docPath)))
	}
	return nil
}

// perCheckBudget is the total time allowed for one document's check, including
// its retries and inter-attempt backoff. Each individual request is already
// bounded by the HTTP client timeout; this bounds the whole check so a slow
// provider cannot stretch a single document across an unbounded series of
// attempts.
func perCheckBudget(cfg *config.Config) time.Duration {
	attempts := cfg.Behavior.MaxRetries + 1
	if attempts < 1 {
		attempts = 1
	}
	budget := cfg.LLM.HTTPTimeout() * time.Duration(attempts)
	for i := 1; i <= cfg.Behavior.MaxRetries; i++ {
		budget += time.Duration(math.Pow(2, float64(i))) * time.Second
	}
	return budget + time.Second
}

// checkDocWithRetry calls provider.Check with exponential backoff. It honours
// ctx: the backoff wait is interruptible and no attempt is started on a
// cancelled context, so an expired per-check deadline returns promptly instead
// of sleeping out the remaining backoff.
func checkDocWithRetry(ctx context.Context, provider types.Provider, maxRetries int, diff, doc string) checkResult {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return checkResult{err: fmt.Errorf("cancelled after %d attempt(s): %w", attempt, err)}
			}
			return checkResult{err: fmt.Errorf("cancelled before the first attempt: %w", err)}
		}
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			select {
			case <-ctx.Done():
				return checkResult{err: fmt.Errorf("cancelled during backoff: %w", ctx.Err())}
			case <-time.After(backoff):
			}
		}
		ok, explanation, err := provider.Check(ctx, diff, doc)
		if err == nil {
			return checkResult{ok: ok, explanation: explanation}
		}
		lastErr = err
	}
	return checkResult{err: fmt.Errorf("all retries exhausted: %w", lastErr)}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// extractNameFromSignature extracts the symbol name used to find the sections
// of a document that discuss a change.
//
// The previous implementation only recognised a signature whose FIRST token was
// a declaration keyword, so `func`, `def`, and bare `fn` worked but everything
// with a leading modifier did not:
//
//	public int add(int a)        -> "" (Java/C#)
//	pub fn build(&self)          -> "" (Rust)
//	export function makeUser()   -> "" (TypeScript)
//	func (a *A) Close()          -> "" (Go receiver read as the name)
//
// An empty name meant `changedNames` was empty, doc chunking degraded to
// sending the WHOLE document, and the "not documented anywhere" note was wrong.
//
// The name is now the identifier immediately before the parameter list (with a
// Go receiver skipped and trailing generics stripped), or the identifier after
// a type keyword for type declarations.
func extractNameFromSignature(sig string) string {
	sig = strings.TrimSpace(sig)
	if sig == "" {
		return ""
	}

	// Type-like declarations carry the name right after the keyword.
	for i, f := range strings.Fields(sig) {
		switch f {
		case "class", "struct", "interface", "trait", "enum", "object",
			"record", "module", "union", "type":
			fields := strings.Fields(sig)
			if i+1 < len(fields) {
				if name := publicIdent(cleanIdent(fields[i+1])); name != "" {
					return name
				}
			}
		}
	}

	searchFrom := 0
	// A Go method puts its receiver in parentheses before the name:
	// `func (a *A) Close(...)`. Skip that group so the receiver is not read as
	// the name.
	if strings.HasPrefix(sig, "func (") {
		if close := strings.IndexByte(sig, ')'); close >= 0 {
			searchFrom = close + 1
		}
	}
	rel := strings.IndexByte(sig[searchFrom:], '(')
	if rel < 0 {
		return ""
	}
	return publicIdent(lastIdent(sig[searchFrom : searchFrom+rel]))
}

// publicIdent rejects names that are empty, keywords, or private-by-convention
// (a leading underscore). Uppercase-initial names (Go, C#), camelCase names
// (Java, TypeScript) and lowercase names (Python, Rust) are all accepted:
// filtering chunking by case silently disabled it for most languages.
func publicIdent(name string) string {
	if name == "" || strings.HasPrefix(name, "_") || isControlKeyword(name) {
		return ""
	}
	return name
}

// isControlKeyword reports whether a token is a control-flow word that a
// permissive pattern may have captured instead of a real symbol name.
func isControlKeyword(s string) bool {
	switch s {
	case "if", "for", "while", "switch", "return", "else", "catch",
		"do", "match", "when", "with", "case", "select", "defer", "go",
		"function":
		return true
	}
	return false
}

// cleanIdent trims a trailing generic, bracket, paren, or annotation clause
// from an identifier: `Stack[T` -> `Stack`, `ID:` -> `ID`.
func cleanIdent(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "[<(:"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// lastIdent returns the final identifier-like token in s, ignoring a trailing
// generic or array clause (`func Map[T any]` -> `Map`, `List<String> get` ->
// `get`).
func lastIdent(s string) string {
	s = strings.TrimRight(s, " \t")
	for len(s) > 0 {
		last := s[len(s)-1]
		if last != '>' && last != ']' {
			break
		}
		open := byte('<')
		if last == ']' {
			open = '['
		}
		depth := 0
		i := len(s) - 1
		for ; i >= 0; i-- {
			switch s[i] {
			case last:
				depth++
			case open:
				depth--
			}
			if depth == 0 {
				break
			}
		}
		if i <= 0 {
			return ""
		}
		s = strings.TrimRight(s[:i], " \t")
	}
	i := len(s)
	for i > 0 && isIdentByte(s[i-1]) {
		i--
	}
	return s[i:]
}

func isIdentByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}
