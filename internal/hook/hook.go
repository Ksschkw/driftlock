package hook

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Ksschkw/driftlock/internal/audit"
	"github.com/Ksschkw/driftlock/internal/config"
	"github.com/Ksschkw/driftlock/internal/diff"
	"github.com/Ksschkw/driftlock/internal/docman"
	"github.com/Ksschkw/driftlock/internal/git"
	"github.com/Ksschkw/driftlock/internal/llm"
	"github.com/Ksschkw/driftlock/internal/llm/types"
	"github.com/Ksschkw/driftlock/internal/output"
	"github.com/Ksschkw/driftlock/internal/updater"
)

type checkResult struct {
	ok          bool
	explanation string
	err         error
}

func Run(ctx context.Context) error {
	return RunWithOptions(ctx, false, false)
}

func RunWithOptions(ctx context.Context, dryRun bool, noFix bool) error {
	if os.Getenv("DRIFTLOCK_SKIP") == "true" {
		return nil
	}
	cfg, err := config.LoadProjectConfig()
	if err != nil {
		return err
	}

	files, err := git.ListStagedFiles()
	if err != nil {
		return fmt.Errorf("failed to list staged files: %w", err)
	}
	if len(files) == 0 {
		fmt.Fprint(os.Stderr, output.GreenStr("driftlock: No files staged for commit. Nothing to check.\n"))
		return nil
	}

	root, err := config.FindProjectRoot()
	if err != nil {
		return err
	}
	docMap, err := config.ResolveDocMapping(cfg.DocMapping, files, root)
	if err != nil {
		return fmt.Errorf("failed to resolve doc mapping: %w", err)
	}
	if len(docMap) == 0 {
		fmt.Fprint(os.Stderr, output.GreenStr("driftlock: Staged files do not match any doc_mapping sources. Nothing to check.\n"))
		return nil
	}

	provider, err := llm.NewProvider(cfg.LLM, cfg.LLM.Prompts)
	if err != nil {
		return fmt.Errorf("failed to create LLM provider: %w", err)
	}

	var fullDiff string
	if cfg.Behavior.IncludeFullDiff {
		fullDiff, _ = git.GetStagedDiff()
	}

	anyStructuralChanges := false
	anyOutOfSync := false
	anyLLMError := false

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
			changes := diff.ExtractStructuralChanges(src, oldContent, newContent)
			allChanges = append(allChanges, changes...)
		}
		if len(allChanges) == 0 {
			continue
		}
		anyStructuralChanges = true

		// Extract only public‑API names for doc chunking.
		changedNames := []string{}
		for _, ch := range allChanges {
			sig := ch.NewSig
			if sig == "" {
				sig = ch.OldSig
			}
			name := extractNameFromSignature(sig)
			if name != "" {
				changedNames = append(changedNames, name)
			}
		}

		docFullPath := filepath.Join(root, docPath)
		fullDocBytes, err := os.ReadFile(docFullPath)
		if err != nil {
			fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("warning: could not read doc %s: %v\n", docPath, err)))
			continue
		}
		fullDoc := string(fullDocBytes)

		diffText := diff.FormatStructuralChanges(allChanges)
		diffForLLM := diffText
		if cfg.Behavior.IncludeFullDiff && fullDiff != "" {
			diffForLLM = fullDiff
		}

		// Chunk the documentation using only the public‑API names.
		chunkedDoc := docman.ExtractRelevantSections(fullDoc, changedNames)
		if os.Getenv("DRIFTLOCK_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG] chunked doc length: %d bytes (full doc: %d)\n", len(chunkedDoc), len(fullDoc))
		}

		// Use the chunked doc for the check.
		result := checkDocWithRetry(ctx, provider, cfg.Behavior.MaxRetries, diffForLLM, chunkedDoc)
		if result.err != nil {
			anyLLMError = true
			fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("driftlock: %s → LLM error: %v\n", docPath, result.err)))
			continue
		}

		cleanExplanation := strings.TrimPrefix(
			strings.TrimPrefix(strings.TrimSpace(result.explanation), "TRUE. "),
			"FALSE. ",
		)

		if result.ok {
			fmt.Fprint(os.Stderr, output.GreenStr(fmt.Sprintf("driftlock: %s → up to date (%s)\n", docPath, cleanExplanation)))
		} else {
			fmt.Fprint(os.Stderr, output.RedStr(fmt.Sprintf("driftlock: %s → outdated (%s)\n", docPath, cleanExplanation)))
			anyOutOfSync = true
		}

		// Auto‑fix uses the CHUNKED doc, then stitches the result back
		// into the full document. This keeps token usage minimal.
		if !result.ok && cfg.Behavior.AutoFix && !dryRun && !noFix {
			updatedSections, err := provider.Fix(ctx, diffForLLM, chunkedDoc)
			if err != nil {
				fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("auto-fix failed for %s: %v\n", docPath, err)))
				continue
			}
			newFullDoc := docman.MergeSectionUpdates(fullDoc, updatedSections)
			if err := updater.WriteDoc(docFullPath, newFullDoc); err != nil {
				fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("failed to write updated doc %s: %v\n", docPath, err)))
				continue
			}
			fmt.Fprint(os.Stderr, output.BoldStr(fmt.Sprintf("driftlock: %s has been updated to reflect your changes.\n", docPath)))
		}

		hash := audit.ComputeHash(diffText, fullDoc)
		if err := audit.LogHash(root, hash, sourceFiles); err != nil {
			fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("warning: audit logging failed: %v\n", err)))
		}
		if cfg.Audit.Solana {
			if err := audit.SendSolanaAudit(ctx, cfg.Audit, hash); err != nil {
				fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("warning: Solana audit logging failed: %v\n", err)))
			}
		}
	}

	// ---- final summary ----
	if anyLLMError && cfg.Behavior.BlockOnLLMError && !dryRun {
		fmt.Fprint(os.Stderr, output.RedStr("\nCommit blocked: LLM check failed and block_on_llm_error is enabled.\n"))
		os.Exit(1)
	}
	if anyLLMError && !cfg.Behavior.BlockOnLLMError {
		fmt.Fprint(os.Stderr, output.YellowStr("\ndriftlock: Some documentation checks could not be completed due to LLM errors. Review manually.\n"))
	}

	if !anyLLMError || !cfg.Behavior.BlockOnLLMError {
		if anyOutOfSync {
			// per‑doc already printed
		} else if anyStructuralChanges {
			fmt.Fprint(os.Stderr, output.GreenStr("\ndriftlock: All documentation matches the latest structural changes.\n"))
		} else {
			fmt.Fprint(os.Stderr, output.GreenStr("\ndriftlock: No structural changes in mapped sources; documentation check skipped.\n"))
		}
	}

	if anyOutOfSync && cfg.Behavior.BlockOnFalse && !dryRun {
		if noFix {
			fmt.Fprint(os.Stderr, output.RedStr("\nCommit blocked: documentation out of sync. Review the flagged issues and update docs manually.\n"))
		} else {
			fmt.Fprint(os.Stderr, output.RedStr("\nCommit blocked: documentation out of sync. Review the updated files and stage them.\n"))
		}
		os.Exit(1)
	}
	if anyOutOfSync && dryRun {
		return fmt.Errorf("documentation drift detected")
	}
	return nil
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
			changes := diff.ExtractStructuralChanges(src, oldContent, newContent)
			allChanges = append(allChanges, changes...)
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

// extractNameFromSignature extracts the function/method name from a
// signature string.  It only returns names that look like public API
// (exported) to avoid polluting the doc chunking with local variables.
func extractNameFromSignature(sig string) string {
	if !strings.Contains(sig, "(") {
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
			// Only keep if it looks exported (uppercase) or is from a
			// language that doesn't use case to denote public (Python/Ruby).
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
	// Heuristic: if the signature contains 'def ' or 'fn ' (Rust),
	// the name is public unless it starts with '_'.
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
