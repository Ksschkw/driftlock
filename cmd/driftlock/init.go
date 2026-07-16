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

	// Decide how to handle the API key so the config can be safely committed.
	// A literal secret must never be committed; an ${ENV_VAR} reference is safe.
	keyIsLiteral := isLiteralSecret(cfg.LLM.APIKey)
	if keyIsLiteral {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println()
		fmt.Println("You entered an API key directly. Committing it is unsafe.")
		if promptBool(reader, "Store it in .env and reference ${DRIFTLOCK_API_KEY} in the config (recommended)", true) {
			if err := writeEnvKey(root, "DRIFTLOCK_API_KEY", cfg.LLM.APIKey); err != nil {
				return fmt.Errorf("failed to write .env: %w", err)
			}
			cfg.LLM.APIKey = "${DRIFTLOCK_API_KEY}"
			keyIsLiteral = false
			fmt.Println("Wrote the key to .env; the config now references ${DRIFTLOCK_API_KEY}.")
		}
	}

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

	// Update .gitignore. Always ignore local state (.driftlock/) and secrets
	// (.env). Only ignore .driftlock.toml itself when it still holds a literal
	// key — otherwise leave it committable so teams share one policy file.
	if err := updateGitignore(root, keyIsLiteral); err != nil {
		fmt.Printf("warning: could not update .gitignore: %v\n", err)
	}

	fmt.Println()
	fmt.Println("Driftlock initialized successfully.")
	if keyIsLiteral {
		fmt.Println("Your config contains a literal API key, so .driftlock.toml was")
		fmt.Println("added to .gitignore. To share config with your team, move the key")
		fmt.Println("to .env and set api_key = \"${DRIFTLOCK_API_KEY}\", then commit the config.")
	} else {
		fmt.Println("The pre-commit hook is active. .driftlock.toml is safe to commit")
		fmt.Println("(it holds no secret) so your team shares one policy; .env and")
		fmt.Println(".driftlock/ are gitignored.")
	}
	return nil
}

// isLiteralSecret reports whether an api_key value is a real secret that must
// not be committed, as opposed to an empty value (Ollama needs none) or an
// ${ENV_VAR} reference (safe — the secret lives in the environment/.env).
func isLiteralSecret(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	return !strings.Contains(key, "${")
}

// writeEnvKey appends KEY=value to the project's .env file, creating it if
// needed and avoiding a duplicate entry for the same key.
func writeEnvKey(root, name, value string) error {
	envPath := filepath.Join(root, ".env")
	existing, err := os.ReadFile(envPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if strings.Contains(string(existing), name+"=") {
		return nil // already present; do not clobber
	}
	f, err := os.OpenFile(envPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		f.WriteString("\n")
	}
	_, err = f.WriteString(name + "=" + value + "\n")
	return err
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

// updateGitignore ensures the appropriate entries are in .gitignore. Local
// state (.driftlock/) and secrets (.env) are always ignored. The config file
// itself is ignored only when it still holds a literal API key; when the key
// is externalized to an env var, the config is committable so a team shares
// one policy.
func updateGitignore(root string, ignoreConfig bool) error {
	ignorePath := filepath.Join(root, ".gitignore")
	entries := []string{".driftlock/", ".env"}
	if ignoreConfig {
		entries = append(entries, ".driftlock.toml")
	}

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
		if !gitignoreHasEntry(content, e) {
			f.WriteString(e + "\n")
		}
	}
	return nil
}

// gitignoreHasEntry reports whether a gitignore already lists an exact entry,
// matching whole lines so that ".env" is not considered present merely because
// ".driftlock/" or a comment contains the substring.
func gitignoreHasEntry(content, entry string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == entry {
			return true
		}
	}
	return false
}
