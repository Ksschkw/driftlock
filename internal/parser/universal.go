package parser

import (
	"regexp"
	"strings"
)

// extractSignatures is the core extraction routine. It selects a language spec
// from the file path, sanitizes comments/strings, honors driftlock:ignore
// annotations, and applies only that language's structural patterns. Unknown
// extensions fall back to a conservative universal code spec.
//
// Names are deduplicated within a file so each structural element appears once,
// regardless of how many patterns happen to match it.
func extractSignatures(filePath, source string) []Signature {
	spec := specForFile(filePath)
	sanitized := sanitize(source, spec)
	ignored := ignoredLines(source, spec)

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
			// Skip declarations on ignored lines.
			startLine := lineOf(sanitized, loc[0])
			if ignored[startLine] {
				continue
			}
			if seen[name] {
				continue
			}
			seen[name] = true
			// The signature text comes from the ORIGINAL source (matching runs
			// on the sanitized copy, whose string literals are blanked — using
			// it would erase default values like `punctuation: str = "!"`).
			// The brace/semicolon cut point is computed on the sanitized slice
			// so a '{' or ';' inside a string can never truncate the signature.
			sanSlice := sanitized[loc[0]:loc[1]]
			origSlice := source[loc[0]:loc[1]]
			if cut := strings.IndexAny(sanSlice, "{;"); cut != -1 {
				origSlice = origSlice[:cut]
			}
			sigs = append(sigs, Signature{Name: name, Signature: tidySignature(origSlice)})
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

// isIgnoredKeyword filters control-flow and statement keywords that permissive
// patterns may accidentally capture as function names. `function` is included
// because a bare-method pattern can otherwise read an anonymous
// `function() {` expression as a symbol named "function".
func isIgnoredKeyword(s string) bool {
	switch s {
	case "if", "for", "while", "switch", "return", "else", "catch",
		"do", "match", "when", "with", "case", "select", "defer", "go",
		"function", "typeof", "new", "delete", "void", "await", "yield",
		"throw", "try", "finally", "super", "this":
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
