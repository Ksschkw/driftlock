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
		Fix: `The documentation above is outdated for the code changes below. Rewrite the affected sections so that they correctly reflect the new signatures and types. Keep the rest of the document unchanged, including any unrelated examples or commentary. Output the complete updated markdown file, nothing else.

Code diff:
{{ .Diff }}

Current documentation:
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
