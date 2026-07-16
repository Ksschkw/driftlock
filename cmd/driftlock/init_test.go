package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsLiteralSecret(t *testing.T) {
	cases := map[string]bool{
		"":                     false, // Ollama: no key needed
		"   ":                  false, // whitespace only
		"${DRIFTLOCK_API_KEY}": false, // env reference is safe to commit
		"prefix-${VAR}-suffix": false, // contains an expansion
		"gsk_realsecret123":    true,  // a literal key must be protected
	}
	for in, want := range cases {
		if got := isLiteralSecret(in); got != want {
			t.Errorf("isLiteralSecret(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestWriteEnvKeyCreatesAndDedupes(t *testing.T) {
	dir := t.TempDir()
	if err := writeEnvKey(dir, "DRIFTLOCK_API_KEY", "secret1"); err != nil {
		t.Fatal(err)
	}
	// A second write for the same key must not append a duplicate.
	if err := writeEnvKey(dir, "DRIFTLOCK_API_KEY", "secret2"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), "DRIFTLOCK_API_KEY="); n != 1 {
		t.Errorf("expected exactly one key entry, found %d in:\n%s", n, data)
	}
	if !strings.Contains(string(data), "DRIFTLOCK_API_KEY=secret1") {
		t.Errorf("original value should be preserved, got:\n%s", data)
	}
}

func TestUpdateGitignoreConfigVisibility(t *testing.T) {
	// Literal key present → .driftlock.toml must be ignored.
	litDir := t.TempDir()
	if err := updateGitignore(litDir, true); err != nil {
		t.Fatal(err)
	}
	lit := readGitignore(t, litDir)
	for _, want := range []string{".driftlock/", ".env", ".driftlock.toml"} {
		if !gitignoreHasEntry(lit, want) {
			t.Errorf("literal-key gitignore missing %q:\n%s", want, lit)
		}
	}

	// Env-referenced key → config is committable, must NOT be ignored.
	envDir := t.TempDir()
	if err := updateGitignore(envDir, false); err != nil {
		t.Fatal(err)
	}
	shared := readGitignore(t, envDir)
	if gitignoreHasEntry(shared, ".driftlock.toml") {
		t.Errorf("committable config should not be gitignored:\n%s", shared)
	}
	for _, want := range []string{".driftlock/", ".env"} {
		if !gitignoreHasEntry(shared, want) {
			t.Errorf("shared gitignore missing %q:\n%s", want, shared)
		}
	}
}

func readGitignore(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
