package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Ksschkw/driftlock/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Driftlock in the current Git repository interactively",
	Long: `Walks you through every configuration option and writes a complete
.driftlock.toml. You can press Enter at any prompt to accept the default
value (shown in brackets). The pre-commit hook and .gitignore entries are
also created automatically.`,
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

	// Build the configuration interactively.
	cfg := interactiveConfig()

	// Write the config file.
	if err := config.WriteConfig(configPath, cfg); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Install pre-commit hook.
	hooksPath := filepath.Join(root, ".git", "hooks")
	if err := os.MkdirAll(hooksPath, 0o755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}
	hookPath := filepath.Join(hooksPath, "pre-commit")
	hookContent := "#!/bin/sh\n# Driftlock pre-commit hook\nexec driftlock hook-run\n"
	if err := os.WriteFile(hookPath, []byte(hookContent), 0o755); err != nil {
		return fmt.Errorf("failed to write hook script: %w", err)
	}

	// Update .gitignore.
	if err := updateGitignore(root); err != nil {
		fmt.Printf("warning: could not update .gitignore: %v\n", err)
	}

	fmt.Println()
	fmt.Println("Driftlock initialized successfully.")
	fmt.Println("A .driftlock.toml has been created, the pre-commit hook is active,")
	fmt.Println("and .driftlock.toml and .driftlock/ have been added to .gitignore.")
	return nil
}

// interactiveConfig prompts the user for every configuration section.
func interactiveConfig() *config.Config {
	reader := bufio.NewReader(os.Stdin)
	cfg := config.DefaultConfig()

	fmt.Println()
	fmt.Println("  Driftlock interactive setup")
	fmt.Println("  Press Enter to accept the default value shown in [brackets].")
	fmt.Println()

	// ── Doc Mapping ──────────────────────────────────────────────
	fmt.Println("── Documentation mapping ──")
	sources := prompt(reader, "Source file patterns (space‑separated)", "src/**")
	docs := prompt(reader, "Documentation files or directories (space‑separated)", "README.md docs/")
	cfg.DocMapping = []config.DocMapEntry{
		{
			Sources: strings.Fields(sources),
			Docs:    strings.Fields(docs),
		},
	}

	// ── LLM ──────────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("── LLM provider ──")
	cfg.LLM.Driver = prompt(reader, "Driver (openai-compatible / ollama)", cfg.LLM.Driver)
	cfg.LLM.Endpoint = prompt(reader, "Full endpoint URL", cfg.LLM.Endpoint)
	cfg.LLM.Model = prompt(reader, "Model name", cfg.LLM.Model)
	cfg.LLM.APIKey = prompt(reader, "API key (or ${ENV_VAR})", cfg.LLM.APIKey)

	fmt.Println()
	fmt.Println("── LLM options ──")
	tempStr := prompt(reader, "Temperature", fmt.Sprintf("%v", cfg.LLM.Options["temperature"]))
	if t, err := strconv.ParseFloat(tempStr, 64); err == nil {
		cfg.LLM.Options["temperature"] = t
	}
	maxTokStr := prompt(reader, "Max tokens", fmt.Sprintf("%v", cfg.LLM.Options["max_tokens"]))
	if mt, err := strconv.Atoi(maxTokStr); err == nil {
		cfg.LLM.Options["max_tokens"] = mt
	}

	// ── Behavior ─────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("── Behavior ──")
	cfg.Behavior.AutoFix = promptBool(reader, "Auto‑fix documentation on drift", cfg.Behavior.AutoFix)
	cfg.Behavior.BlockOnFalse = promptBool(reader, "Block commit when docs are outdated", cfg.Behavior.BlockOnFalse)
	cfg.Behavior.BlockOnLLMError = promptBool(reader, "Block commit when LLM is unreachable", cfg.Behavior.BlockOnLLMError)
	maxRetStr := prompt(reader, "Max LLM retries", fmt.Sprintf("%d", cfg.Behavior.MaxRetries))
	if mr, err := strconv.Atoi(maxRetStr); err == nil {
		cfg.Behavior.MaxRetries = mr
	}
	cfg.Behavior.IncludeFullDiff = promptBool(reader, "Send full diff to LLM (uses more tokens)", cfg.Behavior.IncludeFullDiff)

	// ── Audit (Solana) ───────────────────────────────────────────
	fmt.Println()
	fmt.Println("── Solana audit (optional) ──")
	cfg.Audit.Solana = promptBool(reader, "Enable Solana audit logging", cfg.Audit.Solana)
	if cfg.Audit.Solana {
		cfg.Audit.RPCEndpoint = prompt(reader, "RPC endpoint", "https://api.devnet.solana.com")
		cfg.Audit.KeypairPath = prompt(reader, "Keypair path", "~/.config/solana/id.json")
		cfg.Audit.ProgramID = prompt(reader, "Program ID (leave empty for default Memo program)", "")
	}

	return cfg
}

// prompt prints a label and default, then reads a line from the user.
func prompt(r *bufio.Reader, label, def string) string {
	fmt.Printf("  %s [%s]: ", label, def)
	line, err := r.ReadString('\n')
	if err != nil || strings.TrimSpace(line) == "" {
		return def
	}
	return strings.TrimSpace(line)
}

// promptBool asks a yes/no question and returns true/false.
func promptBool(r *bufio.Reader, label string, def bool) bool {
	defStr := "n"
	if def {
		defStr = "y"
	}
	fmt.Printf("  %s (y/n) [%s]: ", label, defStr)
	line, err := r.ReadString('\n')
	if err != nil || strings.TrimSpace(line) == "" {
		return def
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

// updateGitignore ensures .driftlock.toml and .driftlock/ are in .gitignore.
func updateGitignore(root string) error {
	ignorePath := filepath.Join(root, ".gitignore")
	entries := []string{".driftlock.toml", ".driftlock/"}

	existing, err := os.ReadFile(ignorePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := string(existing)

	f, err := os.OpenFile(ignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		f.WriteString("\n")
	}
	for _, e := range entries {
		if !strings.Contains(content, e) {
			f.WriteString(e + "\n")
		}
	}
	return nil
}
