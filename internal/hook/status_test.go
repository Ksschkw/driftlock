package hook

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBareNameStripsScope(t *testing.T) {
	cases := map[string]string{
		"A.build":   "build",
		"build":     "build",
		"Foo.bar.b": "b",
		"":          "",
		"Type":      "Type",
	}
	for in, want := range cases {
		if got := bareName(in); got != want {
			t.Errorf("bareName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMissingSymbols(t *testing.T) {
	doc := "## Greet\n\n`Greet(name)` returns a greeting. See Also.\n"
	names := []string{"Greet", "Repair", "Also"}
	got := missingSymbols(doc, names)
	if len(got) != 1 || got[0] != "Repair" {
		t.Fatalf("missingSymbols = %v, want [Repair]", got)
	}
}

func TestMissingSymbolsIsCaseInsensitive(t *testing.T) {
	got := missingSymbols("the connect method", []string{"Connect"})
	if len(got) != 0 {
		t.Errorf("case-insensitive match failed: %v", got)
	}
}

// statusFixture builds a repo whose README documents only one of two symbols.
func statusFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	write(t, filepath.Join(dir, "src", "api.go"),
		"package src\n\nfunc Greet(name string) string { return name }\n\nfunc Repair(id int) error { return nil }\n")
	write(t, filepath.Join(dir, "README.md"), "# API\n\n## Greet\n\n`Greet(name)` returns a greeting.\n")
	write(t, filepath.Join(dir, ".driftlock.toml"), strings.Join([]string{
		`[[doc_mapping]]`,
		`sources = ["src/**"]`,
		`docs = ["README.md"]`,
		``,
	}, "\n"))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "base")
	return dir
}

// Informational by default: a repository with gaps must not fail a developer's
// shell prompt or a casual `driftlock status`.
func TestStatusIsInformationalByDefault(t *testing.T) {
	dir := statusFixture(t)
	t.Chdir(dir)

	if err := StatusCheck(StatusOptions{}); err != nil {
		t.Fatalf("status should be informational by default, got %v", err)
	}
}

// Strict mode turns the same report into a failure, for a periodic CI job.
func TestStatusStrictFailsOnMissingSymbols(t *testing.T) {
	dir := statusFixture(t)
	t.Chdir(dir)

	err := StatusCheck(StatusOptions{Strict: true})
	if err == nil {
		t.Fatal("strict status should fail when a symbol is undocumented")
	}
	if !strings.Contains(err.Error(), "not documented") {
		t.Errorf("unexpected error: %v", err)
	}
}

// A fully documented repository passes even in strict mode.
func TestStatusStrictPassesWhenFullyDocumented(t *testing.T) {
	dir := statusFixture(t)
	write(t, filepath.Join(dir, "README.md"),
		"# API\n\n## Greet\n\n`Greet(name)` returns a greeting.\n\n## Repair\n\n`Repair(id)` repairs.\n")
	t.Chdir(dir)

	if err := StatusCheck(StatusOptions{Strict: true}); err != nil {
		t.Fatalf("fully documented repository failed strict status: %v", err)
	}
}

// status must not consult a model: the fixture has no [llm] section at all, so
// any attempt to build a provider would error.
func TestStatusDoesNotRequireAProvider(t *testing.T) {
	dir := statusFixture(t)
	t.Chdir(dir)

	err := StatusCheck(StatusOptions{})
	if err != nil && !errors.Is(err, ErrDrift) {
		t.Fatalf("status required a provider: %v", err)
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".driftlock", "cache.json")); statErr == nil {
		t.Error("status wrote a verdict cache; it should not touch the LLM path")
	}
}
