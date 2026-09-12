package parser_test

import (
	"testing"

	"github.com/Ksschkw/driftlock/internal/parser"
)

// TypeScript and JavaScript class methods are usually written with NO access
// modifier (`run(x) { … }`). The only method pattern required a modifier word,
// so the dominant style was invisible and a method addition/removal produced
// no structural change at all.
func TestTypeScriptUnmodifiedClassMethods(t *testing.T) {
	source := `class Service {
  constructor(private readonly url: string) {}

  run(x: number): string {
    if (x > 0) {
      return this.format(x);
    }
    for (let i = 0; i < x; i++) {
      this.log(i);
    }
    return "";
  }

  private format(x: number) { return String(x); }

  public static create(url: string) { return new Service(url); }

  get size() { return 0; }
}
`
	got := names(parser.ExtractSignatures("service.ts", source))
	for _, want := range []string{"Service", "constructor", "run", "format", "create", "size"} {
		if !got[want] {
			t.Errorf("expected method %q to be extracted, got %v", want, got)
		}
	}
	// Statement keywords and calls must not leak in as symbols.
	for _, phantom := range []string{"if", "for", "return", "log", "String", "let", "this"} {
		if got[phantom] {
			t.Errorf("phantom symbol %q extracted from a statement", phantom)
		}
	}
}

// Multi-modifier methods (`public static run`) were missed because the modifier
// group matched exactly one modifier.
func TestTypeScriptMultiModifierMethod(t *testing.T) {
	source := `class Factory {
  public static async create(name: string): Promise<Factory> {
    return new Factory();
  }
}
`
	got := names(parser.ExtractSignatures("factory.ts", source))
	if !got["create"] {
		t.Errorf("multi-modifier method not extracted, got %v", got)
	}
}

// Plain JavaScript object-literal methods should be found, while a call whose
// argument is an anonymous function must not be mistaken for a declaration.
func TestJavaScriptObjectMethodAndAnonymousFunction(t *testing.T) {
	source := `const handlers = {
  onClick(event) {
    doSomething(event);
  },
};

items.forEach(function () {
  tick();
});
`
	got := names(parser.ExtractSignatures("handlers.js", source))
	if !got["onClick"] {
		t.Errorf("object-literal method not extracted, got %v", got)
	}
	for _, phantom := range []string{"doSomething", "forEach", "function", "tick", "items"} {
		if got[phantom] {
			t.Errorf("phantom symbol %q extracted", phantom)
		}
	}
}

// Statement words such as `delete` are legal member names in JavaScript. The
// keyword filter must not swallow them, while genuine statement lines stay
// rejected by the span-start guard.
func TestJavaScriptStatementWordMethodsAreKept(t *testing.T) {
	source := `const store = {
  delete(key) { return key; },
  get(key) { return key; },
};

function run(key) {
  delete store[key];
  return store.get(key);
}
`
	got := names(parser.ExtractSignatures("store.js", source))
	for _, want := range []string{"delete", "get", "run"} {
		if !got[want] {
			t.Errorf("expected member %q to be extracted, got %v", want, got)
		}
	}
}
