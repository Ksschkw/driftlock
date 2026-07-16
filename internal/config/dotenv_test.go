package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvSetsUnsetVars(t *testing.T) {
	dir := t.TempDir()
	content := `# a comment
export QUOTED="hello world"
SINGLE='single quoted'
PLAIN=bare_value

DRIFTLOCK_API_KEY=gsk_test123
`
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, k := range []string{"QUOTED", "SINGLE", "PLAIN", "DRIFTLOCK_API_KEY"} {
		os.Unsetenv(k)
		t.Cleanup(func() { os.Unsetenv(k) })
	}

	LoadDotEnv(dir)

	cases := map[string]string{
		"QUOTED":            "hello world",
		"SINGLE":            "single quoted",
		"PLAIN":             "bare_value",
		"DRIFTLOCK_API_KEY": "gsk_test123",
	}
	for k, want := range cases {
		if got := os.Getenv(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}

func TestLoadDotEnvDoesNotClobberRealEnv(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SHELL_WINS=from_file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	os.Setenv("SHELL_WINS", "from_shell")
	t.Cleanup(func() { os.Unsetenv("SHELL_WINS") })

	LoadDotEnv(dir)

	// A value exported in the real shell must take precedence over the file.
	if got := os.Getenv("SHELL_WINS"); got != "from_shell" {
		t.Errorf("real shell env was clobbered: got %q, want %q", got, "from_shell")
	}
}

func TestLoadDotEnvMissingFileIsNoOp(t *testing.T) {
	// Must not panic or error when no .env exists.
	LoadDotEnv(t.TempDir())
}

func TestExpandUsesDotEnvKey(t *testing.T) {
	dir := t.TempDir()
	os.Unsetenv("DRIFTLOCK_API_KEY")
	t.Cleanup(func() { os.Unsetenv("DRIFTLOCK_API_KEY") })

	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("DRIFTLOCK_API_KEY=secret-from-env-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgToml := `[[doc_mapping]]
sources = ["src/**"]
docs = ["README.md"]

[llm]
driver = "openai-compatible"
api_key = "${DRIFTLOCK_API_KEY}"
`
	cfgPath := filepath.Join(dir, ".driftlock.toml")
	if err := os.WriteFile(cfgPath, []byte(cfgToml), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLM.APIKey != "secret-from-env-file" {
		t.Errorf("api_key = %q, want it expanded from .env", cfg.LLM.APIKey)
	}
}
