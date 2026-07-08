package parser

import (
	"path/filepath"
	"regexp"
	"strings"
)

// langSpec describes how to sanitize and pattern-match a single language (or
// family of languages). Comment and string spans are blanked out before the
// structural patterns run, which eliminates the vast majority of false
// positives (signatures inside comments, colons inside string literals, etc.).
type langSpec struct {
	name         string
	lineComments []string    // e.g. "//", "#", "--"
	blockComment [][2]string // e.g. {"/*", "*/"}
	stringDelims []string    // e.g. "\"", "'", "`", "\"\"\""
	patterns     []*regexp.Regexp
	// dataLike marks structured-data languages (YAML/JSON/TOML/XML/Markdown)
	// whose "structure" lives in keys/tags/headings rather than code
	// signatures. String/comment stripping is skipped for these because the
	// concept does not apply cleanly.
	dataLike bool
}

// ── Shared pattern groups ───────────────────────────────────────────────────
// Patterns are scoped per language so that, for example, the YAML "key:"
// pattern never runs against a Go file. Each pattern's first (or keyword+name)
// capture group yields the symbol name; see extractNameAndFull.

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

	// Go: supports multi-line params via [\s\S] up to the closing paren.
	pGoFunc = regexp.MustCompile(`(?m)^[\t ]*func\s+(?:\(\s*\w+\s+[\*]?\w+\s*\)\s+)?(\w+)\s*\(([\s\S]*?)\)\s*(?:\([\s\S]*?\)|[\w\[\]\.\*&<> ,]+)?\s*\{?`)

	pGoType = regexp.MustCompile(`(?m)^[\t ]*type\s+(\w+)\s+(struct|interface|func|map|\[|chan|\w)`)

	pPyDef   = regexp.MustCompile(`(?m)^[\t ]*(?:async\s+)?def\s+(\w+)\s*\(([\s\S]*?)\)`)
	pPyClass = regexp.MustCompile(`(?m)^[\t ]*class\s+(\w+)`)

	pRustFn   = regexp.MustCompile(`(?m)^[\t ]*(?:pub(?:\([^)]*\))?\s+)?(?:async\s+)?(?:unsafe\s+)?(?:const\s+)?fn\s+(\w+)\s*(?:<[^>]*>)?\s*\(([\s\S]*?)\)`)
	pRustType = regexp.MustCompile(`(?m)^[\t ]*(?:pub(?:\([^)]*\))?\s+)?(struct|enum|trait|union|impl|type)\s+(\w+)`)

	pArrowFn  = regexp.MustCompile(`(?m)^[\t ]*(?:export\s+)?(?:const|let|var)\s+(\w+)\s*(?::\s*[^=]+)?=\s*(?:async\s+)?\(?([^)=]*)\)?\s*=>`)
	pFuncKw   = regexp.MustCompile(`(?m)^[\t ]*(?:export\s+)?(?:default\s+)?(?:async\s+)?function\s*\*?\s+(\w+)\s*\(([\s\S]*?)\)`)
	pTsType   = regexp.MustCompile(`(?m)^[\t ]*(?:export\s+)?(?:declare\s+)?(?:type|interface|enum)\s+(\w+)`)
	pTsMethod = regexp.MustCompile(`(?m)^[\t ]*(?:public|private|protected|readonly|static|async|get|set)\s+(\w+)\s*\(([\s\S]*?)\)\s*(?::\s*[\w\[\]<>., |]+)?\s*\{`)

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
		patterns:     []*regexp.Regexp{pGoFunc, pGoType},
	},
	"python": {
		name: "python", lineComments: []string{"#"},
		stringDelims: []string{`"""`, "'''", "\"", "'"},
		patterns:     []*regexp.Regexp{pPyDef, pPyClass},
	},
	"javascript": {
		name: "javascript", lineComments: []string{"//"}, blockComment: [][2]string{{"/*", "*/"}},
		stringDelims: []string{"`", "\"", "'"},
		patterns:     []*regexp.Regexp{pFuncKw, pArrowFn, pTsType, pTsMethod, pClassGroup},
	},
	"typescript": {
		name: "typescript", lineComments: []string{"//"}, blockComment: [][2]string{{"/*", "*/"}},
		stringDelims: []string{"`", "\"", "'"},
		patterns:     []*regexp.Regexp{pFuncKw, pArrowFn, pTsType, pTsMethod, pClassGroup},
	},
	"java": {
		name: "java", lineComments: []string{"//"}, blockComment: [][2]string{{"/*", "*/"}},
		stringDelims: []string{"\"", "'"},
		patterns:     []*regexp.Regexp{pJavaMethod, pClassGroup},
	},
	"csharp": {
		name: "csharp", lineComments: []string{"//"}, blockComment: [][2]string{{"/*", "*/"}},
		stringDelims: []string{"\"", "'"},
		patterns:     []*regexp.Regexp{pJavaMethod, pClassGroup},
	},
	"c": {
		name: "c", lineComments: []string{"//"}, blockComment: [][2]string{{"/*", "*/"}},
		stringDelims: []string{"\"", "'"},
		patterns:     []*regexp.Regexp{pCppTemplate, pCFunc, pClassGroup, pDefine},
	},
	"rust": {
		name: "rust", lineComments: []string{"//"}, blockComment: [][2]string{{"/*", "*/"}},
		stringDelims: []string{"\"", "'"},
		patterns:     []*regexp.Regexp{pRustFn, pRustType},
	},
	"swift": {
		name: "swift", lineComments: []string{"//"}, blockComment: [][2]string{{"/*", "*/"}},
		stringDelims: []string{"\"", "'"},
		patterns:     []*regexp.Regexp{pSwiftFunc, pClassGroup},
	},
	"kotlin": {
		name: "kotlin", lineComments: []string{"//"}, blockComment: [][2]string{{"/*", "*/"}},
		stringDelims: []string{"\"", "'"},
		patterns:     []*regexp.Regexp{pKotlinFun, pClassGroup},
	},
	"scala": {
		name: "scala", lineComments: []string{"//"}, blockComment: [][2]string{{"/*", "*/"}},
		stringDelims: []string{"\"", "'"},
		patterns:     []*regexp.Regexp{pScalaDef, pClassGroup},
	},
	"php": {
		name: "php", lineComments: []string{"//", "#"}, blockComment: [][2]string{{"/*", "*/"}},
		stringDelims: []string{"\"", "'"},
		patterns:     []*regexp.Regexp{pPhpFunc, pClassGroup},
	},
	"ruby": {
		name: "ruby", lineComments: []string{"#"},
		stringDelims: []string{"\"", "'"},
		patterns:     []*regexp.Regexp{pRubyDef, pClassGroup},
	},
	"shell": {
		name: "shell", lineComments: []string{"#"},
		stringDelims: []string{"\"", "'"},
		patterns:     []*regexp.Regexp{pShellFunc},
	},
	"lua": {
		name: "lua", lineComments: []string{"--"}, blockComment: [][2]string{{"--[[", "]]"}},
		stringDelims: []string{"\"", "'"},
		patterns:     []*regexp.Regexp{pLuaFunc},
	},
	"clojure": {
		name: "clojure", lineComments: []string{";"},
		stringDelims: []string{"\""},
		patterns:     []*regexp.Regexp{pDefn},
	},
	"sql": {
		name: "sql", lineComments: []string{"--"}, blockComment: [][2]string{{"/*", "*/"}},
		stringDelims: []string{"'"},
		patterns:     []*regexp.Regexp{pSQL},
	},
	"yaml":     {name: "yaml", dataLike: true, patterns: []*regexp.Regexp{pYAMLKey}},
	"json":     {name: "json", dataLike: true, patterns: []*regexp.Regexp{pJSONKey}},
	"toml":     {name: "toml", dataLike: true, patterns: []*regexp.Regexp{pINI, pYAMLKey}},
	"ini":      {name: "ini", dataLike: true, patterns: []*regexp.Regexp{pINI, pYAMLKey}},
	"xml":      {name: "xml", dataLike: true, patterns: []*regexp.Regexp{pXMLTag}},
	"markdown": {name: "markdown", dataLike: true, patterns: []*regexp.Regexp{pMarkdown}},
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
	patterns: []*regexp.Regexp{
		pGoFunc, pPyDef, pRustFn, pFuncKw, pArrowFn, pCFunc, pCppTemplate,
		pSwiftFunc, pKotlinFun, pScalaDef, pPhpFunc, pShellFunc, pLuaFunc,
		pDefn, pClassGroup, pGoType, pDefine,
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
