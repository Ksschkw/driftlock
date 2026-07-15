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
		Check: `You are a precise documentation auditor. Below is a list of structural code changes (function/method signatures that have been added, removed, or modified). Determine whether the provided documentation file explicitly mentions and accurately describes all of these specific changes.

Structural changes:
{{ .Diff }}

Documentation file:
{{ .Doc }}

Answer exactly TRUE if every changed signature is correctly reflected in the documentation, otherwise FALSE. Then provide a one-sentence explanation.`,
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
