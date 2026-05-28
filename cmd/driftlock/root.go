package main

import (
	// "fmt"
	// "os"

	// "github.com/Ksschkw/driftlock/internal/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "driftlock",
	Short: "Driftlock – keep your docs in sync with your code.",
	Long: `Driftlock watches your commits and blocks them if the documentation
is out of sync. It can auto-fix the docs for you.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Default: show help if no subcommand given.
		cmd.Help()
	},
}

func Execute() error {
	return rootCmd.Execute()
}