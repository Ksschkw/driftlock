package parser

import (
	"regexp"
	"strings"
)

// universalPatterns is a list of compiled regexps that match function / method /
// class / interface / struct / trait / object / macro / etc. declarations.
// Each pattern captures the name in group 1 and the full matched text in group 0.
// The patterns are tried in order, and duplicate names are skipped.
var universalPatterns = []*regexp.Regexp{
	// 1. C‑style functions:   int foo(int a, char b)    -> name "foo"
	// Also catches constructors/destructors, functions with attributes.
	regexp.MustCompile(
		`(?m)^[\t ]*(?:(?:static|inline|virtual|explicit|export|constexpr|noexcept|\[\[[^]]+\]\])\s+)*` +
			`(?:(?:unsigned|signed|short|long|long long|int|char|float|double|void|bool|wchar_t|size_t|ptrdiff_t|int\d+_t|uint\d+_t|auto)\s+)+` +
			`(?:\*\s*|&\s*)*` +
			`(\w+)\s*\(([^)]*)\)\s*(?:(?:const|override|final)\s*)*`,
	),

	// 2. Go style: func Hello(name string) string  AND  func (r *Receiver) Method(...)
	regexp.MustCompile(`(?m)^[\t ]*func\s+(?:\(\s*\w+\s+\*?\w+\s*\)\s+)?(\w+)\s*\(([^)]*)\)\s*(?:\([^)]*\)|[\w\[\],*&]+)*`),

	// 3. Python / Ruby style: def my_func(param1, param2):
	regexp.MustCompile(`(?m)^[\t ]*def\s+(\w+)\s*\(([^)]*)\)`),

	// 4. Rust `fn` keyword:   fn process(data: &[u8]) -> Result<(), Error>
	regexp.MustCompile(`(?m)^[\t ]*(?:pub\s+)?fn\s+(\w+)\s*\(([^)]*)\)(?:\s*->\s*[^{]+)?`),

	// 5. JavaScript / TypeScript arrow functions assigned to a variable:
	//    const add = (a, b) => { ... }
	regexp.MustCompile(`(?m)(?:const|let|var)\s+(\w+)\s*=\s*\(?([^)]*)\)?\s*=>`),

	// 6. Classic function keyword: function foo(a,b) { ... }
	regexp.MustCompile(`(?m)\bfunction\s+(\w+)\s*\(([^)]*)\)`),

	// 7. Java / C# / Scala / Kotlin method (modifiers + type name):
	//    public int calculate(int x) { ... }
	//    public static void main(String[] args) { ... }
	regexp.MustCompile(
		`(?m)^[\t ]*(?:(?:public|private|protected|internal|static|final|abstract|override|virtual|async)\s+)*` +
			`(?:\w+(?:<[^>]+>)?\s+)` +
			`(\w+)\s*\(([^)]*)\)`,
	),

	// 8. Generic / template function (C++, Rust, Java, C#, etc.):
	//    template<typename T> void sort(T* a, int n)
	regexp.MustCompile(`(?m)\btemplate\s*<[^>]+>\s*` +
		`(?:(?:static|inline|constexpr|virtual|export)\s+)*` +
		`(?:\w+(?:<[^>]+>)?\s+)+` +
		`(\w+)\s*\(([^)]*)\)`,
	),

	// 9. Class / struct / interface / trait / enum / object / record declaration:
	//    class UserService { ... }
	//    struct Point { ... }
	//    public class Calculator { ... }
	regexp.MustCompile(
		`(?m)^[\t ]*(?:(?:public|private|protected|export|abstract|sealed|final|open|data|case)\s+)*` +
			`\b(class|struct|interface|trait|enum|object|record|module|impl)\s+(\w+)`,
	),

	// 10. Macro / preprocessor definition (#define) – name is the macro name
	regexp.MustCompile(`(?m)^[\t ]*#\s*define\s+(\w+)`),

	// 11. SQL: CREATE TABLE / VIEW / PROCEDURE / FUNCTION
	regexp.MustCompile(`(?i)\bCREATE\s+(TABLE|VIEW|PROCEDURE|FUNCTION|TRIGGER|INDEX)\s+(\w+)`),

	// 12. Bash / shell function: function_name() { ... }
	regexp.MustCompile(`(?m)^[\t ]*(\w+)\s*\(\)\s*\{`),

	// 13. YAML / JSON key-value pair (any level): "key": value  or  key: value
	regexp.MustCompile(`(?m)^[\t ]*"?(\w+)"?\s*:`),

	// 14. XML / HTML tags (element name)
	regexp.MustCompile(`<\s*(\w+)(?:\s[^>]*)?>`),

	// 15. Lua: function foo(params) end
	regexp.MustCompile(`(?m)\bfunction\s+(\w+)\s*\(([^)]*)\)`),

	// 16. Julia: function foo(x, y) ... end
	regexp.MustCompile(`(?m)\bfunction\s+(\w+)\s*\(([^)]*)\)`),

	// 17. PHP: function foo($a, $b) { ... }
	regexp.MustCompile(`(?m)\bfunction\s+(\w+)\s*\(([^)]*)\)`),

	// 18. Swift: func foo(a: Int, b: String) -> Int { ... }
	regexp.MustCompile(`(?m)\bfunc\s+(\w+)\s*\(([^)]*)\)(?:\s*->\s*[^{]+)?`),

	// 19. Kotlin: fun foo(a: Int, b: String): Int { ... }
	regexp.MustCompile(`(?m)\bfun\s+(\w+)\s*\(([^)]*)\)(?:\s*:\s*[^{]+)?`),

	// 20. Scala: def foo(a: Int): Int = { ... }
	regexp.MustCompile(`(?m)\bdef\s+(\w+)\s*\(([^)]*)\)(?:\s*:\s*[^=]+)?\s*=`),

	// 21. Ruby: def foo(a, b) ... end
	regexp.MustCompile(`(?m)\bdef\s+(\w+)\s*\(?([^)]*)\)?`),

	// 22. Lisp / Clojure: (defn function-name [args] ...)  -> name "function-name"
	regexp.MustCompile(`\(\s*defn\s+([\w\-!]+)`),

	// 23. Markdown headings (treated as structural elements)
	regexp.MustCompile(`(?m)^[\t ]*#{1,6}\s+(.*)`),

	// 24. INI / TOML section headers: [section]  -> name "section"
	regexp.MustCompile(`(?m)^[\t ]*\[\s*(\w+)\s*\]`),
}

// extractSignaturesUniversal applies all patterns to source and returns a
// deduplicated list of signatures.
func extractSignaturesUniversal(source string) []Signature {
	seen := make(map[string]bool)
	var sigs []Signature

	for _, re := range universalPatterns {
		matches := re.FindAllStringSubmatch(source, -1)
		for _, m := range matches {
			name := ""
			var full string

			// Determine capture groups (depends on pattern)
			if len(m) >= 3 {
				// Patterns with two explicit groups: (name) and (params) or (keyword, name)
				// For class/struct/etc pattern, m[1] is the keyword, m[2] is the name.
				// For others, m[1] is the name, m[2] is the params list.
				if m[1] != "" && (m[2] == "" || strings.TrimSpace(m[2]) == "") {
					// likely a pattern like 'class Name' where m[1] is name
					name = m[1]
				} else if m[2] != "" && (m[1] == "class" || m[1] == "struct" || m[1] == "interface" ||
					m[1] == "trait" || m[1] == "enum" || m[1] == "object" ||
					m[1] == "record" || m[1] == "module" || m[1] == "impl") {
					name = m[2]
				} else {
					name = m[1]
				}
				full = m[0]
			} else if len(m) == 2 {
				name = m[1]
				full = m[0]
			} else {
				continue
			}

			if name == "" || name == "if" || name == "for" || name == "while" || name == "switch" {
				continue
			}
			if seen[name] {
				continue
			}
			seen[name] = true

			sig := full
			// For very long lines, truncate at first '{' or 'end' to keep signature clean
			if idx := strings.IndexAny(sig, "{;"); idx != -1 {
				sig = strings.TrimSpace(sig[:idx])
			}
			// Remove trailing colon for Python-like
			sig = strings.TrimSuffix(sig, ":")

			sigs = append(sigs, Signature{
				Name:      name,
				Signature: sig,
			})
		}
	}
	return sigs
}
