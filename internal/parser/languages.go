package parser

import (
	"path/filepath"
	"regexp"
	"strings"
)

// pattern pairs a compiled structural regex with the 1-based capture group
// that holds the symbol name. Declaring the group explicitly replaces the old
// heuristic ("group 1 is the name unless group 1 looks like a type keyword"),
// which misread the SQL keyword TABLE as a table name, extracted `x` from
// `def type(x)`, and turned `fn type(x: i32)` into a symbol named `x: i32`.
type pattern struct {
	re        *regexp.Regexp
	nameGroup int
	// scopeGroup, when non-zero, is the capture group holding the enclosing
	// scope of the declaration (a Go receiver type or a Rust impl target).
	// It is reserved for qualified naming; 0 means "no scope".
	scopeGroup int
}

// pat builds a pattern whose symbol name lives in the given capture group.
func pat(re *regexp.Regexp, nameGroup int) pattern {
	return pattern{re: re, nameGroup: nameGroup}
}

// langSpec describes how to sanitize and pattern-match a single language (or
// family of languages). Comment and string spans are blanked out before the
// structural patterns run, which eliminates the vast majority of false
// positives (signatures inside comments, colons inside string literals, etc.).
type langSpec struct {
	name         string
	lineComments []string    // e.g. "//", "#", "--"
	blockComment [][2]string // e.g. {"/*", "*/"}
	stringDelims []string    // e.g. "\"", "'", "`", "\"\"\""
	patterns     []pattern
	// dataLike marks structured-data languages (YAML/JSON/TOML/XML/Markdown)
	// whose "structure" lives in keys/tags/headings rather than code
	// signatures. String/comment stripping is skipped for these because the
	// concept does not apply cleanly.
	dataLike bool
}

// ── Shared pattern groups ───────────────────────────────────────────────────
// Patterns are scoped per language so that, for example, the YAML "key:"
// pattern never runs against a Go file. Each pattern declares which capture
// group holds the symbol name (see the pattern type and pat).

// tsParamList matches a parenthesised parameter list for JS/TS-style
// declarations. It forbids a top-level ';' so a declaration pattern can never
// leap across statements to find a later '{' (which made `doSomething(event);`
// bind to a following anonymous-function body). One level of nested
// parentheses is permitted, covering default values such as `cb = () => {}`.
const tsParamList = `\((?:[^;()]|\([^;()]*\))*\)`

var (
	pCFunc = regexp.MustCompile(
		`(?m)^[\t ]*(?:(?:static|inline|virtual|explicit|export|constexpr|noexcept|\[\[[^]]+\]\])\s+)*` +
			`(?:(?:unsigned|signed|short|long|long long|int|char|float|double|void|bool|wchar_t|size_t|ptrdiff_t|int\d+_t|uint\d+_t|auto)\s+)+` +
			`(?:\*\s*|&\s*)*` +
			`(\w+)\s*\(([^{;]*)\)\s*(?:(?:const|override|final)\s*)*`)

	pCppTemplate = regexp.MustCompile(`(?m)\btemplate\s*<[^>]+>\s*` +
		`(?:(?:static|inline|constexpr|virtual|export)\s+)*` +
		`(?:\w+(?:<[^>]+>)?\s+)+` +
		`(\w+)\s*\(([^{;]*)\)`)

	pDefine = regexp.MustCompile(`(?m)^[\t ]*#\s*define\s+(\w+)`)

	// Go: supports multi-line params, generic receivers (`func (s *Stack[T])`),
	// type-parameter lists (`func Map[T any, U any]`), and function-typed
	// parameters (`f func(T) U`). The parameter group tolerates one level of
	// nested parentheses; a plain non-greedy match stopped at the FIRST ')',
	// which truncated any signature containing a func-typed or callback
	// parameter (e.g. `func Apply(xs []int, f func(int) int) []int`).
	pGoFunc = regexp.MustCompile(
		`(?m)^[\t ]*func\s+` +
			`(?:\(\s*\w+\s+\*?\w+(?:\s*\[[^\]]*\])?\s*\)\s+)?` +
			`(\w+)\s*` +
			`(?:\[[^\]]*\])?\s*` +
			`\((?:[^(){}]|\((?:[^(){}]*)\))*\)` +
			`\s*(?:\([\s\S]*?\)|[\w\[\]\.\*&<> ,]+)?\s*\{?`)

	// Go type declarations, including generic ones (`type Stack[T any] struct`).
	pGoType = regexp.MustCompile(`(?m)^[\t ]*type\s+(\w+)(?:\s*\[[^\]]*\])?\s+(struct|interface|func|map|\[|chan|\w)`)

	pPyDef   = regexp.MustCompile(`(?m)^[\t ]*(?:async\s+)?def\s+(\w+)\s*\(([\s\S]*?)\)`)
	pPyClass = regexp.MustCompile(`(?m)^[\t ]*class\s+(\w+)`)

	pRustFn   = regexp.MustCompile(`(?m)^[\t ]*(?:pub(?:\([^)]*\))?\s+)?(?:async\s+)?(?:unsafe\s+)?(?:const\s+)?fn\s+(\w+)\s*(?:<[^>]*>)?\s*\(([\s\S]*?)\)`)
	pRustType = regexp.MustCompile(`(?m)^[\t ]*(?:pub(?:\([^)]*\))?\s+)?(struct|enum|trait|union|impl|type)\s+(\w+)`)

	pArrowFn  = regexp.MustCompile(`(?m)^[\t ]*(?:export\s+)?(?:const|let|var)\s+(\w+)\s*(?::\s*[^=]+)?=\s*(?:async\s+)?\(?([^)=]*)\)?\s*=>`)
	pFuncKw   = regexp.MustCompile(`(?m)^[\t ]*(?:export\s+)?(?:default\s+)?(?:async\s+)?function\s*\*?\s+(\w+)\s*\(([\s\S]*?)\)`)
	pTsType   = regexp.MustCompile(`(?m)^[\t ]*(?:export\s+)?(?:declare\s+)?(?:type|interface|enum)\s+(\w+)`)
	pTsMethod = regexp.MustCompile(`(?m)^[\t ]*(?:(?:public|private|protected|readonly|static|async|get|set|abstract|override)\s+)+(\w+)\s*(?:<[^>]*>)?\s*` + tsParamList + `\s*(?::\s*[^;{]+)?\s*\{`)
	// pTsBareMethod matches a class/object method with NO access modifier —
	// the dominant style in TypeScript and JavaScript (`run(x) { … }`),
	// which pTsMethod misses because it requires a modifier word. The trailing
	// `{` is what separates a declaration from a call (`foo(x);`), and
	// statement keywords are filtered by isIgnoredKeyword.
	pTsBareMethod = regexp.MustCompile(`(?m)^[\t ]*(\w+)\s*(?:<[^>]*>)?\s*` + tsParamList + `\s*(?::\s*[^;{]+)?\s*\{`)

	pJavaMethod = regexp.MustCompile(
		`(?m)^[\t ]*(?:@\w+(?:\([^)]*\))?\s*)*(?:(?:public|private|protected|internal|static|final|abstract|override|virtual|async|synchronized|native|default)\s+)+` +
			`(?:[\w.$]+(?:<[^>]+>)?(?:\[\])?\s+)` +
			`(\w+)\s*\(([\s\S]*?)\)`)

	pClassGroup = regexp.MustCompile(
		`(?m)^[\t ]*(?:(?:public|private|protected|export|abstract|sealed|final|open|data|case|internal|static)\s+)*` +
			`\b(class|struct|interface|trait|enum|object|record|module)\s+(\w+)`)

	pSwiftFunc = regexp.MustCompile(`(?m)^[\t ]*(?:(?:public|private|internal|fileprivate|open|static|final|override|mutating)\s+)*func\s+(\w+)\s*(?:<[^>]*>)?\s*\(([\s\S]*?)\)`)
	pKotlinFun = regexp.MustCompile(`(?m)^[\t ]*(?:(?:public|private|protected|internal|open|override|suspend|inline)\s+)*fun\s+(?:<[^>]*>\s*)?(\w+)\s*\(([\s\S]*?)\)`)
	pScalaDef  = regexp.MustCompile(`(?m)^[\t ]*(?:(?:private|protected|final|override|implicit)\s+)*def\s+(\w+)\s*(?:\[[^\]]*\])?\s*\(([\s\S]*?)\)`)

	pShellFunc = regexp.MustCompile(`(?m)^[\t ]*(?:function\s+)?([\w-]+)\s*\(\)\s*\{`)
	pLuaFunc   = regexp.MustCompile(`(?m)(?:^|\blocal\s+)function\s+([\w.:]+)\s*\(([^)]*)\)`)
	pDefn      = regexp.MustCompile(`\(\s*defn-?\s+([\w\-!?*]+)`)
	pPhpFunc   = regexp.MustCompile(`(?m)^[\t ]*(?:(?:public|private|protected|static|final|abstract)\s+)*function\s+(\w+)\s*\(([\s\S]*?)\)`)
	pRubyDef   = regexp.MustCompile(`(?m)^[\t ]*def\s+([\w.]+[?!=]?)`)

	// Data / markup patterns.
	pSQL      = regexp.MustCompile(`(?i)\bCREATE\s+(?:OR\s+REPLACE\s+)?(TABLE|VIEW|PROCEDURE|FUNCTION|TRIGGER|INDEX|MATERIALIZED\s+VIEW)\s+(?:IF\s+NOT\s+EXISTS\s+)?[` + "`" + `"']?(\w+)`)
	pYAMLKey  = regexp.MustCompile(`(?m)^[\t ]*"?([\w.-]+)"?\s*:(?:\s|$)`)
	pJSONKey  = regexp.MustCompile(`(?m)^[\t ]*"([\w.-]+)"\s*:`)
	pXMLTag   = regexp.MustCompile(`<\s*([\w:-]+)(?:\s[^>]*)?/?>`)
	pMarkdown = regexp.MustCompile(`(?m)^[\t ]*(#{1,6})\s+(.*)`)
	pINI      = regexp.MustCompile(`(?m)^[\t ]*\[\s*([\w.-]+)\s*\]`)
)

// registry maps a canonical language name to its spec.
var registry = map[string]langSpec{
	"go": {
		name: "go", lineComments: []string{"//"}, blockComment: [][2]string{{"/*", "*/"}},
		stringDelims: []string{"`", "\""},
		patterns:     []pattern{pat(pGoFunc, 1), pat(pGoType, 1)},
	},
	"python": {
		name: "python", lineComments: []string{"#"},
		stringDelims: []string{`"""`, "'''", "\"", "'"},
		patterns:     []pattern{pat(pPyDef, 1), pat(pPyClass, 1)},
	},
	"javascript": {
		name: "javascript", lineComments: []string{"//"}, blockComment: [][2]string{{"/*", "*/"}},
		stringDelims: []string{"`", "\"", "'"},
		patterns:     []pattern{pat(pFuncKw, 1), pat(pArrowFn, 1), pat(pTsType, 1), pat(pTsMethod, 1), pat(pTsBareMethod, 1), pat(pClassGroup, 2)},
	},
	"typescript": {
		name: "typescript", lineComments: []string{"//"}, blockComment: [][2]string{{"/*", "*/"}},
		stringDelims: []string{"`", "\"", "'"},
		patterns:     []pattern{pat(pFuncKw, 1), pat(pArrowFn, 1), pat(pTsType, 1), pat(pTsMethod, 1), pat(pTsBareMethod, 1), pat(pClassGroup, 2)},
	},
	"java": {
		name: "java", lineComments: []string{"//"}, blockComment: [][2]string{{"/*", "*/"}},
		stringDelims: []string{"\"", "'"},
		patterns:     []pattern{pat(pJavaMethod, 1), pat(pClassGroup, 2)},
	},
	"csharp": {
		name: "csharp", lineComments: []string{"//"}, blockComment: [][2]string{{"/*", "*/"}},
		stringDelims: []string{"\"", "'"},
		patterns:     []pattern{pat(pJavaMethod, 1), pat(pClassGroup, 2)},
	},
	"c": {
		name: "c", lineComments: []string{"//"}, blockComment: [][2]string{{"/*", "*/"}},
		stringDelims: []string{"\"", "'"},
		patterns:     []pattern{pat(pCppTemplate, 1), pat(pCFunc, 1), pat(pClassGroup, 2), pat(pDefine, 1)},
	},
	"rust": {
		name: "rust", lineComments: []string{"//"}, blockComment: [][2]string{{"/*", "*/"}},
		stringDelims: []string{"\"", "'"},
		patterns:     []pattern{pat(pRustFn, 1), pat(pRustType, 2)},
	},
	"swift": {
		name: "swift", lineComments: []string{"//"}, blockComment: [][2]string{{"/*", "*/"}},
		stringDelims: []string{"\"", "'"},
		patterns:     []pattern{pat(pSwiftFunc, 1), pat(pClassGroup, 2)},
	},
	"kotlin": {
		name: "kotlin", lineComments: []string{"//"}, blockComment: [][2]string{{"/*", "*/"}},
		stringDelims: []string{"\"", "'"},
		patterns:     []pattern{pat(pKotlinFun, 1), pat(pClassGroup, 2)},
	},
	"scala": {
		name: "scala", lineComments: []string{"//"}, blockComment: [][2]string{{"/*", "*/"}},
		stringDelims: []string{"\"", "'"},
		patterns:     []pattern{pat(pScalaDef, 1), pat(pClassGroup, 2)},
	},
	"php": {
		name: "php", lineComments: []string{"//", "#"}, blockComment: [][2]string{{"/*", "*/"}},
		stringDelims: []string{"\"", "'"},
		patterns:     []pattern{pat(pPhpFunc, 1), pat(pClassGroup, 2)},
	},
	"ruby": {
		name: "ruby", lineComments: []string{"#"},
		stringDelims: []string{"\"", "'"},
		patterns:     []pattern{pat(pRubyDef, 1), pat(pClassGroup, 2)},
	},
	"shell": {
		name: "shell", lineComments: []string{"#"},
		stringDelims: []string{"\"", "'"},
		patterns:     []pattern{pat(pShellFunc, 1)},
	},
	"lua": {
		name: "lua", lineComments: []string{"--"}, blockComment: [][2]string{{"--[[", "]]"}},
		stringDelims: []string{"\"", "'"},
		patterns:     []pattern{pat(pLuaFunc, 1)},
	},
	"clojure": {
		name: "clojure", lineComments: []string{";"},
		stringDelims: []string{"\""},
		patterns:     []pattern{pat(pDefn, 1)},
	},
	"sql": {
		name: "sql", lineComments: []string{"--"}, blockComment: [][2]string{{"/*", "*/"}},
		stringDelims: []string{"'"},
		patterns:     []pattern{pat(pSQL, 2)},
	},
	"yaml":     {name: "yaml", dataLike: true, patterns: []pattern{pat(pYAMLKey, 1)}},
	"json":     {name: "json", dataLike: true, patterns: []pattern{pat(pJSONKey, 1)}},
	"toml":     {name: "toml", dataLike: true, patterns: []pattern{pat(pINI, 1), pat(pYAMLKey, 1)}},
	"ini":      {name: "ini", dataLike: true, patterns: []pattern{pat(pINI, 1), pat(pYAMLKey, 1)}},
	"xml":      {name: "xml", dataLike: true, patterns: []pattern{pat(pXMLTag, 1)}},
	"markdown": {name: "markdown", dataLike: true, patterns: []pattern{pat(pMarkdown, 2)}},
}

// extByLang maps file extensions (without dot, lowercase) to a registry key.
var extByLang = map[string]string{
	"go":       "go",
	"py":       "python",
	"pyi":      "python",
	"js":       "javascript",
	"jsx":      "javascript",
	"mjs":      "javascript",
	"cjs":      "javascript",
	"ts":       "typescript",
	"tsx":      "typescript",
	"java":     "java",
	"cs":       "csharp",
	"c":        "c",
	"h":        "c",
	"cc":       "c",
	"cpp":      "c",
	"cxx":      "c",
	"hpp":      "c",
	"hxx":      "c",
	"rs":       "rust",
	"swift":    "swift",
	"kt":       "kotlin",
	"kts":      "kotlin",
	"scala":    "scala",
	"php":      "php",
	"rb":       "ruby",
	"sh":       "shell",
	"bash":     "shell",
	"zsh":      "shell",
	"lua":      "lua",
	"clj":      "clojure",
	"cljs":     "clojure",
	"sql":      "sql",
	"yaml":     "yaml",
	"yml":      "yaml",
	"json":     "json",
	"toml":     "toml",
	"ini":      "ini",
	"cfg":      "ini",
	"xml":      "xml",
	"html":     "xml",
	"htm":      "xml",
	"svg":      "xml",
	"md":       "markdown",
	"markdown": "markdown",
}

// universalSpec is used for files whose extension is not recognized. It runs
// the conservative set of code patterns (never the noisy data-language
// patterns) and strips the most common comment/string styles.
var universalSpec = langSpec{
	name:         "universal",
	lineComments: []string{"//", "#", "--", ";"},
	blockComment: [][2]string{{"/*", "*/"}},
	stringDelims: []string{"`", "\"", "'"},
	patterns: []pattern{
		pat(pGoFunc, 1), pat(pPyDef, 1), pat(pRustFn, 1), pat(pFuncKw, 1), pat(pArrowFn, 1), pat(pCFunc, 1), pat(pCppTemplate, 1),
		pat(pSwiftFunc, 1), pat(pKotlinFun, 1), pat(pScalaDef, 1), pat(pPhpFunc, 1), pat(pShellFunc, 1), pat(pLuaFunc, 1),
		pat(pDefn, 1), pat(pClassGroup, 2), pat(pGoType, 1), pat(pDefine, 1),
	},
}

// specForFile returns the language spec for the given file path, falling back
// to the universal spec for unknown extensions.
func specForFile(filePath string) langSpec {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filePath), "."))
	if ext == "" {
		// Handle extensionless files by base name (Dockerfile, Makefile…).
		base := strings.ToLower(filepath.Base(filePath))
		switch base {
		case "makefile", "dockerfile":
			return universalSpec
		}
		return universalSpec
	}
	if key, ok := extByLang[ext]; ok {
		return registry[key]
	}
	return universalSpec
}
