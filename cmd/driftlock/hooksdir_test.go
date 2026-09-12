package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	if out, err := exec.Command("git", full...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// core.hooksPath overrides .git/hooks. Writing to the wrong directory produces
// a hook git never runs, so the install silently does nothing.
func TestHooksDirHonoursCoreHooksPath(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init")

	got, err := hooksDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, ".git", "hooks"); got != want {
		t.Errorf("default hooksDir = %q, want %q", got, want)
	}

	git(t, dir, "config", "core.hooksPath", ".husky")
	got, err = hooksDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, ".husky"); got != want {
		t.Errorf("configured hooksDir = %q, want %q", got, want)
	}
}

func TestHooksDirAbsoluteCoreHooksPath(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init")
	abs := filepath.Join(t.TempDir(), "shared-hooks")
	if err := os.MkdirAll(abs, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "config", "core.hooksPath", abs)

	got, err := hooksDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != abs {
		t.Errorf("absolute hooksDir = %q, want %q", got, abs)
	}
}
