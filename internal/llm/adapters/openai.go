package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

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
	system := documentationFixSystemPrompt
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
	raw, err := p.doRequest(ctx, body)
	if err != nil {
		return "", err
	}
	if os.Getenv("DRIFTLOCK_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[DEBUG] LLM raw response:\n%s\n[END DEBUG]\n", raw)
	}
	cleaned := stripPreambleMarkdown(raw)
	return cleaned, nil
}

func (p *openAICompatible) doRequest(ctx context.Context, reqBody map[string]interface{}) (string, error) {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	if os.Getenv("DRIFTLOCK_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[DEBUG] LLM request body:\n%s\n", string(jsonBody))
	}

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
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return "", fmt.Errorf("failed to parse LLM response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty response from LLM")
	}

	if os.Getenv("DRIFTLOCK_DEBUG") != "" && result.Usage.TotalTokens > 0 {
		fmt.Fprintf(os.Stderr, "[DEBUG] token usage: prompt=%d completion=%d total=%d\n",
			result.Usage.PromptTokens, result.Usage.CompletionTokens, result.Usage.TotalTokens)
	}

	return result.Choices[0].Message.Content, nil
}
