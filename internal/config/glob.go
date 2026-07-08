package config

import "strings"

// MatchGlob reports whether path matches a glob pattern with full support for:
//
//   - any run of characters within a single path segment (no "/")
//     **  any number of characters across segments (spans "/")
//     ?   any single non-separator character
//
// Paths are matched using forward slashes regardless of OS. This replaces
// filepath.Match, which does not understand "**" and cannot express recursive
// source trees like "internal/**" or "src/**/*.go".
func MatchGlob(pattern, path string) bool {
	pattern = normalizeSlashes(pattern)
	path = normalizeSlashes(path)
	return matchSegments(pattern, path)
}

func normalizeSlashes(s string) string {
	return strings.ReplaceAll(s, "\\", "/")
}

// matchSegments implements glob matching over the raw strings, treating "**"
// specially so it can cross path separators.
func matchSegments(pattern, name string) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '*':
			if len(pattern) > 1 && pattern[1] == '*' {
				// "**" — consume the star-star (and an optional trailing slash)
				// and try to match the remainder at every position, crossing
				// path separators.
				rest := pattern[2:]
				rest = strings.TrimPrefix(rest, "/")
				if rest == "" {
					return true // trailing ** matches everything remaining
				}
				for i := 0; i <= len(name); i++ {
					if matchSegments(rest, name[i:]) {
						return true
					}
				}
				return false
			}
			// single "*" — match within a segment only (no "/").
			for i := 0; i <= len(name); i++ {
				if i < len(name) && name[i] == '/' {
					break
				}
				if matchSegments(pattern[1:], name[i:]) {
					return true
				}
			}
			return false
		case '?':
			if len(name) == 0 || name[0] == '/' {
				return false
			}
			pattern = pattern[1:]
			name = name[1:]
		default:
			if len(name) == 0 || name[0] != pattern[0] {
				return false
			}
			pattern = pattern[1:]
			name = name[1:]
		}
	}
	return len(name) == 0
}
