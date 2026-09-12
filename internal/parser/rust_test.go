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

// Methods in different impl blocks can share a name and an identical
// signature. Without the enclosing scope they collapse into one symbol, so a
// change to one of them is invisible (or misattributed to the other).
func TestRustImplMethodsAreScopedByType(t *testing.T) {
	source := `impl A {
    pub fn build() -> Self { A }
}

impl B {
    pub fn build() -> Self { B }
}
`
	got := names(parser.ExtractSignatures("m.rs", source))
	for _, want := range []string{"A.build", "B.build"} {
		if !got[want] {
			t.Errorf("expected scoped symbol %q, got %v", want, got)
		}
	}
	if got["build"] {
		t.Errorf("unscoped %q should not appear when impl scopes exist: %v", "build", got)
	}
}

// A generic trait impl scopes to the implementing type (after `for`), not the
// trait name or the token following `impl`.
func TestRustGenericTraitImplScope(t *testing.T) {
	source := `impl<T> Store<T> for Foo<T> {
    pub fn put(&self, value: T) {}
}
`
	got := names(parser.ExtractSignatures("store.rs", source))
	if !got["Foo.put"] {
		t.Errorf("expected Foo.put, got %v", got)
	}
	if got["Store.put"] {
		t.Errorf("method scoped to the trait instead of the type: %v", got)
	}
}

// A free function is not qualified.
func TestRustFreeFunctionStaysUnqualified(t *testing.T) {
	source := "pub fn process(data: &[u8]) -> Result<(), Error> {\n    Ok(())\n}\n"
	got := names(parser.ExtractSignatures("lib.rs", source))
	if !got["process"] {
		t.Errorf("free function lost its unqualified name: %v", got)
	}
}
