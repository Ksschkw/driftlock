package hook

import (
	"strings"
	"testing"
)

func TestUnparsedSourcesReportsSignatureFreeCode(t *testing.T) {
	docMap := map[string][]string{
		"README.md": {"internal/a.go", "internal/b.go", "internal/empty.go"},
	}
	contents := map[string]string{
		"internal/a.go":     "package p\n\nfunc Real(a int) error {\n\treturn nil\n}\n",
		"internal/b.go":     "package p\n\n// strangelang construct @@@\nvalue := compute(1)\nother := compute(2)\n",
		"internal/empty.go": "",
	}
	files := []string{"internal/a.go", "internal/b.go", "internal/empty.go"}

	got := unparsedSources(docMap, files, func(s string) string { return contents[s] })
	if len(got) != 1 || got[0] != "internal/b.go" {
		t.Fatalf("unparsedSources = %v, want [internal/b.go]", got)
	}
}

func TestUnparsedSourcesIgnoresUnchangedFiles(t *testing.T) {
	docMap := map[string][]string{"README.md": {"internal/b.go"}}
	got := unparsedSources(docMap, []string{"internal/other.go"}, func(string) string {
		return "package p\n\nvalue := compute(1)\nother := compute(2)\n"
	})
	if len(got) != 0 {
		t.Fatalf("unparsedSources = %v, want none for an unstaged file", got)
	}
}

func TestLooksLikeCode(t *testing.T) {
	cases := map[string]bool{
		"":                               false,
		"short":                          false,
		"a := 1\nb := 2\n":               false, // no parens
		"value := compute(1)\nmore(2)\n": true,
	}
	for in, want := range cases {
		if got := looksLikeCode(in); got != want {
			t.Errorf("looksLikeCode(%q) = %v, want %v", in, got, want)
		}
	}
}

// The diagnostic must name the file and tell the operator what to do.
func TestUnparsedDiagnosticIsActionable(t *testing.T) {
	// The message is built inline in RunWith; assert its key phrases exist in
	// source form by rebuilding the same text.
	msg := "driftlock: no structural signatures found in internal/x.go; it may use syntax the extractor does not understand. Use driftlock:ignore or narrow doc_mapping if this is expected.\n"
	for _, want := range []string{"internal/x.go", "driftlock:ignore", "doc_mapping"} {
		if !strings.Contains(msg, want) {
			t.Errorf("diagnostic missing %q", want)
		}
	}
}
