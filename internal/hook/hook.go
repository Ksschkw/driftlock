package hook

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/Ksschkw/driftlock/internal/audit"
	"github.com/Ksschkw/driftlock/internal/config"
	"github.com/Ksschkw/driftlock/internal/diff"
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

// Run is the main entry point for the pre-commit hook. It will block and auto-fix.
func Run(ctx context.Context) error {
	return RunWithOptions(ctx, false)
}

// RunWithOptions performs the hook logic. If dryRun is true, it only checks and does not modify files.
func RunWithOptions(ctx context.Context, dryRun bool) error {
	cfg, err := config.LoadProjectConfig()
	if err != nil {
		return err
	}

	// 1. Get staged files
	files, err := git.ListStagedFiles()
	if err != nil {
		return fmt.Errorf("failed to list staged files: %w", err)
	}
	if len(files) == 0 {
		return nil // nothing to check
	}

	// 2. Map files to documentation
	root, err := config.FindProjectRoot()
	if err != nil {
		return err
	}
	docMap, err := config.ResolveDocMapping(cfg.DocMapping, files, root)
	if err != nil {
		return fmt.Errorf("failed to resolve doc mapping: %w", err)
	}
	if len(docMap) == 0 {
		return nil // no doc mapping covers staged files
	}

	// 3. Prepare LLM provider
	provider, err := llm.NewProvider(cfg.LLM, cfg.LLM.Prompts)
	if err != nil {
		return fmt.Errorf("failed to create LLM provider: %w", err)
	}

	// Pre-fetch full diff if needed
	var fullDiff string
	if cfg.Behavior.IncludeFullDiff {
		fullDiff, err = git.GetStagedDiff()
		if err != nil {
			fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("warning: could not get full diff: %v\n", err)))
		}
	}

	anyOutOfSync := false
	anyLLMError := false

	// 4. For each doc, gather structural changes from all mapped sources
	for docPath, sourceFiles := range docMap {
		var allChanges []diff.StructuralChange
		for _, src := range sourceFiles {
			if !contains(files, src) {
				continue
			}
			oldContent, err := git.GetFileContentAtHEAD(src)
			if err != nil {
				fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("warning: could not read old version of %s: %v\n", src, err)))
				continue
			}
			newContent, err := git.GetStagedFileContent(src)
			if err != nil {
				fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("warning: could not read staged version of %s: %v\n", src, err)))
				continue
			}
			// Skip deleted files (newContent will be empty and oldContent may have content, but no actual signature to compare)
			if newContent == "" && oldContent != "" {
				continue
			}
			changes := diff.ExtractStructuralChanges(src, oldContent, newContent)
			allChanges = append(allChanges, changes...)
		}
		if len(allChanges) == 0 {
			continue
		}

		// Read current documentation
		docFullPath := filepath.Join(root, docPath)
		docContent, err := os.ReadFile(docFullPath)
		if err != nil {
			fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("warning: could not read doc %s: %v\n", docPath, err)))
			continue
		}
		diffText := diff.FormatStructuralChanges(allChanges)

		// Choose which diff to send to the LLM
		diffForLLM := diffText
		if cfg.Behavior.IncludeFullDiff && fullDiff != "" {
			diffForLLM = fullDiff
		}

		// LLM check with retries
		result := checkDocWithRetry(ctx, provider, cfg.Behavior.MaxRetries, diffForLLM, string(docContent))

		if result.err != nil {
			anyLLMError = true
			fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("driftlock: %s => LLM error: %v\n", docPath, result.err)))
			continue
		}

		if result.ok {
			fmt.Fprint(os.Stderr, output.GreenStr(fmt.Sprintf("driftlock: %s => TRUE %s\n", docPath, result.explanation)))
		} else {
			fmt.Fprint(os.Stderr, output.RedStr(fmt.Sprintf("driftlock: %s => FALSE %s\n", docPath, result.explanation)))
			anyOutOfSync = true
		}

		// Auto-fix if needed
		if !result.ok && cfg.Behavior.AutoFix && !dryRun {
			newDoc, err := provider.Fix(ctx, diffForLLM, string(docContent))
			if err != nil {
				fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("auto-fix failed for %s: %v\n", docPath, err)))
				continue
			}
			if err := updater.WriteDoc(docFullPath, newDoc); err != nil {
				fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("failed to write updated doc %s: %v\n", docPath, err)))
				continue
			}
			fmt.Fprint(os.Stderr, output.BoldStr(fmt.Sprintf("driftlock: %s has been updated to reflect your changes.\n", docPath)))
		}

		// Audit hash (log regardless)
		hash := audit.ComputeHash(diffText, string(docContent))
		if err := audit.LogHash(root, hash, sourceFiles); err != nil {
			fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("warning: audit logging failed: %v\n", err)))
		}
		if cfg.Audit.Solana {
			if err := audit.SendSolanaAudit(ctx, cfg.Audit, hash); err != nil {
				fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("warning: Solana audit logging failed: %v\n", err)))
			}
		}
	}

	// Final messaging
	// Final messaging
	if anyLLMError {
		fmt.Fprint(os.Stderr, output.YellowStr("\ndriftlock: Some documentation checks could not be completed due to LLM errors. Review manually.\n"))
	} else if !anyOutOfSync && len(docMap) > 0 {
		// We checked docs but found no drift – tell the user.
		fmt.Fprint(os.Stderr, output.GreenStr("driftlock: No structural changes in mapped sources; documentation check skipped.\n"))
	}

	if anyOutOfSync {
		if cfg.Behavior.BlockOnFalse && !dryRun {
			fmt.Fprint(os.Stderr, output.RedStr("\nCommit blocked: documentation out of sync. Review the updated files and stage them.\n"))
			os.Exit(1)
		}
		if dryRun {
			return fmt.Errorf("documentation drift detected")
		}
	}
	return nil
}

// FixAll forces regeneration of all documentation for staged files, bypassing the check.
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

// checkDocWithRetry attempts to check the doc against the LLM with exponential backoff.
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
