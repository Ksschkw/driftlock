package parser_test

import (
	"testing"

	"github.com/Ksschkw/driftlock/internal/parser"
)

func TestGoSignatures(t *testing.T) {
	source := `
package main

func Hello(name string) string {
	return "Hello " + name
}

func (u *User) SetName(n string) {
	u.Name = n
}
`
	sigs := parser.ExtractSignatures("main.go", source)
	if len(sigs) < 2 {
		t.Fatalf("expected at least 2 signatures, got %d", len(sigs))
	}
	names := map[string]bool{"Hello": false, "SetName": false}
	for _, s := range sigs {
		if _, ok := names[s.Name]; ok {
			names[s.Name] = true
		}
	}
	for n, found := range names {
		if !found {
			t.Errorf("missing function %s", n)
		}
	}
}

func TestPythonSignatures(t *testing.T) {
	source := `
def greet(name: str) -> str:
    return "Hi " + name

def add(a: int, b: int = 0) -> int:
    return a + b
`
	sigs := parser.ExtractSignatures("test.py", source)
	if len(sigs) < 2 {
		t.Fatalf("expected at least 2 signatures, got %d", len(sigs))
	}
}

func TestJavaScriptSignatures(t *testing.T) {
	source := `
function foo(a,b) {
    return a + b;
}
const bar = (x) => x * 2;
`
	sigs := parser.ExtractSignatures("script.js", source)
	if len(sigs) < 2 {
		t.Fatalf("expected at least 2 signatures, got %d", len(sigs))
	}
}

func TestJavaClassAndMethod(t *testing.T) {
	source := `
public class Calculator {
    public int add(int a, int b) {
        return a + b;
    }
    private void log(String msg) {
        System.out.println(msg);
    }
}
`
	sigs := parser.ExtractSignatures("Calculator.java", source)
	if len(sigs) < 3 {
		t.Fatalf("expected at least 3 signatures (class + 2 methods), got %d", len(sigs))
	}
}

func TestJSONKeys(t *testing.T) {
	source := `{
    "username": "john",
    "age": 30,
    "isAdmin": true
}`
	sigs := parser.ExtractSignatures("data.json", source)
	if len(sigs) < 3 {
		t.Fatalf("expected at least 3 keys, got %d", len(sigs))
	}
}

func TestYAMLKeys(t *testing.T) {
	source := `
server:
  host: localhost
  port: 8080
database:
  name: mydb
`
	sigs := parser.ExtractSignatures("config.yml", source)
	if len(sigs) < 4 {
		t.Fatalf("expected at least 4 keys, got %d", len(sigs))
	}
}

func TestSQL(t *testing.T) {
	source := `CREATE TABLE users (id INT, name VARCHAR(100));`
	sigs := parser.ExtractSignatures("schema.sql", source)
	if len(sigs) < 1 {
		t.Fatal("expected at least 1 table")
	}
}

func TestShell(t *testing.T) {
	source := `
check_status() {
    echo "OK"
}
`
	sigs := parser.ExtractSignatures("script.sh", source)
	if len(sigs) < 1 {
		t.Fatal("expected at least 1 shell function")
	}
}

func TestLua(t *testing.T) {
	source := `function foo(a, b) return a+b end`
	sigs := parser.ExtractSignatures("foo.lua", source)
	if len(sigs) < 1 {
		t.Fatal("expected at least 1 Lua function")
	}
}
