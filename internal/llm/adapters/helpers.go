package adapters

import (
	"fmt"
	"strings"
)

// stripPreambleMarkdown removes any lines before the first markdown heading (#) or code fence (```).
// If neither is found, the whole content is returned unchanged.
func stripPreambleMarkdown(content string) string {
	lines := strings.Split(content, "\n")
	start := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "```") {
			start = i
			break
		}
	}
	if start >= len(lines) {
		return ""
	}
	return strings.Join(lines[start:], "\n")
}

// parseCheckResponse extracts TRUE/FALSE and explanation from the LLM response.
func parseCheckResponse(text string) (bool, string, error) {
	text = strings.TrimSpace(text)
	upper := strings.ToUpper(text)
	if strings.HasPrefix(upper, "TRUE") {
		return true, text, nil
	}
	if strings.HasPrefix(upper, "FALSE") {
		return false, text, nil
	}
	return false, text, fmt.Errorf("could not parse TRUE/FALSE from response: %s", text)
}
