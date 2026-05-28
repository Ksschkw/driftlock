package main

import (
	// "errors"
	"fmt"
	"os"
	"path/filepath"
	// "strings"

	"github.com/Ksschkw/driftlock/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Driftlock in the current Git repository",
	Long: `Creates a .driftlock.toml with sensible defaults and installs
a pre-commit hook that runs driftlock hook-run.`,
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
	hookContent := fmt.Sprintf(`#!/bin/sh
# Driftlock pre-commit hook
exec driftlock hook-run
`)
	if err := os.WriteFile(hookPath, []byte(hookContent), 0o755); err != nil {
		return fmt.Errorf("failed to write hook script: %w", err)
	}

	fmt.Println("Driftlock initialized successfully.")
	fmt.Println("A .driftlock.toml has been created, and the pre-commit hook is active.")
	return nil
}