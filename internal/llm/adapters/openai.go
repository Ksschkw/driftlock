package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Ksschkw/driftlock/internal/config"
	"github.com/Ksschkw/driftlock/internal/llm/types"
)

type openAICompatible struct {
	cfg     config.LLMConfig
	prompts *config.PromptConfig
	client  *http.Client
}

func NewOpenAICompatible(cfg config.LLMConfig, prompts *config.PromptConfig) (types.Provider, error) {
	if prompts == nil {
		defaultPrompts := types.DefaultPrompts()
		prompts = &defaultPrompts
	}
	return &openAICompatible{
		cfg:     cfg,
		prompts: prompts,
		client:  &http.Client{},
	}, nil
}

func (p *openAICompatible) Check(ctx context.Context, diff, doc string) (bool, string, error) {
	prompt, err := types.RenderPrompt(p.prompts.Check, map[string]string{
		"Diff": diff,
		"Doc":  doc,
	})
	if err != nil {
		return false, "", fmt.Errorf("rendering check prompt: %w", err)
	}
	return p.sendAndParseCheck(ctx, prompt)
}

func (p *openAICompatible) Fix(ctx context.Context, diff, doc string) (string, error) {
	prompt, err := types.RenderPrompt(p.prompts.Fix, map[string]string{
		"Diff": diff,
		"Doc":  doc,
	})
	if err != nil {
		return "", fmt.Errorf("rendering fix prompt: %w", err)
	}
	return p.sendForCompletion(ctx, prompt)
}

func (p *openAICompatible) sendAndParseCheck(ctx context.Context, userPrompt string) (bool, string, error) {
	system := "You are a technical documentation verifier. Answer exactly TRUE or FALSE, then a one-sentence explanation."
	body := map[string]interface{}{
		"model": p.cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": userPrompt},
		},
	}
	if p.cfg.Options != nil {
		for k, v := range p.cfg.Options {
			body[k] = v
		}
	}

	respText, err := p.doRequest(ctx, body)
	if err != nil {
		return false, "", err
	}
	return parseCheckResponse(respText)
}

func (p *openAICompatible) sendForCompletion(ctx context.Context, userPrompt string) (string, error) {
	system := "You are a documentation assistant. Output exactly the updated markdown file, starting from the first character of the document and ending with the last. Do not include any introductory or concluding remarks, explanations, or code fences."
	body := map[string]interface{}{
		"model": p.cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": userPrompt},
		},
	}
	if p.cfg.Options != nil {
		for k, v := range p.cfg.Options {
			body[k] = v
		}
	}
	return p.doRequest(ctx, body)
}

func (p *openAICompatible) doRequest(ctx context.Context, reqBody map[string]interface{}) (string, error) {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	// Use endpoint directly – no path concatenation.
	url := p.cfg.Endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM API error %d: %s", resp.StatusCode, string(respBytes))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return "", fmt.Errorf("failed to parse LLM response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty response from LLM")
	}

	// Strip any conversational preamble the LLM might have added.
	content := stripPreamble(result.Choices[0].Message.Content)
	return content, nil
}

// stripPreamble removes common LLM preamble lines before the actual markdown content.
func stripPreamble(text string) string {
	lines := strings.Split(text, "\n")
	// Skip lines that are clearly not markdown (e.g., introductions)
	for len(lines) > 0 && isPreambleLine(lines[0]) {
		lines = lines[1:]
	}
	return strings.Join(lines, "\n")
}

func isPreambleLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false // blank line could be part of markdown
	}
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(lower, "here") ||
		strings.HasPrefix(lower, "the updated") ||
		strings.HasPrefix(lower, "i have") ||
		strings.HasPrefix(lower, "below") ||
		strings.HasPrefix(lower, "sure") ||
		strings.HasPrefix(lower, "certainly") ||
		strings.HasPrefix(lower, "output")
}

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