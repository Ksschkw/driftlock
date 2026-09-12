package hook

import (
	"os"
	"path/filepath"
	"testing"
)

// unsetEnv clears a variable for the duration of the test, restoring the
// original value (or absence) afterwards.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
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

// A project that configures DRIFTLOCK_SKIP in .env must actually be skipped.
// The check used to run before .env was loaded, so it silently did nothing.
func TestSkipRequestedReadsDotEnv(t *testing.T) {
	unsetEnv(t, "DRIFTLOCK_SKIP")

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("DRIFTLOCK_SKIP=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	if !skipRequested() {
		t.Fatal("DRIFTLOCK_SKIP=true in .env did not request a skip")
	}
}

func TestSkipRequestedFalseWhenUnset(t *testing.T) {
	unsetEnv(t, "DRIFTLOCK_SKIP")

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	if skipRequested() {
		t.Fatal("skipRequested() = true with no DRIFTLOCK_SKIP set")
	}
}

// A value exported in the real environment must win over .env, so a developer
// can override a committed default for one command.
func TestSkipRequestedShellValueWinsOverDotEnv(t *testing.T) {
	unsetEnv(t, "DRIFTLOCK_SKIP")
	t.Setenv("DRIFTLOCK_SKIP", "false")

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("DRIFTLOCK_SKIP=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	if skipRequested() {
		t.Fatal("shell DRIFTLOCK_SKIP=false was overridden by .env")
	}
}
