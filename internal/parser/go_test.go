package parser_test

import (
	"strings"
	"testing"

	"github.com/Ksschkw/driftlock/internal/parser"
)

// findBy returns the signature with the given name, or false.
func findBy(sigs []parser.Signature, name string) (parser.Signature, bool) {
	for _, s := range sigs {
		if s.Name == name {
			return s, true
		}
	}
	return parser.Signature{}, false
}

// Generic functions were extracted as NOTHING because the Go pattern required
// an opening paren immediately after the function name, so a type-parameter
// list (`[T any]`) defeated the match entirely. A repo using generics got a
// silent hole in its API surface.
func TestGoGenericFunction(t *testing.T) {
	source := `package p

func Map[T any, U any](xs []T, f func(T) U) []U {
	return nil
}
`
	sigs := parser.ExtractSignatures("a.go", source)
	sig, ok := findBy(sigs, "Map")
	if !ok {
		t.Fatalf("generic function Map not extracted; got %v", names(sigs))
	}
	for _, want := range []string{"[T any, U any]", "xs []T", "f func(T) U", "[]U"} {
		if !strings.Contains(sig.Signature, want) {
			t.Errorf("signature %q missing %q", sig.Signature, want)
		}
	}
}

// A generic receiver (`func (s *Stack[T]) Push`) must not hide the method.
func TestGoGenericReceiverMethod(t *testing.T) {
	source := `package p

type Stack[T any] struct{ items []T }

func (s *Stack[T]) Push(v T) {
	s.items = append(s.items, v)
}

func (s *Stack[T]) Pop() (T, bool) {
	return s.items[0], true
}
`
	sigs := parser.ExtractSignatures("stack.go", source)
	for _, want := range []string{"Push", "Pop"} {
		if _, ok := findBy(sigs, want); !ok {
			t.Errorf("generic receiver method %q not extracted; got %v", want, names(sigs))
		}
	}
}

// A func-typed parameter contains a ')' before the real end of the list. The
// old non-greedy parameter match stopped at that inner paren and emitted a
// truncated signature such as `func Apply(xs []int, f func(int`.
func TestGoFuncTypedParameterNotTruncated(t *testing.T) {
	source := `package p

func Apply(xs []int, f func(int) int) []int {
	return nil
}
`
	sigs := parser.ExtractSignatures("apply.go", source)
	sig, ok := findBy(sigs, "Apply")
	if !ok {
		t.Fatalf("Apply not extracted; got %v", names(sigs))
	}
	if !strings.Contains(sig.Signature, "f func(int) int") {
		t.Errorf("func-typed parameter truncated: %q", sig.Signature)
	}
	if !strings.HasSuffix(strings.TrimSpace(sig.Signature), "[]int") {
		t.Errorf("return type lost after func-typed parameter: %q", sig.Signature)
	}
}
