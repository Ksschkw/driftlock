package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHTTPTimeoutDefaults(t *testing.T) {
	if got := (LLMConfig{}).HTTPTimeout(); got != DefaultLLMTimeout {
		t.Errorf("zero config timeout = %v, want %v", got, DefaultLLMTimeout)
	}
	if got := (LLMConfig{TimeoutSeconds: -5}).HTTPTimeout(); got != DefaultLLMTimeout {
		t.Errorf("negative timeout = %v, want the default %v", got, DefaultLLMTimeout)
	}
	if got := DefaultConfig().LLM.HTTPTimeout(); got != DefaultLLMTimeout {
		t.Errorf("DefaultConfig timeout = %v, want %v", got, DefaultLLMTimeout)
	}
}

func TestHTTPTimeoutOverride(t *testing.T) {
	if got := (LLMConfig{TimeoutSeconds: 5}).HTTPTimeout(); got != 5*time.Second {
		t.Errorf("configured timeout = %v, want 5s", got)
	}
}

// A timeout in the config file must survive loading; the default must apply
// when the field is omitted.
func TestLoadConfigTimeout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".driftlock.toml")

	if err := os.WriteFile(path, []byte("[llm]\ntimeout_seconds = 12\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.LLM.HTTPTimeout(); got != 12*time.Second {
		t.Errorf("loaded timeout = %v, want 12s", got)
	}

	omitted := filepath.Join(dir, "omitted.toml")
	if err := os.WriteFile(omitted, []byte("[llm]\nmodel = \"x\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadConfig(omitted)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.LLM.HTTPTimeout(); got != DefaultLLMTimeout {
		t.Errorf("omitted timeout = %v, want the default %v", got, DefaultLLMTimeout)
	}
}
