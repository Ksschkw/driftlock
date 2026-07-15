package docman

import (
	"regexp"
	"strings"
)

var headingRE = regexp.MustCompile(`(?m)^(#{1,6})\s+(.*)`)

// ExtractRelevantSections returns only the most specific sections of the
// markdown document whose content mentions any of the given names, plus a
// separate note describing symbols that appear nowhere in the document.
// Ancestor sections are excluded to avoid duplicated content. The full
// document is never returned.
//
// The note MUST NOT be embedded in the returned sections: it is Driftlock
// metadata for the LLM's context, and embedding it in the doc content caused
// it to be preserved verbatim into users' documentation by the auto-fix.
func ExtractRelevantSections(markdown string, names []string) (sections string, note string) {
	if len(names) == 0 {
		return "", "No changed symbols were provided."
	}

	lowerNames := make(map[string]bool)
	for _, n := range names {
		lowerNames[strings.ToLower(n)] = true
	}

	secs := splitIntoSections(markdown)

	// 1. Find all sections that mention any changed name.
	matchingIdx := make([]bool, len(secs))
	for i, sec := range secs {
		lower := strings.ToLower(sec.content)
		for name := range lowerNames {
			if strings.Contains(lower, name) {
				matchingIdx[i] = true
				break
			}
		}
	}

	// 2. Keep only the deepest matching sections: a section is kept if none
	//    of its descendants are also matching. This avoids duplicate content.
	var selected []string
	for i, sec := range secs {
		if !matchingIdx[i] {
			continue
		}
		hasMatchingChild := false
		level := countHeadingLevel(sec.heading)
		for j := i + 1; j < len(secs); j++ {
			childLevel := countHeadingLevel(secs[j].heading)
			if childLevel <= level {
				break
			}
			if matchingIdx[j] {
				hasMatchingChild = true
				break
			}
		}
		if !hasMatchingChild {
			selected = append(selected, sec.heading+sec.content)
		}
	}

	// 3. Identify any names that never appeared in the document. The original
	//    (case-preserved) name is reported so the LLM sees the real symbol.
	undocumented := make([]string, 0, len(names))
	for _, name := range names {
		lower := strings.ToLower(name)
		found := false
		for _, sec := range secs {
			if strings.Contains(strings.ToLower(sec.content), lower) {
				found = true
				break
			}
		}
		if !found {
			undocumented = append(undocumented, name)
		}
	}

	if len(undocumented) > 0 {
		note = "The following changed symbols are not mentioned in the documentation: " + strings.Join(undocumented, ", ") + "."
	}
	if len(selected) > 0 {
		return strings.Join(selected, "\n"), note
	}
	if note == "" {
		note = "The documentation does not mention any of the changed symbols."
	}
	return "", note
}

// MergeSectionUpdates takes the full original markdown document and a string
// that contains one or more updated sections (each starting with the original
// heading).  It replaces the content of those sections in the full document
// and returns the resulting complete document.
//
// The updatedSections string is expected to be a concatenation of sections,
// each beginning with a markdown heading that exactly matches a heading in
// fullDoc.  The replacement matches by heading text and heading level.
func MergeSectionUpdates(fullDoc string, updatedSections string) string {
	if updatedSections == "" {
		return fullDoc
	}

	origSections := splitIntoSections(fullDoc)
	updated := splitIntoSections(updatedSections)

	// Build a map from (heading level + text) to the new content for that section.
	replacements := make(map[string]string)
	for _, us := range updated {
		key := strings.TrimRight(us.heading, "\n")
		replacements[key] = us.content
	}

	// Reconstruct the full document, replacing matching sections.
	var result strings.Builder
	for _, os := range origSections {
		key := strings.TrimRight(os.heading, "\n")
		if newContent, ok := replacements[key]; ok {
			result.WriteString(os.heading)
			result.WriteString(newContent)
		} else {
			result.WriteString(os.heading)
			result.WriteString(os.content)
		}
	}
	return result.String()
}

// countHeadingLevel returns the number of '#' characters at the beginning of the heading line.
func countHeadingLevel(heading string) int {
	h := strings.TrimRight(heading, "\n")
	level := 0
	for _, r := range h {
		if r == '#' {
			level++
		} else {
			break
		}
	}
	return level
}

type section struct {
	heading string
	content string
}

func splitIntoSections(md string) []section {
	matches := headingRE.FindAllStringSubmatchIndex(md, -1)
	if len(matches) == 0 {
		return []section{{heading: "", content: md}}
	}

	var secs []section
	for i, m := range matches {
		headingStart := m[0]
		headingEnd := m[1]
		level := len(md[m[2]:m[3]])

		contentStart := headingEnd
		contentEnd := len(md)

		for j := i + 1; j < len(matches); j++ {
			nextLevel := len(md[matches[j][2]:matches[j][3]])
			if nextLevel <= level {
				contentEnd = matches[j][0]
				break
			}
		}

		headingLine := md[headingStart:headingEnd]
		content := md[contentStart:contentEnd]
		secs = append(secs, section{heading: headingLine, content: content})
	}
	return secs
}
