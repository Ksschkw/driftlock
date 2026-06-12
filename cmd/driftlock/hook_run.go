package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Ksschkw/driftlock/internal/hook"
	"github.com/spf13/cobra"
)

var (
	noFix bool
)

var hookRunCmd = &cobra.Command{
	Use:   "hook-run",
	Short: "Internal command called by the pre-commit hook",
	Long:  `Do not call manually. Used as the pre-commit hook entry point.`,
	RunE:  runHook,
}

func init() {
	hookRunCmd.Flags().BoolVar(&noFix, "no-fix", false, "Block commit if docs are outdated but do not auto‑fix them")
	rootCmd.AddCommand(hookRunCmd)
}

func runHook(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	if err := hook.RunWithOptions(ctx, false, noFix); err != nil {
		fmt.Fprintf(os.Stderr, "driftlock: %v\n", err)
		os.Exit(1)
	}
	return nil
}
