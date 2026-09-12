package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	// TimeoutSeconds bounds a single LLM request. It defaults to 60 when
	// omitted or non-positive. Without it a hung or black-holed provider left
	// the http.Client waiting forever, which hangs `git commit`.
	TimeoutSeconds int `toml:"timeout_seconds"`
}

// DefaultLLMTimeout is the per-request LLM timeout used when the config does
// not specify one.
const DefaultLLMTimeout = 60 * time.Second

// HTTPTimeout returns the per-request timeout for LLM calls.
func (l LLMConfig) HTTPTimeout() time.Duration {
	if l.TimeoutSeconds <= 0 {
		return DefaultLLMTimeout
	}
	return time.Duration(l.TimeoutSeconds) * time.Second
}

// PromptConfig allows users to override the default prompts.
type PromptConfig struct {
	Check string `toml:"check"`
	Fix   string `toml:"fix"`
}

// BehaviorConfig controls how Driftlock acts.
type BehaviorConfig struct {
	AutoFix         bool `toml:"auto_fix"`
	BlockOnFalse    bool `toml:"block_on_false"`
	MaxRetries      int  `toml:"max_retries"`
	IncludeFullDiff bool `toml:"include_full_diff"`
	// BlockOnLLMError forces a commit block when the LLM is unreachable,
	// instead of allowing the commit with a warning.
	BlockOnLLMError bool `toml:"block_on_llm_error"`
	// Cache enables the content-addressed verdict cache, which avoids
	// re-billing the LLM for identical (model, diff, doc) checks across
	// commit amends, rebases, and CI re-runs. Enabled by default.
	Cache *bool `toml:"cache"`
	// ReportUnparsed warns when a mapped source file yields no structural
	// signatures at all, which usually means the extractor did not understand
	// it. Off by default because a file that legitimately declares nothing is
	// common; the warning is always available through DRIFTLOCK_DEBUG.
	ReportUnparsed bool `toml:"report_unparsed"`
	// CheckMode selects how a check is decided. See the CheckMode constants.
	// An unset or unrecognised value resolves to "auto".
	CheckMode string `toml:"check_mode"`
}

// Check modes for BehaviorConfig.CheckMode.
const (
	// CheckModeAuto decides deterministically when the change set allows it and
	// falls back to the model otherwise. This is the default.
	CheckModeAuto = "auto"
	// CheckModeDeterministic never calls the model. Changes it cannot decide
	// (a modified signature that the documentation does mention) are reported
	// as unjudged rather than guessed, so Driftlock can run with no API key.
	CheckModeDeterministic = "deterministic"
	// CheckModeLLM always consults the model, as Driftlock did before the
	// deterministic path existed.
	CheckModeLLM = "llm"
)

// DebugEnabled reports whether DRIFTLOCK_DEBUG asks for verbose output. Any of
// "1", "true", "yes", or "on" (case-insensitive) enables it; an empty value,
// "0", "false", "no", or "off" disables it.
//
// The check used to be "is the variable non-empty", so the documented
// `DRIFTLOCK_DEBUG=0` turned debugging ON and every commit dumped raw LLM
// payloads to stderr.
func DebugEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DRIFTLOCK_DEBUG"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// ResolvedCheckMode returns the effective check mode. An empty or unrecognised
// value resolves to auto, so configs written before this option existed keep
// working and a typo cannot silently disable the model.
func (b BehaviorConfig) ResolvedCheckMode() string {
	switch strings.ToLower(strings.TrimSpace(b.CheckMode)) {
	case CheckModeDeterministic:
		return CheckModeDeterministic
	case CheckModeLLM:
		return CheckModeLLM
	default:
		return CheckModeAuto
	}
}

// CacheEnabled reports whether the verdict cache is on. It defaults to true
// when the field is omitted from the config file.
func (b BehaviorConfig) CacheEnabled() bool {
	return b.Cache == nil || *b.Cache
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
			Driver:         "ollama",
			Endpoint:       "http://localhost:11434",
			Model:          "codestral:22b",
			TimeoutSeconds: int(DefaultLLMTimeout / time.Second),
			Options: map[string]any{
				"temperature": 0.0,
				"max_tokens":  2048,
			},
		},
		Behavior: BehaviorConfig{
			AutoFix:         true,
			BlockOnFalse:    true,
			MaxRetries:      2,
			IncludeFullDiff: false,
			BlockOnLLMError: false, // default: allow commit on LLM error
			CheckMode:       CheckModeAuto,
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

	// Load a sibling .env before expanding, so ${VARS} referenced in the config
	// resolve from it. Real shell environment variables take precedence.
	LoadDotEnv(filepath.Dir(configPath))

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
