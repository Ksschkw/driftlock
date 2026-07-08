package parser

import "strings"

// ignoreMarker, when present in a comment on a declaration line or the line
// immediately above it, excludes that symbol from structural analysis.
const ignoreMarker = "driftlock:ignore"

// sanitize blanks out comment and string spans in the source, replacing their
// characters with spaces while preserving newlines (so byte offsets and line
// numbers are identical to the original). This prevents structural patterns
// from matching inside comments or string literals — the single largest source
// of false positives.
//
// It is a deliberately simple single-pass scanner: it is not a full lexer, but
// it correctly handles the common cases (line comments, block comments, and
// single/multi-character string delimiters with backslash escapes) across the
// mainstream languages Driftlock targets.
func sanitize(source string, spec langSpec) string {
	if spec.dataLike {
		// Structured-data formats: stripping quotes would remove the very keys
		// we want to match, so leave the content intact.
		return source
	}

	b := []byte(source)
	out := make([]byte, len(b))
	copy(out, b)

	blank := func(i int) {
		if out[i] != '\n' {
			out[i] = ' '
		}
	}

	i := 0
	n := len(b)
	for i < n {
		// Line comments.
		if lc := matchAny(b, i, spec.lineComments); lc != "" {
			for i < n && b[i] != '\n' {
				blank(i)
				i++
			}
			continue
		}
		// Block comments.
		if open, close := matchBlock(b, i, spec.blockComment); open != "" {
			for j := 0; j < len(open) && i < n; j++ {
				blank(i)
				i++
			}
			for i < n && !hasPrefixAt(b, i, close) {
				blank(i)
				i++
			}
			for j := 0; j < len(close) && i < n; j++ {
				blank(i)
				i++
			}
			continue
		}
		// String literals.
		if delim := matchAny(b, i, spec.stringDelims); delim != "" {
			for j := 0; j < len(delim) && i < n; j++ {
				blank(i)
				i++
			}
			for i < n {
				if b[i] == '\\' && len(delim) == 1 { // escape inside quoted string
					blank(i)
					i++
					if i < n {
						blank(i)
						i++
					}
					continue
				}
				if hasPrefixAt(b, i, delim) {
					for j := 0; j < len(delim) && i < n; j++ {
						blank(i)
						i++
					}
					break
				}
				blank(i)
				i++
			}
			continue
		}
		i++
	}
	return string(out)
}

// matchAny returns the first token in toks that is a prefix of b[i:], preferring
// longer tokens (so "\"\"\"" wins over "\"").
func matchAny(b []byte, i int, toks []string) string {
	best := ""
	for _, t := range toks {
		if len(t) > len(best) && hasPrefixAt(b, i, t) {
			best = t
		}
	}
	return best
}

func matchBlock(b []byte, i int, blocks [][2]string) (string, string) {
	for _, pair := range blocks {
		if hasPrefixAt(b, i, pair[0]) {
			return pair[0], pair[1]
		}
	}
	return "", ""
}

func hasPrefixAt(b []byte, i int, tok string) bool {
	if tok == "" || i+len(tok) > len(b) {
		return false
	}
	for j := 0; j < len(tok); j++ {
		if b[i+j] != tok[j] {
			return false
		}
	}
	return true
}

// ignoredLines scans the ORIGINAL (un-sanitized) source for the ignore marker
// and returns the set of 1-based line numbers whose declarations should be
// skipped. Two placements are supported:
//
//	func F() {} // driftlock:ignore   → inline: only this line is ignored
//	// driftlock:ignore
//	func F() {}                        → standalone: the NEXT line is ignored
//
// An inline marker must not suppress the following declaration, so the next
// line is only added when the marker sits on a comment-only line.
func ignoredLines(source string, spec langSpec) map[int]bool {
	ignored := map[int]bool{}
	openers := append([]string{}, spec.lineComments...)
	for _, bc := range spec.blockComment {
		openers = append(openers, bc[0])
	}

	for idx, line := range strings.Split(source, "\n") {
		pos := strings.Index(line, ignoreMarker)
		if pos < 0 {
			continue
		}
		lineNo := idx + 1
		ignored[lineNo] = true

		before := line[:pos]
		commentOnly := strings.TrimSpace(before) == ""
		if !commentOnly {
			for _, op := range openers {
				if p := strings.Index(before, op); p >= 0 && strings.TrimSpace(before[:p]) == "" {
					commentOnly = true
					break
				}
			}
		}
		if commentOnly {
			ignored[lineNo+1] = true
		}
	}
	return ignored
}

// lineOf returns the 1-based line number for a byte offset in source.
func lineOf(source string, offset int) int {
	if offset > len(source) {
		offset = len(source)
	}
	return strings.Count(source[:offset], "\n") + 1
}
