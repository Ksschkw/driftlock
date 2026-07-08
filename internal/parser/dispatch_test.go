package parser_test

import (
	"testing"

	"github.com/Ksschkw/driftlock/internal/parser"
)

func names(sigs []parser.Signature) map[string]bool {
	m := map[string]bool{}
	for _, s := range sigs {
		m[s.Name] = true
	}
	return m
}

// A Go file that contains, inside comments and strings, tokens that look like
// Python defs, YAML keys, and Markdown headings must NOT produce phantom
// signatures. This is the core false-positive regression that motivated the
// language-dispatch + sanitization rewrite.
func TestNoFalsePositivesFromCommentsAndStrings(t *testing.T) {
	source := `package main

// This comment mentions def phantom(): and key: value and # Heading
/* block: def alsoPhantom() and more: stuff */

func Real(a int) error {
	msg := "def stringDef(): and yaml_key: value inside a string"
	m := map[string]int{"jsonish": 1}
	_ = msg
	_ = m
	return nil
}
`
	got := names(parser.ExtractSignatures("main.go", source))
	if !got["Real"] {
		t.Fatalf("expected to find Real, got %v", got)
	}
	for _, phantom := range []string{"phantom", "alsoPhantom", "stringDef", "key", "yaml_key", "jsonish", "block", "more"} {
		if got[phantom] {
			t.Errorf("phantom signature %q leaked from a comment/string", phantom)
		}
	}
}

// YAML "key:" patterns must never run against source-code files.
func TestGoFileDoesNotMatchYAMLKeys(t *testing.T) {
	source := `package config

type Server struct {
	Host string
	Port int
}
`
	got := names(parser.ExtractSignatures("config.go", source))
	if !got["Server"] {
		t.Errorf("expected Server type, got %v", got)
	}
	// "Host"/"Port" are struct fields, not top-level structural symbols, and
	// must not be captured as YAML keys.
	if got["Host"] || got["Port"] {
		t.Errorf("struct fields leaked as signatures: %v", got)
	}
}

func TestIgnoreAnnotationInline(t *testing.T) {
	source := `package main

func Kept(a int) {}
func Hidden(b int) {} // driftlock:ignore
func AlsoKept(c int) {}
`
	got := names(parser.ExtractSignatures("main.go", source))
	if got["Hidden"] {
		t.Errorf("inline driftlock:ignore did not suppress Hidden: %v", got)
	}
	if !got["Kept"] || !got["AlsoKept"] {
		t.Errorf("inline ignore over-suppressed neighbors: %v", got)
	}
}

func TestIgnoreAnnotationStandalone(t *testing.T) {
	source := `package main

// driftlock:ignore
func Hidden(a int) {}
func Kept(b int) {}
`
	got := names(parser.ExtractSignatures("main.go", source))
	if got["Hidden"] {
		t.Errorf("standalone driftlock:ignore did not suppress the next declaration: %v", got)
	}
	if !got["Kept"] {
		t.Errorf("standalone ignore wrongly suppressed Kept: %v", got)
	}
}

func TestMultiLineGoSignature(t *testing.T) {
	source := `package main

func Complicated(
	a int,
	b string,
	c float64,
) (string, error) {
	return "", nil
}
`
	sigs := parser.ExtractSignatures("main.go", source)
	if !names(sigs)["Complicated"] {
		t.Fatalf("expected Complicated across multiple lines, got %v", names(sigs))
	}
}

func TestTypeScriptInterfacesAndTypes(t *testing.T) {
	source := `export interface User {
	id: number;
	name: string;
}

export type ID = string;

export function makeUser(name: string): User {
	return { id: 1, name };
}
`
	got := names(parser.ExtractSignatures("user.ts", source))
	for _, want := range []string{"User", "ID", "makeUser"} {
		if !got[want] {
			t.Errorf("expected %q in %v", want, got)
		}
	}
}

func TestRustSignatures(t *testing.T) {
	source := `pub fn process(data: &[u8]) -> Result<(), Error> {
	Ok(())
}

struct Config {
	name: String,
}
`
	got := names(parser.ExtractSignatures("lib.rs", source))
	if !got["process"] || !got["Config"] {
		t.Errorf("expected process and Config, got %v", got)
	}
}

// Unknown extensions fall back to the universal code spec, which should still
// find obvious functions but never emit data-language noise.
func TestUnknownExtensionFallback(t *testing.T) {
	source := `func Handler(w Writer) {}
def helper(x):
    pass
`
	got := names(parser.ExtractSignatures("mystery.xyz", source))
	if !got["Handler"] || !got["helper"] {
		t.Errorf("universal fallback missed functions: %v", got)
	}
}
