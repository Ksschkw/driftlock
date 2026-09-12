package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Overlapping source globs match the same file more than once. It must appear
// once per document, or its structural changes and audit entries are duplicated.
func TestResolveDocMappingDeduplicatesSources(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# doc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries := []DocMapEntry{{
		Sources: []string{"src/**", "src/*.go"},
		Docs:    []string{"README.md"},
	}}
	staged := []string{"src/a.go"}

	docMap, err := ResolveDocMapping(entries, staged, root)
	if err != nil {
		t.Fatal(err)
	}
	got := docMap["README.md"]
	if len(got) != 1 {
		t.Fatalf("source recorded %d times, want 1: %v", len(got), got)
	}
}

// Two mapping entries pointing at the same document must not duplicate either.
func TestResolveDocMappingDeduplicatesAcrossEntries(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# doc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries := []DocMapEntry{
		{Sources: []string{"src/**"}, Docs: []string{"README.md"}},
		{Sources: []string{"src/a.go"}, Docs: []string{"README.md"}},
	}

	docMap, err := ResolveDocMapping(entries, []string{"src/a.go"}, root)
	if err != nil {
		t.Fatal(err)
	}
	if got := docMap["README.md"]; len(got) != 1 {
		t.Fatalf("source recorded %d times, want 1: %v", len(got), got)
	}
}

// Distinct sources mapping to one document are all preserved.
func TestResolveDocMappingKeepsDistinctSources(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# doc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries := []DocMapEntry{{Sources: []string{"src/**"}, Docs: []string{"README.md"}}}

	docMap, err := ResolveDocMapping(entries, []string{"src/a.go", "src/b.go"}, root)
	if err != nil {
		t.Fatal(err)
	}
	if got := docMap["README.md"]; len(got) != 2 {
		t.Fatalf("expected 2 distinct sources, got %v", got)
	}
}
