package main

import (
	"fmt"
	"os"

	"github.com/Ksschkw/driftlock/internal/audit"
	"github.com/spf13/cobra"
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Show recent driftlock audit hashes",
	Long:  `Displays the last 20 entries from the local audit log.`,
	RunE:  runLog,
}

func init() {
	rootCmd.AddCommand(logCmd)
}

func runLog(cmd *cobra.Command, args []string) error {
	lines, err := audit.ReadLog(20)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read audit log: %v\n", err)
		os.Exit(1)
	}
	for _, line := range lines {
		fmt.Println(line)
	}
	return nil
}