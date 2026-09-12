package parser

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// extractSignatures is the core extraction routine. It selects a language spec
// from the file path, sanitizes comments/strings, honors driftlock:ignore
// annotations, and applies only that language's structural patterns. Unknown
// extensions fall back to a conservative universal code spec.
//
// Duplicates are removed by (name, signature), not by name alone. Keying on the
// bare name discarded every same-named declaration after the first, so a file
// with `func (a *A) Close()` and `func (b *B) Close()` reported only one of
// them and a change to the other was invisible.
func extractSignatures(filePath, source string) []Signature {
	spec := specForFile(filePath)
	sanitized := sanitize(source, spec)
	ignored := ignoredLines(source, spec)

	// Scope lookup is computed once per file. Brace counting runs on the
	// SANITIZED source so a brace inside a string or comment cannot unbalance
	// the depth.
	var scopes map[int]string
	if spec.scopePattern != nil {
		scopes = computeScopes(sanitized, spec.scopePattern)
	}

	seen := make(map[string]bool)
	var sigs []Signature

	for _, p := range spec.patterns {
		locs := p.re.FindAllStringSubmatchIndex(sanitized, -1)
		for _, loc := range locs {
			match := submatchStrings(sanitized, loc)
			name := groupAt(match, p.nameGroup)
			if name == "" || isIgnoredKeyword(name) {
				continue
			}
			// A permissive modifier-less pattern (Java/C#, JS/TS) can read a
			// statement as a declaration — `return foo(a);` looks like a method
			// named foo with a return type of "return". A span that begins with
			// a statement keyword is never a declaration.
			if isStatementStart(match[0]) {
				continue
			}
			// Skip declarations on ignored lines.
			startLine := lineOf(sanitized, loc[0])
			if ignored[startLine] {
				continue
			}
			// The signature text comes from the ORIGINAL source (matching runs
			// on the sanitized copy, whose string literals are blanked — using
			// it would erase default values like `punctuation: str = "!"`).
			// The brace/semicolon cut point is computed on the sanitized slice
			// so a '{' or ';' inside a string can never truncate the signature.
			sanSlice := sanitized[loc[0]:loc[1]]
			origSlice := source[loc[0]:loc[1]]
			// Comments and strings are blanked to spaces in the sanitized copy,
			// so a trailing comment shows up as trailing spaces. Trim to the
			// last meaningful sanitized character so the original's comment is
			// not carried into the signature.
			if n := len(strings.TrimRight(sanSlice, " \t")); n < len(sanSlice) {
				sanSlice = sanSlice[:n]
				origSlice = origSlice[:n]
			}
			// C-like declarations end at the body brace or statement
			// terminator. Indentation-delimited languages (Python, Ruby) do
			// not, and a '{' or ';' there is legitimate signature content:
			// `def f(x: dict = {})` was cut to `def f(x: dict = `.
			if p.goTypeExpr {
				if cut := cutGoTypeBody(sanSlice); cut != -1 {
					origSlice = origSlice[:cut]
				}
			} else if !spec.indentDelimited {
				if cut := strings.IndexAny(sanSlice, "{;"); cut != -1 {
					origSlice = origSlice[:cut]
				}
			}
			sig := tidySignature(origSlice)
			if !isPublicSymbol(spec.visibility, name, sig) {
				continue
			}
			if scope, ok := scopes[startLine]; ok && scope != "" {
				name = scope + "." + name
			}
			key := name + "\x00" + sig
			if seen[key] {
				continue
			}
			seen[key] = true
			sigs = append(sigs, Signature{Name: name, Signature: sig})
		}
	}
	return sigs
}

// submatchStrings converts a FindAllStringSubmatchIndex location slice into the
// equivalent []string that FindStringSubmatch would return (empty string for
// groups that did not participate in the match).
func submatchStrings(src string, loc []int) []string {
	out := make([]string, len(loc)/2)
	for i := 0; i < len(loc); i += 2 {
		if loc[i] < 0 {
			out[i/2] = ""
			continue
		}
		out[i/2] = src[loc[i]:loc[i+1]]
	}
	return out
}

// isStatementStart reports whether a matched span begins with a statement
// keyword rather than a declaration. Without it, a modifier-less pattern can
// read `return foo(a);` as a method declaration whose return type is "return"
// and whose name is foo.
func isStatementStart(span string) bool {
	fields := strings.Fields(span)
	if len(fields) == 0 {
		return false
	}
	switch strings.ToLower(fields[0]) {
	case "return", "throw", "new", "yield", "await", "assert", "raise",
		"delete", "typeof", "print", "del", "else", "goto", "break", "continue":
		return true
	}
	return false
}

// goTypeBodyRE matches a Go type declaration whose type expression is a struct
// or interface, i.e. one that opens a body rather than naming an element type.
var goTypeBodyRE = regexp.MustCompile(`^\s*type\s+\w+(?:\s*\[[^\]]*\])?\s+(?:=\s*)?(?:struct|interface)\b`)

// cutGoTypeBody decides where a Go type declaration's signature ends.
//
// A struct or interface body is dropped so the result matches the multi-line
// form (`type Foo struct {` -> `type Foo struct`). A balanced brace group that
// is part of the type — `type Set[T comparable] map[T]struct{}` — is kept,
// because the element type is API surface and a change to it must be detected.
func cutGoTypeBody(s string) int {
	if goTypeBodyRE.MatchString(s) {
		return strings.IndexByte(s, '{')
	}
	return cutUnbalancedBrace(s)
}

// cutUnbalancedBrace returns the index of the first '{' in s that is not
// closed later in s, or -1 when every brace is balanced. It lets Go type
// declarations keep `map[K]V{}` (balanced, part of the type) while dropping
// `struct {` (unbalanced, the start of a body).
func cutUnbalancedBrace(s string) int {
	i := 0
	for i < len(s) {
		if s[i] != '{' {
			i++
			continue
		}
		depth := 0
		j := i
		for ; j < len(s); j++ {
			switch s[j] {
			case '{':
				depth++
			case '}':
				depth--
			}
			if depth == 0 {
				break
			}
		}
		if depth != 0 {
			return i
		}
		i = j + 1
	}
	return -1
}

// isPublicSymbol applies a language's visibility rule so private declarations
// never reach the structural diff. An empty rule treats everything as public.
func isPublicSymbol(rule, name, sig string) bool {
	switch rule {
	case "go":
		r, _ := utf8.DecodeRuneInString(name)
		return unicode.IsUpper(r)
	case "underscore":
		return !strings.HasPrefix(name, "_")
	case "rustpub":
		return strings.Contains(sig, "pub ")
	default:
		return true
	}
}

// computeScopes maps each 1-based line number inside a scope-opening block to
// the scope name captured from that block's declaration. It is used for Rust
// `impl` blocks so `A::build` and `B::build` are distinct symbols rather than
// two identical `build` signatures that collapse into one.
//
// Depth is counted on sanitized source (comments and strings blanked). The
// declaration line itself is excluded so the impl declaration is not qualified
// with its own name.
func computeScopes(sanitized string, re *regexp.Regexp) map[int]string {
	out := make(map[int]string)
	for _, loc := range re.FindAllStringSubmatchIndex(sanitized, -1) {
		if loc[2] < 0 || loc[3] <= loc[2] {
			continue
		}
		name := sanitized[loc[2]:loc[3]]
		open := strings.IndexByte(sanitized[loc[1]:], '{')
		if open < 0 {
			continue
		}
		open += loc[1]

		depth := 0
		end := len(sanitized)
	scan:
		for i := open; i < len(sanitized); i++ {
			switch sanitized[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					end = i
					break scan
				}
			}
		}
		for line := lineOf(sanitized, open) + 1; line <= lineOf(sanitized, end); line++ {
			out[line] = name
		}
	}
	return out
}

// groupAt returns capture group n (1-based) from a FindStringSubmatch-style
// slice, or "" when the group did not participate or does not exist. Patterns
// declare their name group explicitly (see the pattern type and pat), so name
// extraction never has to guess.
func groupAt(match []string, n int) string {
	if n <= 0 || n >= len(match) {
		return ""
	}
	return match[n]
}

// isIgnoredKeyword filters control-flow keywords that permissive patterns may
// accidentally capture as function names. `function` is included because a
// bare-method pattern can otherwise read an anonymous `function() {` expression
// as a symbol named "function".
//
// Only keywords that can never be a declaration name are listed. Statement
// words such as `new` and `delete` are deliberately absent: `fn new()` (the
// standard Rust constructor) and `delete() {}` (a legal JS method) are real
// symbols. Statement lines are rejected by isStatementStart instead, which
// inspects the start of the matched span rather than the captured name.
func isIgnoredKeyword(s string) bool {
	switch s {
	case "if", "for", "while", "switch", "return", "else", "catch",
		"do", "match", "when", "with", "case", "select", "defer", "go",
		"function":
		return true
	}
	return false
}

var wsCollapse = regexp.MustCompile(`[\t ]*\n[\t ]*`)

// tidySignature compacts a signature slice whose brace/semicolon truncation has
// already been decided by the caller (on sanitized text): it collapses internal
// newlines from multi-line parameter lists and trims decoration.
func tidySignature(raw string) string {
	raw = wsCollapse.ReplaceAllString(strings.TrimSpace(raw), " ")
	return strings.TrimSuffix(strings.TrimSpace(raw), ":")
}

// cleanSignature trims a raw match down to a compact one-line signature: it
// stops at the opening brace / statement terminator and collapses any internal
// newlines (from multi-line parameter lists) into single spaces.
func cleanSignature(raw string) string {
	raw = strings.TrimSpace(raw)
	if idx := strings.IndexAny(raw, "{;"); idx != -1 {
		raw = strings.TrimSpace(raw[:idx])
	}
	return tidySignature(raw)
}
