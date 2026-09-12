package hook

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitEnv(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir, "-c", "user.email=test@example.com", "-c", "user.name=Test"}, args...)
	cmd := exec.Command("git", full...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The check must judge the STAGED doc, because that is what the commit will
// contain. Reading the working tree let an unstaged doc edit mask a stale
// staged doc, so Driftlock blessed a commit it had never actually checked.
func TestReadDocForCheckUsesIndexNotWorkingTree(t *testing.T) {
	dir := t.TempDir()
	gitEnv(t, dir, "init")
	docPath := filepath.Join(dir, "README.md")
	writeFile(t, docPath, "committed content\n")
	gitEnv(t, dir, "add", "README.md")
	gitEnv(t, dir, "commit", "-m", "init")

	// Edit the working tree WITHOUT staging it.
	writeFile(t, docPath, "working tree content\n")

	got, err := readDocForCheck(dir, "README.md", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if got != "committed content\n" {
		t.Errorf("check read %q; want the staged blob %q", got, "committed content\n")
	}
}

func TestReadDocForCheckFollowsStagedUpdate(t *testing.T) {
	dir := t.TempDir()
	gitEnv(t, dir, "init")
	docPath := filepath.Join(dir, "README.md")
	writeFile(t, docPath, "v1\n")
	gitEnv(t, dir, "add", "README.md")
	gitEnv(t, dir, "commit", "-m", "init")

	writeFile(t, docPath, "v2\n")
	gitEnv(t, dir, "add", "README.md")

	got, err := readDocForCheck(dir, "README.md", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if got != "v2\n" {
		t.Errorf("check read %q; want staged %q", got, "v2\n")
	}
}

// An untracked doc has no index blob, so the working tree is the only source.
func TestReadDocForCheckFallsBackToDiskForUntracked(t *testing.T) {
	dir := t.TempDir()
	gitEnv(t, dir, "init")
	writeFile(t, filepath.Join(dir, "NEW.md"), "brand new\n")

	got, err := readDocForCheck(dir, "NEW.md", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if got != "brand new\n" {
		t.Errorf("untracked doc read %q; want %q", got, "brand new\n")
	}
}
