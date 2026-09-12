package parser_test

import (
	"testing"

	"github.com/Ksschkw/driftlock/internal/parser"
)

// A structural change to a private declaration must not trigger a documentation
// check. The parser previously extracted everything, so renaming a private
// helper produced a change the LLM was asked to reconcile — noise that makes a
// blocking gate feel arbitrary.
func TestGoVisibilityFiltersUnexported(t *testing.T) {
	source := `package p

func Exported(a int) {}
func unexported(b int) {}

type Public struct{}
type private struct{}

func (s *Public) Method(x int) {}
func (s *Public) method(x int) {}
`
	got := names(parser.ExtractSignatures("v.go", source))
	for _, want := range []string{"Exported", "Public", "Method"} {
		if !got[want] {
			t.Errorf("expected exported symbol %q, got %v", want, got)
		}
	}
	for _, bad := range []string{"unexported", "private", "method"} {
		if got[bad] {
			t.Errorf("unexported symbol %q reached the extraction: %v", bad, got)
		}
	}
}

func TestPythonVisibilityFiltersUnderscore(t *testing.T) {
	source := `def public(x):
    pass


def _private(x):
    pass


class Client:
    def __init__(self):
        pass
`
	got := names(parser.ExtractSignatures("v.py", source))
	if !got["public"] || !got["Client"] {
		t.Errorf("expected public and Client, got %v", got)
	}
	if got["_private"] {
		t.Errorf("underscore-prefixed function reached the extraction: %v", got)
	}
	if got["__init__"] {
		t.Errorf("dunder method reached the extraction: %v", got)
	}
}

func TestRustVisibilityRequiresPub(t *testing.T) {
	source := `pub fn public_fn() {}

fn private_fn() {}

pub struct Public {}

struct Private {}

impl Public {
    pub fn build() -> Self { Public }

    fn internal(&self) {}
}
`
	got := names(parser.ExtractSignatures("v.rs", source))
	for _, want := range []string{"public_fn", "Public", "Public.build"} {
		if !got[want] {
			t.Errorf("expected public symbol %q, got %v", want, got)
		}
	}
	for _, bad := range []string{"private_fn", "Private", "Public.internal"} {
		if got[bad] {
			t.Errorf("non-pub symbol %q reached the extraction: %v", bad, got)
		}
	}
}
