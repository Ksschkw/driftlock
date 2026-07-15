package adapters

import (
	"fmt"
	"regexp"
	"strings"
)

// documentationFixSystemPrompt governs the auto-fix rewrite. Driftlock may
// hand the model either a full document (the `fix` command) or a small set of
// extracted sections (the chunked pre-commit path). In both cases the required
// output is the same: a rewrite of *exactly the content provided*, with every
// heading preserved verbatim so the result can be stitched back into the source
// document by heading match. Telling the model to emit "the complete file"
// (the previous behavior) was wrong for the chunked path and caused the model
// to invent surrounding structure that would then fail to merge.
const documentationFixSystemPrompt = `You are a precise documentation editor. You are given a fragment of Markdown documentation (one or more sections) that is outdated with respect to some code changes. Rewrite it so it accurately reflects the new signatures, types, and behavior.

Rules:
- Rewrite ONLY the content you are given. Do not add unrelated sections or invent surrounding document structure.
- Preserve every Markdown heading EXACTLY as provided — identical text and identical level (number of leading '#'). The output is stitched back into the full document by matching these headings, so any change to a heading will drop the update.
- Keep the existing writing style, tone, and formatting. Preserve parts that are still correct.
- If a changed symbol is not yet documented, add a concise description under the most appropriate existing heading you were given.
- Output ONLY the rewritten Markdown. No preamble, no explanations, no surrounding code fences.`

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

var verdictRE = regexp.MustCompile(`(?i)\b(TRUE|FALSE)\b`)

// thinkRE matches the <think>...</think> reasoning blocks emitted by
// reasoning models (DeepSeek-R1, QwQ, etc.). Their contents are scratch work,
// not the answer, and must never influence verdict parsing or doc output.
var thinkRE = regexp.MustCompile(`(?is)<think>.*?</think>`)

// stripReasoning removes reasoning-model scratch blocks from a response.
func stripReasoning(text string) string {
	return strings.TrimSpace(thinkRE.ReplaceAllString(text, ""))
}

// parseCheckResponse extracts a TRUE/FALSE verdict and explanation from an LLM
// response. It is deliberately tolerant: models routinely wrap the verdict in
// markdown (`**FALSE**`), prefix it with reasoning ("Answer: FALSE"), or lead
// with a JSON-ish object. Rather than demanding the response *start* with the
// verdict (the previous behavior, which produced spurious "could not parse"
// errors and, with block_on_llm_error, spurious commit blocks), we strip
// common decorations and take the first standalone TRUE/FALSE token.
func parseCheckResponse(text string) (bool, string, error) {
	clean := stripReasoning(text)
	// Strip surrounding markdown emphasis/code fences that models add.
	stripped := strings.Map(func(r rune) rune {
		switch r {
		case '*', '`', '#', '"', '\'':
			return -1
		}
		return r
	}, clean)

	// The system prompt instructs the model to answer with the verdict FIRST,
	// so a leading TRUE/FALSE is authoritative and immune to the word "true"
	// appearing later in reasoning prose.
	upper := strings.ToUpper(strings.TrimSpace(stripped))
	switch {
	case strings.HasPrefix(upper, "TRUE"):
		return true, clean, nil
	case strings.HasPrefix(upper, "FALSE"):
		return false, clean, nil
	}

	// Fallback: first standalone verdict token anywhere in the response.
	loc := verdictRE.FindStringIndex(stripped)
	if loc == nil {
		return false, clean, fmt.Errorf("could not parse TRUE/FALSE from response: %q", truncate(clean, 200))
	}
	return strings.EqualFold(stripped[loc[0]:loc[1]], "TRUE"), clean, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
