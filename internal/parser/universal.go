package parser

import (
	"regexp"
	"strings"
)

// universalPatterns is an ordered list of regular expressions that capture
// function, method, class, interface, struct, and other structural
// declarations from source code in virtually any programming language
// (and many markup/data formats).
//
// The patterns are applied in order. For each match, the "human‑readable
// name" of the declaration is extracted and a signature string is built.
// Duplicate names (often caused by overlapping patterns) are ignored so
// that each structural element appears only once in the final diff.
var universalPatterns = []*regexp.Regexp{

	// ── C‑style functions ──────────────────────────────────────────
	// int foo(int a, char b)    → name "foo"
	// Also handles constructors, destructors, and attributes.
	regexp.MustCompile(
		`(?m)^[\t ]*(?:(?:static|inline|virtual|explicit|export|constexpr|noexcept|\[\[[^]]+\]\])\s+)*` +
			`(?:(?:unsigned|signed|short|long|long long|int|char|float|double|void|bool|wchar_t|size_t|ptrdiff_t|int\d+_t|uint\d+_t|auto)\s+)+` +
			`(?:\*\s*|&\s*)*` +
			`(\w+)\s*\(([^)]*)\)\s*(?:(?:const|override|final)\s*)*`,
	),

	// ── Go functions and methods ────────────────────────────────────
	// func Hello(name string) string               → name "Hello"
	// func (r *Receiver) Method(a int) error       → name "Method"
	regexp.MustCompile(`(?m)^[\t ]*func\s+(?:\(\s*\w+\s+\*?\w+\s*\)\s+)?(\w+)\s*\(([^)]*)\)\s*(?:\([^)]*\)|[\w\[\],*&]+)*`),

	// ── Python / Ruby "def" ─────────────────────────────────────────
	// def my_func(param1, param2):   → name "my_func"
	regexp.MustCompile(`(?m)^[\t ]*def\s+(\w+)\s*\(([^)]*)\)`),

	// ── Rust "fn" ───────────────────────────────────────────────────
	// fn process(data: &[u8]) -> Result<(), Error>   → name "process"
	regexp.MustCompile(`(?m)^[\t ]*(?:pub\s+)?fn\s+(\w+)\s*\(([^)]*)\)(?:\s*->\s*[^{]+)?`),

	// ── JavaScript / TypeScript arrow functions ─────────────────────
	// const add = (a, b) => { ... }   → name "add"
	regexp.MustCompile(`(?m)(?:const|let|var)\s+(\w+)\s*=\s*\(?([^)]*)\)?\s*=>`),

	// ── Classic "function" keyword (JS, Lua, PHP, Julia, …) ─────────
	regexp.MustCompile(`(?m)\bfunction\s+(\w+)\s*\(([^)]*)\)`),

	// ── Java / C# / Scala / Kotlin methods ──────────────────────────
	// public int calculate(int x) { ... }   → name "calculate"
	regexp.MustCompile(
		`(?m)^[\t ]*(?:(?:public|private|protected|internal|static|final|abstract|override|virtual|async)\s+)*` +
			`(?:\w+(?:<[^>]+>)?\s+)` +
			`(\w+)\s*\(([^)]*)\)`,
	),

	// ── Template / generic functions (C++, Java, C#, Rust, …) ───────
	// template<typename T> void sort(T* a, int n)   → name "sort"
	regexp.MustCompile(`(?m)\btemplate\s*<[^>]+>\s*` +
		`(?:(?:static|inline|constexpr|virtual|export)\s+)*` +
		`(?:\w+(?:<[^>]+>)?\s+)+` +
		`(\w+)\s*\(([^)]*)\)`,
	),

	// ── Class / struct / interface / trait / enum / object / record ──
	// class UserService { … }            → name "UserService"
	// public class Calculator { … }      → name "Calculator"
	regexp.MustCompile(
		`(?m)^[\t ]*(?:(?:public|private|protected|export|abstract|sealed|final|open|data|case)\s+)*` +
			`\b(class|struct|interface|trait|enum|object|record|module|impl)\s+(\w+)`,
	),

	// ── Preprocessor #define ────────────────────────────────────────
	regexp.MustCompile(`(?m)^[\t ]*#\s*define\s+(\w+)`),

	// ── SQL DDL ─────────────────────────────────────────────────────
	// CREATE TABLE users ( … )   → name "users"
	regexp.MustCompile(`(?i)\bCREATE\s+(TABLE|VIEW|PROCEDURE|FUNCTION|TRIGGER|INDEX)\s+(\w+)`),

	// ── Shell / Bash functions ──────────────────────────────────────
	// my_func() { … }   → name "my_func"
	regexp.MustCompile(`(?m)^[\t ]*(\w+)\s*\(\)\s*\{`),

	// ── YAML / JSON keys (any nesting level) ────────────────────────
	regexp.MustCompile(`(?m)^[\t ]*"?(\w+)"?\s*:`),

	// ── XML / HTML tags ─────────────────────────────────────────────
	regexp.MustCompile(`<\s*(\w+)(?:\s[^>]*)?>`),

	// ── Swift "func" ────────────────────────────────────────────────
	regexp.MustCompile(`(?m)\bfunc\s+(\w+)\s*\(([^)]*)\)(?:\s*->\s*[^{]+)?`),

	// ── Kotlin "fun" ────────────────────────────────────────────────
	regexp.MustCompile(`(?m)\bfun\s+(\w+)\s*\(([^)]*)\)(?:\s*:\s*[^{]+)?`),

	// ── Scala "def" ─────────────────────────────────────────────────
	regexp.MustCompile(`(?m)\bdef\s+(\w+)\s*\(([^)]*)\)(?:\s*:\s*[^=]+)?\s*=`),

	// ── Lisp / Clojure (defn) ───────────────────────────────────────
	regexp.MustCompile(`\(\s*defn\s+([\w\-!]+)`),

	// ── Markdown headings ───────────────────────────────────────────
	regexp.MustCompile(`(?m)^[\t ]*#{1,6}\s+(.*)`),

	// ── INI / TOML sections ─────────────────────────────────────────
	regexp.MustCompile(`(?m)^[\t ]*\[\s*(\w+)\s*\]`),
}

// extractSignaturesUniversal applies all universal patterns to the source
// text and returns a deduplicated slice of signatures.
func extractSignaturesUniversal(source string) []Signature {
	seen := make(map[string]bool)
	var sigs []Signature

	for _, re := range universalPatterns {
		for _, match := range re.FindAllStringSubmatch(source, -1) {
			name, full := extractNameAndFull(match)
			if name == "" || isIgnoredKeyword(name) {
				continue
			}
			if seen[name] {
				continue
			}
			seen[name] = true

			sig := cleanSignature(full)
			sigs = append(sigs, Signature{Name: name, Signature: sig})
		}
	}
	return sigs
}

// extractNameAndFull deduces the human‑readable name from regex match groups.
//
// Many patterns capture two groups: a keyword (like "class" or "struct") and
// an identifier. This function returns the identifier as the name.
// For patterns that only capture a name (single group), that group is returned.
func extractNameAndFull(match []string) (string, string) {
	if len(match) == 0 {
		return "", ""
	}
	full := match[0]

	// Patterns with two capture groups: m[1] is either a keyword or the name.
	// If m[1] is a known type keyword and m[2] is non‑empty, use m[2] as the name.
	if len(match) >= 3 && match[2] != "" && isTypeKeyword(match[1]) {
		return match[2], full
	}

	// Otherwise, the first group (m[1]) is the name.
	if len(match) >= 2 && match[1] != "" {
		return match[1], full
	}

	return "", full
}

// isTypeKeyword returns true if s is a keyword that introduces a type/class
// definition rather than the name itself.
func isTypeKeyword(s string) bool {
	switch s {
	case "class", "struct", "interface", "trait", "enum",
		"object", "record", "module", "impl":
		return true
	}
	return false
}

// isIgnoredKeyword filters out common control‑flow keywords that some
// patterns may accidentally match (e.g. "if", "for", "switch").
func isIgnoredKeyword(s string) bool {
	switch s {
	case "if", "for", "while", "switch", "return":
		return true
	}
	return false
}

// cleanSignature trims trailing braces, semicolons, and colons so the
// signature remains compact.
func cleanSignature(raw string) string {
	raw = strings.TrimSpace(raw)
	if idx := strings.IndexAny(raw, "{;"); idx != -1 {
		raw = strings.TrimSpace(raw[:idx])
	}
	return strings.TrimSuffix(raw, ":")
}
