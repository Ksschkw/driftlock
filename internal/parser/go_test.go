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

// A generic type declaration (`type Stack[T any] struct`) was extracted as
// nothing because the pattern required whitespace immediately after the name,
// so the `[T any]` list between the name and the kind keyword broke the match.
func TestGoGenericTypeDeclaration(t *testing.T) {
	source := `package p

type Stack[T any] struct {
	items []T
}

type Set[T comparable] map[T]struct{}

type Handler[T any, R any] interface {
	Handle(T) (R, error)
}
`
	sigs := parser.ExtractSignatures("types.go", source)
	for _, want := range []string{"Stack", "Set", "Handler"} {
		if _, ok := findBy(sigs, want); !ok {
			t.Errorf("generic type %q not extracted; got %v", want, names(sigs))
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

// A Go type alias to a map/slice/chan keeps its element type in the signature;
// the old pattern stopped at the kind keyword, so `type Set map[T]struct{}`
// was captured as `type Set map` and a change to the element type was invisible.
func TestGoTypeExpressionFidelity(t *testing.T) {
	source := `package p

type Set[T comparable] map[T]int

type Names []string

type Stream chan int

type Fn func(int) int

type Alias = string
`
	sigs := parser.ExtractSignatures("types.go", source)
	cases := map[string]string{
		"Set":    "map[T]int",
		"Names":  "[]string",
		"Stream": "chan int",
		"Fn":     "func(int) int",
		"Alias":  "string",
	}
	for name, want := range cases {
		sig, ok := findBy(sigs, name)
		if !ok {
			t.Errorf("type %q not extracted; got %v", name, names(sigs))
			continue
		}
		if !strings.Contains(sig.Signature, want) {
			t.Errorf("type %q signature %q missing %q", name, sig.Signature, want)
		}
	}
}

// A struct/interface body is dropped, matching the multi-line form, so field
// formatting does not decide whether fields appear in the signature.
func TestGoStructBodyExcludedFromSignature(t *testing.T) {
	source := `package p

type Inline struct{ A int }

type Multiline struct {
	B string
}

type Reader interface{ Read(p []byte) (int, error) }
`
	sigs := parser.ExtractSignatures("s.go", source)
	for _, name := range []string{"Inline", "Multiline", "Reader"} {
		sig, ok := findBy(sigs, name)
		if !ok {
			t.Fatalf("type %q not extracted; got %v", name, names(sigs))
		}
		if strings.Contains(sig.Signature, "A int") || strings.Contains(sig.Signature, "B string") || strings.Contains(sig.Signature, "Read(") {
			t.Errorf("type %q signature includes a body: %q", name, sig.Signature)
		}
	}
}

// A trailing line comment must not be carried into the signature now that the
// type expression runs to end of line.
func TestGoTypeTrailingCommentExcluded(t *testing.T) {
	source := "package p\n\ntype Set[T comparable] map[T]int // element type\n\ntype Foo struct { // body\n\tA int\n}\n"
	sigs := parser.ExtractSignatures("c.go", source)
	set, ok := findBy(sigs, "Set")
	if !ok {
		t.Fatalf("Set not extracted; got %v", names(sigs))
	}
	if strings.Contains(set.Signature, "//") || strings.Contains(set.Signature, "element type") {
		t.Errorf("trailing comment leaked into signature: %q", set.Signature)
	}
	foo, ok := findBy(sigs, "Foo")
	if !ok {
		t.Fatalf("Foo not extracted; got %v", names(sigs))
	}
	if strings.Contains(foo.Signature, "//") {
		t.Errorf("trailing comment leaked into signature: %q", foo.Signature)
	}
}
