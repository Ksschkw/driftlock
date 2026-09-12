package parser_test

import (
	"testing"

	"github.com/Ksschkw/driftlock/internal/parser"
)

// countByName returns how many extracted signatures carry the given name.
func countByName(sigs []parser.Signature, name string) int {
	n := 0
	for _, s := range sigs {
		if s.Name == name {
			n++
		}
	}
	return n
}

// Two methods with the same name on different receiver types are distinct
// symbols. Keying the dedup set on the bare name discarded the second one, so a
// change to it was invisible.
func TestGoSameNamedMethodsBothSurvive(t *testing.T) {
	source := `package p

type A struct{}
type B struct{}

func (a *A) Close() error { return nil }
func (b *B) Close() error { return nil }
`
	sigs := parser.ExtractSignatures("m.go", source)
	if n := countByName(sigs, "Close"); n != 2 {
		t.Fatalf("expected 2 Close methods, got %d: %v", n, sigs)
	}
}

func TestJavaOverloadsBothSurvive(t *testing.T) {
	source := `class H {
    public int f(int a) { return a; }
    public int f(String s) { return 0; }
}
`
	sigs := parser.ExtractSignatures("H.java", source)
	if n := countByName(sigs, "f"); n != 2 {
		t.Fatalf("expected 2 f overloads, got %d: %v", n, sigs)
	}
}

// Two patterns matching the same declaration must still yield a single symbol.
func TestPatternOverlapDoesNotDuplicate(t *testing.T) {
	source := `export interface User {
    id: number;
}
`
	sigs := parser.ExtractSignatures("user.ts", source)
	if n := countByName(sigs, "User"); n != 1 {
		t.Fatalf("expected 1 User, got %d: %v", n, sigs)
	}
}
