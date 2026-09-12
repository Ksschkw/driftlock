package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// GitHub does not evaluate ${{ }} expressions in action metadata `default`
// fields: they become literal strings. A default of
// `${{ github.event.pull_request.base.sha }}` therefore reached `git diff` as
// that literal text, and the shipped workflow failed on a bad revision rather
// than checking anything.
func TestActionMetadataDefaultsAreLiterals(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "action.yml"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")
	defaultLine := regexp.MustCompile(`^\s*default:\s*(.*)$`)
	for i, line := range lines {
		m := defaultLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if strings.Contains(m[1], "${{") {
			t.Errorf("action.yml:%d default uses an expression, which GitHub does not evaluate: %s", i+1, strings.TrimSpace(line))
		}
	}
}

// The refs must be resolved inside a run step, where the workflow context is
// available, and exposed as step outputs.
func TestActionResolvesRefsInRunStep(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "action.yml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{
		"id: refs",
		"GITHUB_OUTPUT",
		"${{ github.event.pull_request.base.sha }}",
		"${{ github.sha }}",
		"steps.refs.outputs.base",
		"steps.refs.outputs.head",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("action.yml is missing %q", want)
		}
	}
}

// An explicitly supplied ref must still win over the auto-detected one.
func TestActionResolutionHonoursExplicitInputs(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "action.yml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{"INPUT_BASE", "INPUT_HEAD"} {
		if !strings.Contains(content, want) {
			t.Errorf("action.yml does not consult %q before falling back", want)
		}
	}
}
