package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Ksschkw/driftlock/internal/hook"
	"github.com/spf13/cobra"
)

var fixCmd = &cobra.Command{
	Use:   "fix",
	Short: "Force regeneration of documentation from current code",
	Long:  `Scans staged changes and forcefully updates all linked documentation files, regardless of the check result.`,
	RunE:  runFix,
}

func init() {
	rootCmd.AddCommand(fixCmd)
}

func runFix(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	if err := hook.FixAll(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "driftlock: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("All documents have been regenerated. Review and stage them.")
	return nil
}