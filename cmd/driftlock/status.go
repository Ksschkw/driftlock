package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Ksschkw/driftlock/internal/hook"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show documentation coverage status for the entire repository",
	Long: `Scans all tracked source files that match the doc_mapping, extracts
every function signature, and checks whether the linked documentation covers them.

This command does not modify any files and exits with code 1 if drift is found.`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	if err := hook.StatusCheck(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "driftlock: %v\n", err)
		os.Exit(1)
	}
	return nil
}
