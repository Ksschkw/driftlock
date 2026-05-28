package hook

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Ksschkw/driftlock/internal/audit"
	"github.com/Ksschkw/driftlock/internal/config"
	"github.com/Ksschkw/driftlock/internal/diff"
	"github.com/Ksschkw/driftlock/internal/git"
	"github.com/Ksschkw/driftlock/internal/llm"
	"github.com/Ksschkw/driftlock/internal/updater"
)

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
	docMap, err := config.ResolveDocMapping(cfg.DocMapping, files)
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

	root, _ := config.FindProjectRoot()
	anyOutOfSync := false

	// 4. For each doc, gather structural changes from all mapped sources
	for docPath, sourceFiles := range docMap {
		var allChanges []diff.StructuralChange
		for _, src := range sourceFiles {
			// Only proceed if src is staged
			if !contains(files, src) {
				continue
			}
			oldContent, err := git.GetFileContentAtHEAD(src)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not read old version of %s: %v\n", src, err)
				continue
			}
			newContent, err := git.GetStagedFileContent(src)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not read staged version of %s: %v\n", src, err)
				continue
			}
			changes := diff.ExtractStructuralChanges(src, oldContent, newContent)
			allChanges = append(allChanges, changes...)
		}
		if len(allChanges) == 0 {
			continue
		}

		// Read current documentation
		docContent, err := os.ReadFile(filepath.Join(root, docPath))
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not read doc %s: %v\n", docPath, err)
			continue
		}
		diffText := diff.FormatStructuralChanges(allChanges)
		docText := string(docContent)

		// Check with LLM
		ok, explanation, err := provider.Check(ctx, diffText, docText)
		if err != nil {
			return fmt.Errorf("LLM check failed for %s: %w", docPath, err)
		}
		fmt.Fprintf(os.Stderr, "driftlock: %s => %s\n", docPath, explanation)

		if !ok {
			anyOutOfSync = true
			if cfg.Behavior.AutoFix && !dryRun {
				newDoc, err := provider.Fix(ctx, diffText, docText)
				if err != nil {
					return fmt.Errorf("auto-fix failed for %s: %w", docPath, err)
				}
				if err := updater.WriteDoc(filepath.Join(root, docPath), newDoc); err != nil {
					return fmt.Errorf("failed to write updated doc %s: %w", docPath, err)
				}
				fmt.Fprintf(os.Stderr, "driftlock: %s has been updated to reflect your changes.\n", docPath)
			}
		}

		// Audit hash (log regardless)
		hash := audit.ComputeHash(diffText, docText)
		audit.LogHash(root, hash, sourceFiles)
		if cfg.Audit.Solana {
			if err := audit.SendSolanaAudit(ctx, cfg.Audit, hash); err != nil {
				fmt.Fprintf(os.Stderr, "warning: Solana audit logging failed: %v\n", err)
			}
		}
	}

	if anyOutOfSync {
		if cfg.Behavior.BlockOnFalse && !dryRun {
			fmt.Fprintf(os.Stderr, "\nCommit blocked: documentation out of sync. Review the updated files and stage them.\n")
			os.Exit(1) // ensures pre-commit fails
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
	docMap, err := config.ResolveDocMapping(cfg.DocMapping, files)
	if err != nil {
		return err
	}
	provider, err := llm.NewProvider(cfg.LLM, cfg.LLM.Prompts)
	if err != nil {
		return err
	}
	root, _ := config.FindProjectRoot()
	for docPath, sourceFiles := range docMap {
		var allChanges []diff.StructuralChange
		for _, src := range sourceFiles {
			if !contains(files, src) {
				continue
			}
			oldContent, _ := git.GetFileContentAtHEAD(src)
			newContent, _ := git.GetStagedFileContent(src)
			changes := diff.ExtractStructuralChanges(src, oldContent, newContent)
			allChanges = append(allChanges, changes...)
		}
		if len(allChanges) == 0 {
			continue
		}
		docContent, _ := os.ReadFile(filepath.Join(root, docPath))
		diffText := diff.FormatStructuralChanges(allChanges)
		newDoc, err := provider.Fix(ctx, diffText, string(docContent))
		if err != nil {
			return fmt.Errorf("fix failed for %s: %w", docPath, err)
		}
		if err := updater.WriteDoc(filepath.Join(root, docPath), newDoc); err != nil {
			return err
		}
		fmt.Printf("Updated %s\n", docPath)
	}
	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}