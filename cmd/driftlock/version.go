package main

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Build metadata. Release builds override these with:
//
//	go build -ldflags "\
//	  -X main.version=$TAG \
//	  -X main.commit=$(git rev-parse --short HEAD) \
//	  -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
//	  -o driftlock ./cmd/driftlock
//
// The defaults make it obvious when a binary was built from a working tree
// rather than a release.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version and build information",
	Long: `Print the Driftlock version and the metadata baked in at build time.

Release binaries report the release tag; a binary built from a working tree
reports "dev", which makes it clear that a bug report is against unreleased
code.`,
	Run: func(cmd *cobra.Command, args []string) {
		printVersion()
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func printVersion() {
	fmt.Printf("driftlock %s\n", version)
	fmt.Printf("  commit:  %s\n", commit)
	fmt.Printf("  built:   %s\n", date)
	fmt.Printf("  go:      %s\n", runtime.Version())
	fmt.Printf("  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
}
