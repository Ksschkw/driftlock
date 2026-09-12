package parser_test

import (
	"strings"
	"testing"

	"github.com/Ksschkw/driftlock/internal/parser"
)

// A Python return annotation (`-> int`) was not part of the extracted
// signature, so changing only the return type produced no structural change
// and the documentation was never checked. The README promises that a changed
// return type triggers a check; for Python it did not.
func TestPythonReturnAnnotationInSignature(t *testing.T) {
	source := "def parse(text: str, limit: int = 10) -> dict[str, int]:\n    return {}\n"
	sigs := parser.ExtractSignatures("parse.py", source)
	sig, ok := findBy(sigs, "parse")
	if !ok {
		t.Fatalf("parse not extracted; got %v", names(sigs))
	}
	if !strings.Contains(sig.Signature, "-> dict[str, int]") {
		t.Errorf("return annotation missing from signature: %q", sig.Signature)
	}
}

func TestPythonAsyncReturnAnnotation(t *testing.T) {
	source := "async def fetch(url: str) -> bytes:\n    return b\"\"\n"
	sigs := parser.ExtractSignatures("fetch.py", source)
	sig, ok := findBy(sigs, "fetch")
	if !ok {
		t.Fatalf("fetch not extracted; got %v", names(sigs))
	}
	if !strings.Contains(sig.Signature, "-> bytes") {
		t.Errorf("async return annotation missing: %q", sig.Signature)
	}
}

// A '{' or ';' inside a Python parameter default used to truncate the
// signature because the C-style brace/semicolon cut was applied to every
// language. `def f(x: dict = {})` became `def f(x: dict = `.
func TestPythonDefaultsWithBracesDoNotTruncate(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"dict default", "def f(x: dict = {}) -> dict:\n    return x\n", "x: dict = {}"},
		{"set default", "def g(items: set = {1, 2}):\n    return items\n", "items: set = {1, 2}"},
		{"nested call default", "def h(n: int = len(\"abc\")) -> int:\n    return n\n", "n: int = len("},
		{"lambda default", "def k(cb=lambda: None) -> None:\n    pass\n", "cb=lambda: None"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sigs := parser.ExtractSignatures("d.py", tc.src)
			if len(sigs) != 1 {
				t.Fatalf("expected 1 signature, got %v", sigs)
			}
			if !strings.Contains(sigs[0].Signature, tc.want) {
				t.Errorf("signature %q missing %q", sigs[0].Signature, tc.want)
			}
			if strings.HasSuffix(strings.TrimSpace(sigs[0].Signature), "=") {
				t.Errorf("signature truncated at a brace/semicolon: %q", sigs[0].Signature)
			}
		})
	}
}
