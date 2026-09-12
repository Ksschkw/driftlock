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

func TestResolvedCheckMode(t *testing.T) {
	cases := map[string]string{
		"":               CheckModeAuto,
		"auto":           CheckModeAuto,
		"AUTO":           CheckModeAuto,
		" deterministic": CheckModeDeterministic,
		"llm":            CheckModeLLM,
		"LLM":            CheckModeLLM,
		"nonsense":       CheckModeAuto, // a typo must not disable the model
	}
	for in, want := range cases {
		if got := (BehaviorConfig{CheckMode: in}).ResolvedCheckMode(); got != want {
			t.Errorf("ResolvedCheckMode(%q) = %q, want %q", in, got, want)
		}
	}
	if got := DefaultConfig().Behavior.ResolvedCheckMode(); got != CheckModeAuto {
		t.Errorf("default check mode = %q, want auto", got)
	}
}

func TestLoadConfigCheckMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".driftlock.toml")
	if err := os.WriteFile(path, []byte("[behavior]\ncheck_mode = \"deterministic\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Behavior.ResolvedCheckMode(); got != CheckModeDeterministic {
		t.Errorf("loaded check mode = %q, want deterministic", got)
	}
}
