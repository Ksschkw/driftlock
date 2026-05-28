package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Ksschkw/driftlock/internal/hook"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check for documentation drift without fixing",
	Long:  `Runs the same checks as the pre-commit hook but does not modify any files. Exits with non-zero if docs are out of sync.`,
	RunE:  runCheck,
}

func init() {
	rootCmd.AddCommand(checkCmd)
}

func runCheck(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	dryRun := true
	if err := hook.RunWithOptions(ctx, dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "driftlock: %v\n", err)
		os.Exit(1)
	}
	return nil
}