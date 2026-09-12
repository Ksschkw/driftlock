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
