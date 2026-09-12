package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout runs fn with os.Stdout redirected and returns what it printed.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(r)
		done <- string(data)
	}()
	fn()
	_ = w.Close()
	os.Stdout = old
	return <-done
}

func TestVersionCommandReportsBuildMetadata(t *testing.T) {
	oldVersion, oldCommit, oldDate := version, commit, date
	version, commit, date = "v9.9.9", "abc1234", "2026-01-02T03:04:05Z"
	t.Cleanup(func() { version, commit, date = oldVersion, oldCommit, oldDate })

	out := captureStdout(t, printVersion)
	for _, want := range []string{"v9.9.9", "abc1234", "2026-01-02T03:04:05Z", "go:", "os/arch:"} {
		if !strings.Contains(out, want) {
			t.Errorf("version output missing %q:\n%s", want, out)
		}
	}
}

// A working-tree build must say so, so a bug report is not mistaken for a
// release.
func TestVersionDefaultsToDev(t *testing.T) {
	if version != "dev" {
		t.Errorf("default version = %q, want dev", version)
	}
}

func TestVersionCommandIsRegistered(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "version" {
			found = true
		}
	}
	if !found {
		t.Error("version subcommand is not registered on the root command")
	}
}
