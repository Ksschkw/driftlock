package parser_test

import (
	"strings"
	"testing"

	"github.com/Ksschkw/driftlock/internal/parser"
)

// PHP 7+ declares return types inline; they were dropped, so a return-type-only
// change produced no structural change at all.
func TestPhpReturnTypes(t *testing.T) {
	source := `<?php

function get(): int {
    return 1;
}

function find(): ?string {
    return null;
}

function parse(): int|string {
    return 1;
}
`
	sigs := parser.ExtractSignatures("api.php", source)
	for name, want := range map[string]string{
		"get":   ": int",
		"find":  ": ?string",
		"parse": ": int|string",
	} {
		sig, ok := findBy(sigs, name)
		if !ok {
			t.Errorf("function %q not extracted; got %v", name, names(sigs))
			continue
		}
		if !strings.Contains(sig.Signature, want) {
			t.Errorf("signature %q missing return type %q", sig.Signature, want)
		}
	}
}
