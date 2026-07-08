package config

import "testing"

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"internal/**", "internal/hook/hook.go", true},
		{"internal/**", "internal/config.go", true},
		{"internal/**", "cmd/main.go", false},
		{"cmd/**", "cmd/driftlock/main.go", true},
		{"src/**/*.go", "src/a/b/c.go", true},
		{"src/**/*.go", "src/a/b/c.py", false},
		{"src/*.go", "src/a.go", true},
		{"src/*.go", "src/a/b.go", false}, // single * does not cross separators
		{"*.md", "README.md", true},
		{"*.md", "docs/README.md", false},
		{"**/*.md", "docs/guide/intro.md", true},
		{"pkg/?.go", "pkg/a.go", true},
		{"pkg/?.go", "pkg/ab.go", false},
		{"exact/path.go", "exact/path.go", true},
		{"exact/path.go", "exact/other.go", false},
	}
	for _, c := range cases {
		if got := MatchGlob(c.pattern, c.path); got != c.want {
			t.Errorf("MatchGlob(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

func TestResolveDocMappingRecursive(t *testing.T) {
	entries := []DocMapEntry{
		{Sources: []string{"internal/**"}, Docs: []string{"README.md"}},
	}
	staged := []string{"internal/hook/hook.go", "internal/config/config.go", "cmd/main.go"}
	m, err := ResolveDocMapping(entries, staged, "/tmp")
	if err != nil {
		t.Fatal(err)
	}
	got := m["README.md"]
	if len(got) != 2 {
		t.Fatalf("expected 2 nested sources mapped to README.md, got %v", got)
	}
}
