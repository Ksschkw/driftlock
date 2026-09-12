package diff

import (
	"strings"
	"testing"
)

func changeByKind(changes []StructuralChange, kind string) []StructuralChange {
	var out []StructuralChange
	for _, c := range changes {
		if c.Change == kind {
			out = append(out, c)
		}
	}
	return out
}

func TestAddedRemovedModified(t *testing.T) {
	oldSrc := `package p
func Kept(a int) {}
func Removed(b int) {}
func Changed(c int) {}
`
	newSrc := `package p
func Kept(a int) {}
func Changed(c int, d string) {}
func Added(e int) {}
`
	changes := ExtractStructuralChanges("p.go", oldSrc, newSrc)

	if got := changeByKind(changes, "added"); len(got) != 1 || got[0].NewSig == "" {
		t.Errorf("expected 1 added, got %v", got)
	}
	if got := changeByKind(changes, "removed"); len(got) != 1 {
		t.Errorf("expected 1 removed, got %v", got)
	}
	if got := changeByKind(changes, "modified"); len(got) != 1 {
		t.Errorf("expected 1 modified, got %v", got)
	}
}

// A body-only edit changes no signatures and must produce zero structural
// changes — this silence is the whole point of Driftlock.
func TestBodyOnlyEditIsSilent(t *testing.T) {
	oldSrc := `package p
func F(a int) int { return a }
`
	newSrc := `package p
func F(a int) int { return a * 2 }
`
	if changes := ExtractStructuralChanges("p.go", oldSrc, newSrc); len(changes) != 0 {
		t.Errorf("body-only edit should be silent, got %v", changes)
	}
}

func TestRenameIsRemoveAndAdd(t *testing.T) {
	oldSrc := "package p\nfunc OldName(a int) {}\n"
	newSrc := "package p\nfunc NewName(a int) {}\n"
	changes := ExtractStructuralChanges("p.go", oldSrc, newSrc)
	if len(changeByKind(changes, "added")) != 1 || len(changeByKind(changes, "removed")) != 1 {
		t.Errorf("rename should be one add + one remove, got %v", changes)
	}
}

// A Python return-annotation change must register as one modified signature.
// Previously the annotation was not part of the signature, so this was silent.
func TestPythonReturnTypeChangeIsModified(t *testing.T) {
	oldSrc := "def parse(text: str) -> int:\n    return 1\n"
	newSrc := "def parse(text: str) -> str:\n    return \"x\"\n"
	changes := ExtractStructuralChanges("parse.py", oldSrc, newSrc)
	got := changeByKind(changes, "modified")
	if len(got) != 1 {
		t.Fatalf("expected 1 modified for a return-type change, got %v", changes)
	}
	if !strings.Contains(got[0].NewSig, "-> str") {
		t.Errorf("modified signature does not show the new return type: %q", got[0].NewSig)
	}
}

// A Rust return-type change must register as one modified signature.
func TestRustReturnTypeChangeIsModified(t *testing.T) {
	oldSrc := "pub fn parse(s: &str) -> i32 { 0 }\n"
	newSrc := "pub fn parse(s: &str) -> String { String::new() }\n"
	changes := ExtractStructuralChanges("lib.rs", oldSrc, newSrc)
	got := changeByKind(changes, "modified")
	if len(got) != 1 {
		t.Fatalf("expected 1 modified for a return-type change, got %v", changes)
	}
	if !strings.Contains(got[0].NewSig, "-> String") {
		t.Errorf("modified signature does not show the new return type: %q", got[0].NewSig)
	}
}

// Deleting one of two same-named methods on different receiver types must be a
// single `removed`, not a bogus `modified` comparing the two unrelated methods,
// and not silence.
func TestSameNameMethodRemovalIsRemoved(t *testing.T) {
	oldSrc := `package p

type A struct{}
type B struct{}

func (a *A) Close() error { return nil }
func (b *B) Close() error { return nil }
`
	newSrc := `package p

type A struct{}
type B struct{}

func (b *B) Close() error { return nil }
`
	changes := ExtractStructuralChanges("m.go", oldSrc, newSrc)
	if got := changeByKind(changes, "removed"); len(got) != 1 {
		t.Fatalf("expected 1 removed, got %v", changes)
	}
	if got := changeByKind(changes, "modified"); len(got) != 0 {
		t.Errorf("expected no modified, got %v", got)
	}
}

// Deleting one Java overload while another survives must be reported.
func TestJavaOverloadRemovalIsRemoved(t *testing.T) {
	oldSrc := `class H {
    public int f(int a) { return a; }
    public int f(String s) { return 0; }
}
`
	newSrc := `class H {
    public int f(int a) { return a; }
}
`
	changes := ExtractStructuralChanges("H.java", oldSrc, newSrc)
	if got := changeByKind(changes, "removed"); len(got) != 1 {
		t.Fatalf("expected 1 removed overload, got %v", changes)
	}
}

// Changing exactly one of two same-named methods is a single `modified`.
func TestSameNameMethodEditIsModified(t *testing.T) {
	oldSrc := `package p

type A struct{}
type B struct{}

func (a *A) Close() error { return nil }
func (b *B) Close() error { return nil }
`
	newSrc := `package p

type A struct{}
type B struct{}

func (a *A) Close(timeout int) error { return nil }
func (b *B) Close() error { return nil }
`
	changes := ExtractStructuralChanges("m.go", oldSrc, newSrc)
	if got := changeByKind(changes, "modified"); len(got) != 1 {
		t.Fatalf("expected 1 modified, got %v", changes)
	}
	if got := changeByKind(changes, "removed"); len(got) != 0 {
		t.Errorf("expected no removed, got %v", got)
	}
}

// Output order must be stable because the formatted diff feeds the LLM cache
// key; a random map order would make identical runs miss the cache.
func TestChangeOrderIsDeterministic(t *testing.T) {
	oldSrc := "package p\nfunc Zeta() {}\nfunc Alpha() {}\nfunc Mid() {}\n"
	newSrc := "package p\n"
	first := ExtractStructuralChanges("p.go", oldSrc, newSrc)
	for i := 0; i < 20; i++ {
		again := ExtractStructuralChanges("p.go", oldSrc, newSrc)
		for j := range first {
			if first[j] != again[j] {
				t.Fatalf("change order unstable at %d: %v vs %v", j, first, again)
			}
		}
	}
}

// The formatted diff labels each change with the parser's (possibly qualified)
// symbol name. Without it, two Rust impl blocks declaring an identical
// signature are indistinguishable to the reader and to the LLM.
func TestFormatShowsQualifiedName(t *testing.T) {
	oldSrc := `impl A {
    pub fn build() -> Self { A }
}

impl B {
    pub fn build() -> Self { B }
}
`
	newSrc := `impl A {
    pub fn build() -> Self { A }
}
`
	changes := ExtractStructuralChanges("m.rs", oldSrc, newSrc)
	if len(changes) != 1 || changes[0].Name != "B.build" {
		t.Fatalf("expected one B.build change, got %+v", changes)
	}
	out := FormatStructuralChanges(changes)
	if !strings.Contains(out, "B.build") {
		t.Errorf("formatted diff does not name the removed symbol: %q", out)
	}
}

// A PHP return-type change must register as one modified signature.
func TestPhpReturnTypeChangeIsModified(t *testing.T) {
	oldSrc := "<?php\nfunction get(): int { return 1; }\n"
	newSrc := "<?php\nfunction get(): string { return \"\"; }\n"
	changes := ExtractStructuralChanges("api.php", oldSrc, newSrc)
	got := changeByKind(changes, "modified")
	if len(got) != 1 {
		t.Fatalf("expected 1 modified for a PHP return-type change, got %v", changes)
	}
	if !strings.Contains(got[0].NewSig, ": string") {
		t.Errorf("modified signature does not show the new return type: %q", got[0].NewSig)
	}
}
