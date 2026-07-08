package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Ksschkw/driftlock/internal/hook"
	"github.com/spf13/cobra"
)

var (
	checkBase   string
	checkHead   string
	checkReport bool
	checkJSON   bool
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check for documentation drift without fixing",
	Long: `Runs the same checks as the pre-commit hook but never modifies files.

Default (local): checks the staged index, exactly like the pre-commit hook.

CI mode: pass --base (and optionally --head) to check the changes between two
git refs instead of the staging index. This is how Driftlock runs against a
pull request, e.g.:

    driftlock check --base origin/main --head HEAD

Exits non-zero when documentation is out of sync (unless --report is set),
making it suitable as a required CI status check.`,
	RunE: runCheck,
}

func init() {
	checkCmd.Flags().StringVar(&checkBase, "base", "", "Base git ref to compare against (enables CI range mode)")
	checkCmd.Flags().StringVar(&checkHead, "head", "", "Head git ref (defaults to HEAD; only used with --base)")
	checkCmd.Flags().BoolVar(&checkReport, "report", false, "Report drift without failing (exit 0 even on drift)")
	checkCmd.Flags().BoolVar(&checkJSON, "json", false, "Emit a machine-readable JSON report to stdout")
	rootCmd.AddCommand(checkCmd)
}

func runCheck(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	opts := hook.Options{
		DryRun:  true,
		NoFix:   true,
		BaseRef: checkBase,
		HeadRef: checkHead,
		Report:  checkReport,
		JSON:    checkJSON,
	}
	if err := hook.RunWith(ctx, opts); err != nil {
		if !checkJSON {
			fmt.Fprintf(os.Stderr, "driftlock: %v\n", err)
		}
		os.Exit(1)
	}
	return nil
}
