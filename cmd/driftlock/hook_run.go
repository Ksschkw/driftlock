package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Ksschkw/driftlock/internal/hook"
	"github.com/spf13/cobra"
)

var hookRunCmd = &cobra.Command{
	Use:   "hook-run",
	Short: "Internal command called by the pre-commit hook",
	Long:  `Do not call manually. Used as the pre-commit hook entry point.`,
	RunE:  runHook,
}

func init() {
	rootCmd.AddCommand(hookRunCmd)
}

func runHook(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	if err := hook.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "driftlock: %v\n", err)
		os.Exit(1)
	}
	return nil
}