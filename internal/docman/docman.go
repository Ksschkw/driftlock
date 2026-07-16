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
// heading). It replaces the content of those sections in the full document and
// returns the resulting complete document.
//
// Sections in updatedSections whose headings match a heading in fullDoc
// replace that section's content. Sections with genuinely NEW headings (e.g.
// documentation for a symbol that had no section before) are appended to the
// end of the document — previously they were silently dropped, which meant
// auto-fix could never document a brand-new symbol under its own heading.
func MergeSectionUpdates(fullDoc string, updatedSections string) string {
	if updatedSections == "" {
		return fullDoc
	}

	// Leaf (non-overlapping) splits are essential here: the hierarchical split
	// used for extraction includes subsection text inside the parent's content,
	// so reconstructing or appending from it duplicates every nested section
	// (observed in E2E as a doubled '### ...' block in the user's README).
	origSections := splitIntoLeafSections(fullDoc)
	updated := splitIntoLeafSections(updatedSections)

	origHeadings := make(map[string]bool)
	for _, os := range origSections {
		origHeadings[headingKey(os.heading)] = true
	}

	// Headings present in the original are replacements; unseen headings are
	// new sections, deduplicated and appended at the end.
	replacements := make(map[string]string)
	seenNew := make(map[string]bool)
	var newSections []section
	for _, us := range updated {
		key := headingKey(us.heading)
		if key == "" {
			continue // preamble noise from the model is never merged
		}
		if !origHeadings[key] {
			if !seenNew[key] {
				seenNew[key] = true
				newSections = append(newSections, us)
			}
			continue
		}
		replacements[key] = us.content
	}

	var result strings.Builder
	for _, os := range origSections {
		result.WriteString(os.heading)
		if newContent, ok := replacements[headingKey(os.heading)]; ok && os.heading != "" {
			result.WriteString(newContent)
		} else {
			result.WriteString(os.content)
		}
	}
	for _, ns := range newSections {
		if !strings.HasSuffix(result.String(), "\n") {
			result.WriteString("\n")
		}
		result.WriteString(ns.heading)
		result.WriteString(ns.content)
	}
	return result.String()
}

// headingKey normalizes a heading line for matching: surrounding whitespace is
// insignificant. LLMs frequently emit an otherwise-identical heading with a
// trailing space, which previously defeated both replacement matching and
// new-section deduplication (observed as a duplicated section in E2E).
func headingKey(heading string) string {
	return strings.TrimSpace(heading)
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

// splitIntoLeafSections splits md into NON-overlapping segments: each
// section's content ends at the next heading of ANY level. Unlike
// splitIntoSections (hierarchical, used for extraction), reassembling these
// segments reproduces the document exactly — including any preamble before
// the first heading, which the hierarchical split drops.
func splitIntoLeafSections(md string) []section {
	matches := headingRE.FindAllStringSubmatchIndex(md, -1)
	if len(matches) == 0 {
		return []section{{heading: "", content: md}}
	}
	var secs []section
	if matches[0][0] > 0 {
		secs = append(secs, section{heading: "", content: md[:matches[0][0]]})
	}
	for i, m := range matches {
		end := len(md)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		secs = append(secs, section{heading: md[m[0]:m[1]], content: md[m[1]:end]})
	}
	return secs
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
