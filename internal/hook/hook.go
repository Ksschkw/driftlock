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

// Run is the main entry point for the pre-commit hook.
func Run(ctx context.Context) error {
	return RunWithOptions(ctx, false)
}

func RunWithOptions(ctx context.Context, dryRun bool) error {
	cfg, err := config.LoadProjectConfig()
	if err != nil {
		return err
	}

	files, err := git.ListStagedFiles()
	if err != nil {
		return fmt.Errorf("failed to list staged files: %w", err)
	}
	if len(files) == 0 {
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

		docFullPath := filepath.Join(root, docPath)
		docContent, err := os.ReadFile(docFullPath)
		if err != nil {
			fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("warning: could not read doc %s: %v\n", docPath, err)))
			continue
		}
		diffText := diff.FormatStructuralChanges(allChanges)
		diffForLLM := diffText
		if cfg.Behavior.IncludeFullDiff && fullDiff != "" {
			diffForLLM = fullDiff
		}

		result := checkDocWithRetry(ctx, provider, cfg.Behavior.MaxRetries, diffForLLM, string(docContent))
		if result.err != nil {
			anyLLMError = true
			fmt.Fprint(os.Stderr, output.YellowStr(fmt.Sprintf("driftlock: %s → LLM error: %v\n", docPath, result.err)))
			continue
		}

		cleanExplanation := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(result.explanation), "TRUE. "), "FALSE. ")
		if result.ok {
			fmt.Fprint(os.Stderr, output.GreenStr(fmt.Sprintf("driftlock: %s → up to date (%s)\n", docPath, cleanExplanation)))
		} else {
			fmt.Fprint(os.Stderr, output.RedStr(fmt.Sprintf("driftlock: %s → outdated (%s)\n", docPath, cleanExplanation)))
			anyOutOfSync = true
		}

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

	// ----- truthful summary -----
	if anyLLMError {
		fmt.Fprint(os.Stderr, output.YellowStr("\ndriftlock: Some documentation checks could not be completed due to LLM errors.\n"))
	} else if anyOutOfSync {
		// Already printed per-doc; the block message will be printed below.
	} else if anyStructuralChanges {
		fmt.Fprint(os.Stderr, output.GreenStr("\ndriftlock: All documentation matches the latest structural changes.\n"))
	} else {
		fmt.Fprint(os.Stderr, output.GreenStr("\ndriftlock: No structural changes in mapped sources; documentation check skipped.\n"))
	}

	if anyOutOfSync && cfg.Behavior.BlockOnFalse && !dryRun {
		fmt.Fprint(os.Stderr, output.RedStr("\nCommit blocked: documentation out of sync. Review the updated files and stage them.\n"))
		os.Exit(1)
	}
	if anyOutOfSync && dryRun {
		return fmt.Errorf("documentation drift detected")
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
