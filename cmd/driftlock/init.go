package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Ksschkw/driftlock/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Driftlock in the current Git repository",
	Long: `Creates a .driftlock.toml with sensible defaults, installs a pre-commit hook,
and adds .driftlock.toml and .driftlock/ to .gitignore.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	root, err := config.FindProjectRoot()
	if err != nil {
		return fmt.Errorf("not inside a Git repository: %w", err)
	}

	configPath := filepath.Join(root, ".driftlock.toml")
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf(".driftlock.toml already exists; remove it first if you want to reinitialize")
	}

	// Write default config
	cfg := config.DefaultConfig()
	if err := config.WriteConfig(configPath, cfg); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Install pre-commit hook
	hooksPath := filepath.Join(root, ".git", "hooks")
	if err := os.MkdirAll(hooksPath, 0o755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}
	hookPath := filepath.Join(hooksPath, "pre-commit")
	hookContent := `#!/bin/sh
# Driftlock pre-commit hook
exec driftlock hook-run
`
	if err := os.WriteFile(hookPath, []byte(hookContent), 0o755); err != nil {
		return fmt.Errorf("failed to write hook script: %w", err)
	}

	// Add .driftlock.toml and .driftlock/ to .gitignore
	if err := updateGitignore(root); err != nil {
		fmt.Printf("warning: could not update .gitignore: %v\n", err)
	}

	fmt.Println("Driftlock initialized successfully.")
	fmt.Println("A .driftlock.toml has been created, and the pre-commit hook is active.")
	fmt.Println(".driftlock.toml and .driftlock/ have been added to .gitignore.")
	return nil
}

func updateGitignore(root string) error {
	ignorePath := filepath.Join(root, ".gitignore")
	lines := []string{
		".driftlock.toml",
		".driftlock/",
	}

	// Read existing content
	existing, err := os.ReadFile(ignorePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := string(existing)
	needsUpdate := false
	for _, line := range lines {
		if !strings.Contains(content, line) {
			needsUpdate = true
			break
		}
	}
	if !needsUpdate {
		return nil
	}

	// Append missing lines
	f, err := os.OpenFile(ignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Ensure a newline if file is not empty
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		f.WriteString("\n")
	}
	for _, line := range lines {
		if !strings.Contains(content, line) {
			f.WriteString(line + "\n")
		}
	}
	return nil
}
