package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config represents the entire .driftlock.toml structure.
type Config struct {
	DocMapping []DocMapEntry  `toml:"doc_mapping"`
	LLM        LLMConfig      `toml:"llm"`
	Behavior   BehaviorConfig `toml:"behavior"`
	Audit      AuditConfig    `toml:"audit"`
}

// DocMapEntry maps source file globs to documentation files.
type DocMapEntry struct {
	Sources []string `toml:"sources"`
	Docs    []string `toml:"docs"`
}

// LLMConfig holds all LLM-related configuration.
type LLMConfig struct {
	Driver   string         `toml:"driver"`
	Endpoint string         `toml:"endpoint"`
	Model    string         `toml:"model"`
	APIKey   string         `toml:"api_key"`
	Options  map[string]any `toml:"options"`
	Prompts  *PromptConfig  `toml:"prompts"`
}

// PromptConfig allows users to override the default prompts.
type PromptConfig struct {
	Check string `toml:"check"`
	Fix   string `toml:"fix"`
}

// BehaviorConfig controls how Driftlock acts.
type BehaviorConfig struct {
	AutoFix      bool `toml:"auto_fix"`
	BlockOnFalse bool `toml:"block_on_false"`
	MaxRetries   int  `toml:"max_retries"`
}

// AuditConfig holds the optional Solana audit settings.
type AuditConfig struct {
	Solana      bool   `toml:"solana"`
	RPCEndpoint string `toml:"rpc_endpoint"`
	KeypairPath string `toml:"keypair_path"`
	ProgramID   string `toml:"program_id"`
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		DocMapping: []DocMapEntry{
			{
				Sources: []string{"src/**"},
				Docs:    []string{"README.md", "docs/"},
			},
		},
		LLM: LLMConfig{
			Driver:   "ollama",
			Endpoint: "http://localhost:11434",
			Model:    "codestral:22b",
			Options: map[string]any{
				"temperature": 0.0,
				"max_tokens":  4096,
			},
		},
		Behavior: BehaviorConfig{
			AutoFix:      true,
			BlockOnFalse: true,
			MaxRetries:   2,
		},
		Audit: AuditConfig{
			Solana: false,
		},
	}
}

// LoadConfig loads the configuration from the given path.
// If the file does not exist, it returns the default config.
func LoadConfig(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return cfg, nil
	}

	if _, err := toml.DecodeFile(configPath, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	// Expand environment variables in string fields.
	cfg.LLM.APIKey = os.ExpandEnv(cfg.LLM.APIKey)
	cfg.LLM.Endpoint = os.ExpandEnv(cfg.LLM.Endpoint)
	if cfg.LLM.Prompts != nil {
		cfg.LLM.Prompts.Check = os.ExpandEnv(cfg.LLM.Prompts.Check)
		cfg.LLM.Prompts.Fix = os.ExpandEnv(cfg.LLM.Prompts.Fix)
	}

	return cfg, nil
}

// WriteConfig writes the configuration to the given path.
func WriteConfig(configPath string, cfg *Config) error {
	f, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := toml.NewEncoder(f)
	return enc.Encode(cfg)
}

// FindProjectRoot locates the root of the Git repository containing the current directory.
func FindProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("not inside a Git repository (no .git directory found)")
}

// LoadProjectConfig searches for .driftlock.toml from the Git root.
func LoadProjectConfig() (*Config, error) {
	root, err := FindProjectRoot()
	if err != nil {
		return nil, err
	}
	configFile := filepath.Join(root, ".driftlock.toml")
	cfg, err := LoadConfig(configFile)
	if err != nil {
		return nil, fmt.Errorf("error loading configuration: %w", err)
	}
	return cfg, nil
}