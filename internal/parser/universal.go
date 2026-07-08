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

	for _, re := range spec.patterns {
		locs := re.FindAllStringSubmatchIndex(sanitized, -1)
		for _, loc := range locs {
			match := submatchStrings(sanitized, loc)
			name, full := extractNameAndFull(match)
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
			sigs = append(sigs, Signature{Name: name, Signature: cleanSignature(full)})
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

// extractNameAndFull deduces the symbol name from regex capture groups.
//
// Patterns that capture a keyword (class/struct/…) plus an identifier put the
// keyword in group 1 and the identifier in group 2. All other patterns put the
// name in group 1.
func extractNameAndFull(match []string) (string, string) {
	if len(match) == 0 {
		return "", ""
	}
	full := match[0]

	if len(match) >= 3 && match[2] != "" && isTypeKeyword(match[1]) {
		return match[2], full
	}
	if len(match) >= 2 && match[1] != "" {
		return match[1], full
	}
	return "", full
}

func isTypeKeyword(s string) bool {
	switch s {
	case "class", "struct", "interface", "trait", "enum",
		"object", "record", "module", "impl", "union", "type":
		return true
	}
	return false
}

// isIgnoredKeyword filters control-flow keywords that permissive patterns may
// accidentally capture as function names.
func isIgnoredKeyword(s string) bool {
	switch s {
	case "if", "for", "while", "switch", "return", "else", "catch",
		"do", "match", "when", "with", "case", "select", "defer", "go":
		return true
	}
	return false
}

var wsCollapse = regexp.MustCompile(`[\t ]*\n[\t ]*`)

// cleanSignature trims a raw match down to a compact one-line signature: it
// stops at the opening brace / statement terminator and collapses any internal
// newlines (from multi-line parameter lists) into single spaces.
func cleanSignature(raw string) string {
	raw = strings.TrimSpace(raw)
	if idx := strings.IndexAny(raw, "{;"); idx != -1 {
		raw = strings.TrimSpace(raw[:idx])
	}
	raw = wsCollapse.ReplaceAllString(raw, " ")
	return strings.TrimSuffix(strings.TrimSpace(raw), ":")
}
