package parser_test

import (
	"strings"
	"testing"

	"github.com/Ksschkw/driftlock/internal/parser"
)

// Swift, Kotlin, and Scala declare return types inline; they were omitted from
// signatures, so a return-type-only change was invisible.
func TestSwiftKotlinScalaReturnTypes(t *testing.T) {
	cases := []struct {
		lang string
		file string
		src  string
		name string
		want string
	}{
		{"swift", "a.swift", "func fetch(url: String) -> Data {\n    return Data()\n}\n", "fetch", "-> Data"},
		{"kotlin", "a.kt", "fun transform(input: List<Int>): List<String> {\n    return emptyList()\n}\n", "transform", ": List<String>"},
		{"scala", "a.scala", "def isReady(state: String): Boolean = {\n    true\n}\n", "isReady", ": Boolean"},
	}
	for _, tc := range cases {
		t.Run(tc.lang, func(t *testing.T) {
			sigs := parser.ExtractSignatures(tc.file, tc.src)
			sig, ok := findBy(sigs, tc.name)
			if !ok {
				t.Fatalf("%s not extracted; got %v", tc.name, names(sigs))
			}
			if !strings.Contains(sig.Signature, tc.want) {
				t.Errorf("return type missing from signature: %q", sig.Signature)
			}
		})
	}
}

// A function-typed parameter contains parentheses; the parameter list must not
// truncate at them.
func TestSwiftFunctionTypedParameter(t *testing.T) {
	source := "func onDone(handler: (Int) -> Void, label: String) {\n}\n"
	sigs := parser.ExtractSignatures("cb.swift", source)
	sig, ok := findBy(sigs, "onDone")
	if !ok {
		t.Fatalf("onDone not extracted; got %v", names(sigs))
	}
	if !strings.Contains(sig.Signature, "label: String") {
		t.Errorf("parameter list truncated at a function-typed parameter: %q", sig.Signature)
	}
}
