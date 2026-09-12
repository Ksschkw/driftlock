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

// DRIFTLOCK_DEBUG follows the documented truthiness rule. The old check treated
// any non-empty value as on, so the documented `DRIFTLOCK_DEBUG=0` enabled
// verbose output and dumped raw LLM payloads on every commit.
func TestDebugEnabled(t *testing.T) {
	for _, on := range []string{"1", "true", "TRUE", "yes", "on", " on "} {
		t.Run("on_"+on, func(t *testing.T) {
			t.Setenv("DRIFTLOCK_DEBUG", on)
			if !DebugEnabled() {
				t.Errorf("DebugEnabled() = false for %q", on)
			}
		})
	}
	for _, off := range []string{"", "0", "false", "FALSE", "no", "off", "nonsense"} {
		t.Run("off_"+off, func(t *testing.T) {
			t.Setenv("DRIFTLOCK_DEBUG", off)
			if DebugEnabled() {
				t.Errorf("DebugEnabled() = true for %q", off)
			}
		})
	}
}

// The shipped example must parse into the documented values now that inline
// comments are stripped.
func TestExampleDotEnvValues(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".env.example"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"DRIFTLOCK_DEBUG", "DRIFTLOCK_SKIP", "DRIFTLOCK_STRICT_LLM", "DRIFTLOCK_API_KEY"} {
		old, had := os.LookupEnv(k)
		os.Unsetenv(k)
		t.Cleanup(func() {
			if had {
				_ = os.Setenv(k, old)
			} else {
				_ = os.Unsetenv(k)
			}
		})
	}

	LoadDotEnv(dir)

	cases := map[string]string{
		"DRIFTLOCK_DEBUG":      "0",
		"DRIFTLOCK_SKIP":       "false",
		"DRIFTLOCK_STRICT_LLM": "false",
		"DRIFTLOCK_API_KEY":    "your-api-key-here",
	}
	for k, want := range cases {
		if got := os.Getenv(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if DebugEnabled() {
		t.Error("the example .env turns debugging on")
	}
}
