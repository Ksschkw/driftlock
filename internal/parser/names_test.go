package parser_test

import (
	"testing"

	"github.com/Ksschkw/driftlock/internal/parser"
)

// Name extraction used to be a heuristic: "group 1 is the name unless group 1
// looks like a type keyword, in which case group 2 is". That conflated patterns
// whose group 1 is a keyword with patterns whose group 1 is the name, so any
// declaration whose NAME happened to be a type keyword was mangled. These tests
// pin the explicit capture-group metadata that replaced the heuristic.

func TestNameNotConfusedByKeywordLikeSymbol(t *testing.T) {
	cases := []struct {
		name string
		file string
		src  string
		want string
	}{
		{"python def named type", "a.py", "def type(x):\n    pass\n", "type"},
		{"rust fn named type", "a.rs", "pub fn type(x: i32) -> i32 { x }\n", "type"},
		{"kotlin fun named type", "a.kt", "fun type(x: Int) {}\n", "type"},
		{"go method name", "a.go", "package p\n\nfunc (t *T) Object(x int) {}\n", "Object"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sigs := parser.ExtractSignatures(tc.file, tc.src)
			if _, ok := findBy(sigs, tc.want); !ok {
				t.Errorf("expected symbol %q, got %v", tc.want, names(sigs))
			}
			// The parameter list must never be mistaken for a symbol name.
			for _, s := range sigs {
				if s.Name == "x" || s.Name == "x: i32" || s.Name == "x: Int" {
					t.Errorf("parameter list leaked as name: %q", s.Name)
				}
			}
		})
	}
}

// SQL patterns capture the object KIND in group 1 and the NAME in group 2. The
// heuristic made every table a symbol literally named "TABLE".
func TestSQLNamesAreObjectNames(t *testing.T) {
	src := "CREATE TABLE users (id INT);\nCREATE VIEW active_users AS SELECT 1;\n"
	sigs := parser.ExtractSignatures("schema.sql", src)
	got := names(sigs)
	for _, want := range []string{"users", "active_users"} {
		if !got[want] {
			t.Errorf("expected SQL object %q, got %v", want, got)
		}
	}
	for _, bad := range []string{"TABLE", "VIEW"} {
		if got[bad] {
			t.Errorf("SQL keyword %q leaked as a symbol name", bad)
		}
	}
}

// Markdown captures the '#' run in group 1 and the heading text in group 2;
// group 1 as the name made every heading collapse into a symbol named "#".
func TestMarkdownHeadingNames(t *testing.T) {
	src := "# Getting Started\n\n## Installation\n"
	sigs := parser.ExtractSignatures("guide.md", src)
	got := names(sigs)
	if got["#"] {
		t.Errorf("heading marker leaked as a symbol name: %v", got)
	}
	if !got["Getting Started"] || !got["Installation"] {
		t.Errorf("heading text not used as the name: %v", got)
	}
}
