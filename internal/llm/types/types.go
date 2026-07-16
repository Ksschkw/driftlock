package types

import (
	"bytes"
	"context"
	"text/template"

	"github.com/Ksschkw/driftlock/internal/config"
)

// Provider defines the interface for LLM interactions.
type Provider interface {
	Check(ctx context.Context, diff, doc string) (bool, string, error)
	Fix(ctx context.Context, diff, doc string) (string, error)
}

// DefaultPrompts returns the built-in check and fix prompts.
func DefaultPrompts() config.PromptConfig {
	return config.PromptConfig{
		Check: `You are a precise documentation auditor. Below is a list of structural code changes (signatures added, removed, or modified), followed by documentation that should describe this code as it NOW exists.

Judge exactly one thing: is the documentation consistent with the CURRENT (new) state of the signatures?
- Modified signature: the documentation must describe the new form and must not describe only the old form.
- Added symbol: the documentation must mention it.
- Removed symbol: the documentation must not still present it as existing. It does NOT need to say a removal happened.
- The documentation is NOT required to narrate the change history — only to be accurate about the API as it now is.
- Do not invent requirements beyond the listed changes.

Structural changes:
{{ .Diff }}

Documentation:
{{ .Doc }}

Answer exactly TRUE if the documentation is consistent with the current signatures, otherwise FALSE. Then provide a one-sentence explanation.`,
		Fix: `Update the documentation fragment below so it accurately reflects the code changes. Preserve every Markdown heading exactly as given (identical text and level) — the result is merged back into the full document by matching headings. Keep ALL existing content that is still accurate: update what the code changed, add what is missing, and delete nothing that remains true. Output only the updated Markdown, nothing else.

Code changes:
{{ .Diff }}

Documentation to update:
{{ .Doc }}`,
	}
}

// RenderPrompt applies the given template string and data map.
func RenderPrompt(tmplStr string, data map[string]string) (string, error) {
	tmpl, err := template.New("prompt").Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
