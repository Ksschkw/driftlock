package main

import (
	"fmt"
	"os"

	"github.com/Ksschkw/driftlock/internal/hook"
	"github.com/spf13/cobra"
)

var statusStrict bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Report documentation coverage for the whole repository",
	Long: `Scans every tracked source file that matches the doc_mapping, extracts its
public signatures, and reports which ones the mapped documentation never
mentions.

This is deterministic: no LLM is called, so it is instant, free, and
reproducible. It never modifies files.

By default the report is informational and the command exits 0 — a
whole-repository scan of a mature project will always have gaps. Pass --strict
to exit non-zero when any public symbol is undocumented, which is useful as a
periodic CI job rather than a per-commit gate.`,
	RunE: runStatus,
}

func init() {
	statusCmd.Flags().BoolVar(&statusStrict, "strict", false, "Exit non-zero when any public symbol is undocumented")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	if err := hook.StatusCheck(hook.StatusOptions{Strict: statusStrict}); err != nil {
		fmt.Fprintf(os.Stderr, "driftlock: %v\n", err)
		os.Exit(1)
	}
	return nil
}
