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

// The shipped example must actually load, and must exercise the documented
// defaults. It is the first file a new user copies, so rot here is expensive.
func TestExampleConfigLoads(t *testing.T) {
	// LoadConfig loads a sibling .env; keep the repository's own .env from
	// leaking into this process for the rest of the test binary.
	for _, key := range []string{"DRIFTLOCK_SKIP", "DRIFTLOCK_DEBUG", "DRIFTLOCK_API_KEY"} {
		old, had := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if had {
				_ = os.Setenv(key, old)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}

	cfg, err := LoadConfig(filepath.Join("..", "..", ".driftlock.example.toml"))
	if err != nil {
		t.Fatalf(".driftlock.example.toml does not load: %v", err)
	}
	if got := cfg.Behavior.ResolvedCheckMode(); got != CheckModeAuto {
		t.Errorf("example check_mode = %q, want auto", got)
	}
	if got := cfg.LLM.HTTPTimeout(); got <= 0 {
		t.Errorf("example LLM timeout = %v, want positive", got)
	}
	if len(cfg.DocMapping) == 0 {
		t.Error("example has no doc_mapping")
	}
}
