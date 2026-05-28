package parser

import (
	"path/filepath"
	"regexp"
	"strings"
)

// pythonParser
type pythonParser struct{}

func (p pythonParser) ExtractSignatures(source string) []Signature {
	// Matches 'def func_name(params):' optionally with return type '-> type:'
	re := regexp.MustCompile(`(?m)^\s*def\s+(\w+)\s*\(([^)]*)\)\s*(->\s*\S+)?\s*:`)
	matches := re.FindAllStringSubmatch(source, -1)
	var sigs []Signature
	for _, m := range matches {
		name := m[1]
		params := strings.TrimSpace(m[2])
		ret := strings.TrimSpace(m[3])
		sig := "def " + name + "(" + params + ")"
		if ret != "" {
			sig += " " + ret
		}
		sigs = append(sigs, Signature{Name: name, Signature: sig})
	}
	return sigs
}

// jsParser for JavaScript/TypeScript
type jsParser struct{}

func (p jsParser) ExtractSignatures(source string) []Signature {
	// Matches function declarations: function name(params) { ... }
	reFunc := regexp.MustCompile(`(?m)(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*\(([^)]*)\)(?:\s*:\s*\w+)?\s*{`)
	// Matches arrow functions assigned to const/let/var: const name = (params) => { ... }
	reArrow := regexp.MustCompile(`(?m)(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?\(?([^)]*)\)?\s*=>`)
	var sigs []Signature
	for _, m := range reFunc.FindAllStringSubmatch(source, -1) {
		name := m[1]
		params := strings.TrimSpace(m[2])
		sigs = append(sigs, Signature{Name: name, Signature: "function " + name + "(" + params + ")"})
	}
	for _, m := range reArrow.FindAllStringSubmatch(source, -1) {
		name := m[1]
		params := strings.TrimSpace(m[2])
		sigs = append(sigs, Signature{Name: name, Signature: name + " = (" + params + ") => { ... }"})
	}
	return sigs
}

// goParser
type goParser struct{}

func (p goParser) ExtractSignatures(source string) []Signature {
	// Matches 'func name(params) (return types) {'
	re := regexp.MustCompile(`(?m)^\s*func\s+(\(\w+\s+\*?\w+\)\s+)?(\w+)\s*\(([^)]*)\)\s*([\w\[\]*,\.\s]*)`)
	matches := re.FindAllStringSubmatch(source, -1)
	var sigs []Signature
	for _, m := range matches {
		receiver := strings.TrimSpace(m[1])
		name := m[2]
		params := strings.TrimSpace(m[3])
		returns := strings.TrimSpace(m[4])
		sig := "func "
		if receiver != "" {
			sig += receiver + " "
		}
		sig += name + "(" + params + ")"
		if returns != "" {
			sig += " " + returns
		}
		sigs = append(sigs, Signature{Name: name, Signature: sig})
	}
	return sigs
}

// rustParser
type rustParser struct{}

func (p rustParser) ExtractSignatures(source string) []Signature {
	// Matches 'fn name(params) -> ReturnType {'
	re := regexp.MustCompile(`(?m)^\s*(?:pub\s+)?fn\s+(\w+)\s*\(([^)]*)\)(?:\s*->\s*([^{]+))?\s*{`)
	matches := re.FindAllStringSubmatch(source, -1)
	var sigs []Signature
	for _, m := range matches {
		name := m[1]
		params := strings.TrimSpace(m[2])
		ret := strings.TrimSpace(m[3])
		sig := "fn " + name + "(" + params + ")"
		if ret != "" {
			sig += " -> " + ret
		}
		sigs = append(sigs, Signature{Name: name, Signature: sig})
	}
	return sigs
}

// languageMap maps file extensions to parsers.
var languageMap = map[string]LanguageParser{
	".py":    pythonParser{},
	".js":    jsParser{},
	".ts":    jsParser{},
	".jsx":   jsParser{},
	".tsx":   jsParser{},
	".go":    goParser{},
	".rs":    rustParser{},
}

// ExtractSignatures detects the language from the file extension and returns signatures.
func ExtractSignatures(filePath string, source string) []Signature {
	ext := filepath.Ext(filePath)
	parser, ok := languageMap[ext]
	if !ok {
		return nil
	}
	return parser.ExtractSignatures(source)
}