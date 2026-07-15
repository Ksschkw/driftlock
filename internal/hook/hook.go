package hook

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
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
	if os.Getenv("DRIFTLOCK_SKIP") == "true" {
		return nil
	}

	cfg, err := config.LoadProjectConfig()
	if err != nil {
		return err
	}
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

	provider, err := llm.NewProvider(cfg.LLM, cfg.LLM.Prompts)
	if err != nil {
		return fmt.Errorf("failed to create LLM provider: %w", err)
	}

	verdictCache := cache.Load(root, cfg.Behavior.CacheEnabled())
	defer func() { _ = verdictCache.Save() }()

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
		fullDocBytes, err := os.ReadFile(docFullPath)
		if err != nil {
			if !opts.JSON {
				fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("warning: could not read doc %s: %v\n", docPath, err)))
			}
			continue
		}
		fullDoc := string(fullDocBytes)

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
		if chunkNote != "" {
			checkDoc += "\n\n" + chunkNote
		}
		if os.Getenv("DRIFTLOCK_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG] %s chunked doc: %d bytes (full: %d)\n", docPath, len(chunkedDoc), len(fullDoc))
		}

		dr := DocResult{Doc: docPath, Changes: summarizeChanges(allChanges)}

		// Consult the cache before spending tokens.
		cacheKey := cache.Key(cfg.LLM.Model, diffForLLM, checkDoc)
		var result checkResult
		if cached, ok := verdictCache.Get(cacheKey); ok {
			result = checkResult{ok: cached.OK, explanation: cached.Explanation}
			if os.Getenv("DRIFTLOCK_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "[DEBUG] %s: cache hit\n", docPath)
			}
		} else {
			result = checkDocWithRetry(ctx, provider, cfg.Behavior.MaxRetries, diffForLLM, checkDoc)
			if result.err == nil {
				verdictCache.Set(cacheKey, cache.Entry{OK: result.ok, Explanation: result.explanation})
			}
		}

		if result.err != nil {
			anyLLMError = true
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
			updatedSections, ferr := provider.Fix(ctx, diffForLLM, chunkedDoc)
			if ferr != nil {
				if !opts.JSON {
					fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("auto-fix failed for %s: %v\n", docPath, ferr)))
				}
			} else {
				newFullDoc := docman.MergeSectionUpdates(fullDoc, updatedSections)
				if werr := updater.WriteDoc(docFullPath, newFullDoc); werr != nil {
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

	if opts.JSON {
		printJSON(report)
	} else {
		printTextSummary(anyStructuralChanges, anyOutOfSync, anyLLMError, cfg, dryRun)
	}

	// Report mode is purely informational.
	if opts.Report {
		return nil
	}

	// Blocking decisions.
	if anyLLMError && cfg.Behavior.BlockOnLLMError && !dryRun {
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

func printTextSummary(anyStructuralChanges, anyOutOfSync, anyLLMError bool, cfg *config.Config, dryRun bool) {
	if anyLLMError && !cfg.Behavior.BlockOnLLMError {
		fmt.Fprint(os.Stderr, output.YellowStr("\ndriftlock: Some documentation checks could not be completed due to LLM errors. Review manually.\n"))
		return // a check that errored must not also claim success
	}
	if anyLLMError && cfg.Behavior.BlockOnLLMError && !dryRun {
		fmt.Fprint(os.Stderr, output.RedStr("\nCommit blocked: LLM check failed and block_on_llm_error is enabled.\n"))
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

// checkDocWithRetry calls provider.Check with exponential backoff.
func checkDocWithRetry(ctx context.Context, provider types.Provider, maxRetries int, diff, doc string) checkResult {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			time.Sleep(backoff)
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

// extractNameFromSignature extracts the function/method name from a signature
// string, keeping only names that look like public API to avoid polluting doc
// chunking with local symbols.
func extractNameFromSignature(sig string) string {
	if !strings.Contains(sig, "(") {
		// Type/class declarations have no parens; take the last token.
		fields := strings.Fields(sig)
		if len(fields) >= 2 {
			name := fields[len(fields)-1]
			if isExportedName(name) {
				return name
			}
		}
		return ""
	}
	parts := strings.Fields(sig)
	if len(parts) == 0 {
		return ""
	}
	first := parts[0]
	if first == "func" || first == "def" || first == "fn" || first == "function" ||
		first == "fun" || first == "defn" || first == "class" || first == "struct" ||
		first == "interface" || first == "trait" || first == "enum" {
		if len(parts) >= 2 {
			name := parts[1]
			if idx := strings.IndexByte(name, '('); idx != -1 {
				name = name[:idx]
			}
			if isExportedName(name) || isLowercasePublic(sig) {
				return name
			}
			return ""
		}
	}
	return ""
}

func isExportedName(name string) bool {
	return len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z'
}

func isLowercasePublic(sig string) bool {
	if strings.Contains(sig, "def ") || strings.Contains(sig, "fn ") {
		parts := strings.Fields(sig)
		if len(parts) >= 2 {
			name := parts[1]
			if strings.Contains(name, "(") {
				name = name[:strings.IndexByte(name, '(')]
			}
			return len(name) > 0 && name[0] != '_'
		}
	}
	return false
}
