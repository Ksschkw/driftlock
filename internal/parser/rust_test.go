package parser_test

import (
	"strings"
	"testing"

	"github.com/Ksschkw/driftlock/internal/parser"
)

// A Rust return type was not part of the extracted signature, so changing only
// the return type produced no structural change and the docs were not checked.
func TestRustReturnTypeInSignature(t *testing.T) {
	source := `pub fn process(data: &[u8]) -> Result<(), Error> {
    Ok(())
}
`
	sigs := parser.ExtractSignatures("lib.rs", source)
	sig, ok := findBy(sigs, "process")
	if !ok {
		t.Fatalf("process not extracted; got %v", names(sigs))
	}
	if !strings.Contains(sig.Signature, "-> Result<(), Error>") {
		t.Errorf("return type missing from Rust signature: %q", sig.Signature)
	}
}

func TestRustWhereClauseAndAsync(t *testing.T) {
	source := `pub async fn load<T>(key: &str) -> Option<T>
where
    T: Clone,
{
    None
}
`
	sigs := parser.ExtractSignatures("store.rs", source)
	sig, ok := findBy(sigs, "load")
	if !ok {
		t.Fatalf("load not extracted; got %v", names(sigs))
	}
	if !strings.Contains(sig.Signature, "-> Option<T>") {
		t.Errorf("return type missing with where clause: %q", sig.Signature)
	}
}

// A closure-typed parameter contains a ')' inside the parameter list; the old
// non-greedy parameter match stopped there and truncated the signature.
func TestRustClosureParameterNotTruncated(t *testing.T) {
	source := `pub fn apply(x: i32, f: impl Fn(i32) -> i32) -> i32 {
    f(x)
}
`
	sigs := parser.ExtractSignatures("apply.rs", source)
	sig, ok := findBy(sigs, "apply")
	if !ok {
		t.Fatalf("apply not extracted; got %v", names(sigs))
	}
	if !strings.Contains(sig.Signature, "f: impl Fn(i32) -> i32") {
		t.Errorf("closure parameter truncated: %q", sig.Signature)
	}
}
