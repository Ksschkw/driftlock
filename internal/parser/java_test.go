package parser_test

import (
	"testing"

	"github.com/Ksschkw/driftlock/internal/parser"
)

// Java methods are frequently package-private (no access modifier) and C#
// methods are frequently written without one too. The only Java pattern
// required at least one modifier, so those methods were invisible and their
// signature changes produced no structural change.
func TestJavaPackagePrivateMethods(t *testing.T) {
	source := `public class Calculator {
    int add(int a, int b) {
        return a + b;
    }

    void log(String msg);

    Calculator(int seed) {
        this.seed = seed;
    }

    public int multiply(int a, int b) { return a * b; }
}
`
	got := names(parser.ExtractSignatures("Calculator.java", source))
	for _, want := range []string{"Calculator", "add", "log", "multiply"} {
		if !got[want] {
			t.Errorf("expected %q to be extracted, got %v", want, got)
		}
	}
	for _, phantom := range []string{"return", "System", "println", "out", "a", "b", "seed"} {
		if got[phantom] {
			t.Errorf("phantom symbol %q extracted from a statement or parameter", phantom)
		}
	}
}

func TestJavaInterfaceMethods(t *testing.T) {
	source := `interface Shape {
    double area();
    int sides();
}
`
	got := names(parser.ExtractSignatures("Shape.java", source))
	for _, want := range []string{"Shape", "area", "sides"} {
		if !got[want] {
			t.Errorf("expected %q to be extracted, got %v", want, got)
		}
	}
}

// C# expression-bodied members end in `=> expr;` rather than a brace, and must
// not be missed or mistaken for statements.
func TestCSharpUnmodifiedAndExpressionBodied(t *testing.T) {
	source := `public class Repo {
    List<Item> All() { return items; }
    void Save(int id) => store.Put(id);
    int Count => items.Count;
}
`
	got := names(parser.ExtractSignatures("Repo.cs", source))
	for _, want := range []string{"Repo", "All", "Save", "Count"} {
		if !got[want] {
			t.Errorf("expected %q to be extracted, got %v", want, got)
		}
	}
	for _, phantom := range []string{"return", "store", "Put", "items"} {
		if got[phantom] {
			t.Errorf("phantom symbol %q extracted", phantom)
		}
	}
}

// Statement lines must never be read as modifier-less method declarations.
func TestJavaStatementsAreNotDeclarations(t *testing.T) {
	source := `class Body {
    void run() {
        int total = compute(a, b);
        String s = format(total);
        System.out.println(s);
        return format(a);
    }
}
`
	got := names(parser.ExtractSignatures("Body.java", source))
	for _, phantom := range []string{"compute", "format", "println", "total", "s", "return"} {
		if got[phantom] {
			t.Errorf("statement leaked as a declaration: %q in %v", phantom, got)
		}
	}
	if !got["run"] {
		t.Errorf("real method run not extracted: %v", got)
	}
}
