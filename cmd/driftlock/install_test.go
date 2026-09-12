package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func anyNoteContains(notes []string, want string) bool {
	for _, n := range notes {
		if strings.Contains(n, want) {
			return true
		}
	}
	return false
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A fresh repository gets a standalone hook that fails loudly if the binary is
// missing.
func TestInstallHookFreshRepo(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init")

	notes, err := installPreCommitHook(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) == 0 || !strings.Contains(notes[0], "Installed") {
		t.Errorf("unexpected notes: %v", notes)
	}

	hook := readFile(t, filepath.Join(dir, ".git", "hooks", "pre-commit"))
	if !strings.Contains(hook, "exec driftlock hook-run") {
		t.Errorf("fresh hook does not run driftlock:\n%s", hook)
	}
	if !strings.Contains(hook, driftlockHookBegin) {
		t.Errorf("fresh hook is missing the ownership marker:\n%s", hook)
	}
	info, err := os.Stat(filepath.Join(dir, ".git", "hooks", "pre-commit"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("hook is not executable: %v", info.Mode())
	}
}

// An existing hook from husky/lint-staged/pre-commit must be preserved and
// Driftlock appended, not overwritten. Clobbering it destroys a working setup.
func TestInstallHookPreservesForeignHook(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init")
	hooksDirPath := filepath.Join(dir, ".git", "hooks")
	if err := os.MkdirAll(hooksDirPath, 0o755); err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(hooksDirPath, "pre-commit")
	original := "#!/bin/sh\n# my custom hook\necho custom-check\n"
	if err := os.WriteFile(hookPath, []byte(original), 0o755); err != nil {
		t.Fatal(err)
	}

	notes, err := installPreCommitHook(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !anyNoteContains(notes, "Appended") {
		t.Errorf("no append note in %v", notes)
	}

	hook := readFile(t, hookPath)
	if !strings.Contains(hook, "echo custom-check") {
		t.Errorf("foreign hook content was destroyed:\n%s", hook)
	}
	if !strings.Contains(hook, "driftlock hook-run") {
		t.Errorf("driftlock was not appended:\n%s", hook)
	}
	// The chained block must not use exec, which would skip the rest of the
	// original hook if it were placed before it.
	if strings.Contains(hook, "exec driftlock") {
		t.Errorf("chained block uses exec and would replace the shell:\n%s", hook)
	}
}

// A repository with core.hooksPath set gets the hook in that directory.
func TestInstallHookRespectsHooksPath(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init")
	git(t, dir, "config", "core.hooksPath", ".husky")

	if _, err := installPreCommitHook(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".husky", "pre-commit")); err != nil {
		t.Fatalf("hook not written to core.hooksPath: %v", err)
	}
}

// The original hook is preserved before modification, so a developer can always
// see (and restore) what was there.
func TestInstallHookBacksUpForeignHook(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init")
	hooksDirPath := filepath.Join(dir, ".git", "hooks")
	if err := os.MkdirAll(hooksDirPath, 0o755); err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(hooksDirPath, "pre-commit")
	original := "#!/bin/sh\necho original\n"
	if err := os.WriteFile(hookPath, []byte(original), 0o755); err != nil {
		t.Fatal(err)
	}

	notes, err := installPreCommitHook(dir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range notes {
		if strings.Contains(n, "Backed up") {
			found = true
		}
	}
	if !found {
		t.Errorf("no backup note in %v", notes)
	}

	backup := readFile(t, hookPath+".driftlock-backup")
	if backup != original {
		t.Errorf("backup content = %q, want %q", backup, original)
	}

	// A second install must not overwrite the true original backup.
	modified := readFile(t, hookPath)
	if _, err := installPreCommitHook(dir); err != nil {
		t.Fatal(err)
	}
	if again := readFile(t, hookPath+".driftlock-backup"); again != original {
		t.Errorf("backup was overwritten on re-install: %q", again)
	}
	if !strings.Contains(modified, "driftlock") {
		t.Errorf("first install did not append driftlock: %q", modified)
	}
}

// Running the installer twice must not append a second Driftlock block. A
// duplicated block would run the check twice on every commit.
func TestInstallHookIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init")

	if _, err := installPreCommitHook(dir); err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	first := readFile(t, hookPath)

	notes, err := installPreCommitHook(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !anyNoteContains(notes, "already contains") {
		t.Errorf("second install did not report idempotency: %v", notes)
	}
	second := readFile(t, hookPath)
	if first != second {
		t.Errorf("hook changed on re-install:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	if n := strings.Count(second, driftlockHookBegin); n != 1 {
		t.Errorf("driftlock marker appears %d times, want 1:\n%s", n, second)
	}
}

// The same guard applies to a chained foreign hook.
func TestInstallHookIsIdempotentWithForeignHook(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init")
	hooksDirPath := filepath.Join(dir, ".git", "hooks")
	if err := os.MkdirAll(hooksDirPath, 0o755); err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(hooksDirPath, "pre-commit")
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\necho custom\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := installPreCommitHook(dir); err != nil {
		t.Fatal(err)
	}
	first := readFile(t, hookPath)
	if _, err := installPreCommitHook(dir); err != nil {
		t.Fatal(err)
	}
	second := readFile(t, hookPath)
	if first != second {
		t.Errorf("foreign hook changed on re-install:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	if n := strings.Count(second, driftlockHookBegin); n != 1 {
		t.Errorf("driftlock marker appears %d times, want 1", n)
	}
	if !strings.Contains(second, "echo custom") {
		t.Errorf("foreign content lost on re-install:\n%s", second)
	}
}

// The hook invokes `driftlock` by name, so install must notice when it is not
// resolvable rather than leaving the user with failing or no-op commits.
func TestDriftlockOnPath(t *testing.T) {
	empty := t.TempDir()
	t.Setenv("PATH", empty)
	if driftlockOnPath() {
		t.Fatal("driftlockOnPath() = true with an empty PATH")
	}

	bin := t.TempDir()
	stub := filepath.Join(bin, "driftlock")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if !driftlockOnPath() {
		t.Fatal("driftlockOnPath() = false with a stub on PATH")
	}
}
