package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	// "strings"

	"github.com/Ksschkw/driftlock/internal/config"
	"github.com/Ksschkw/driftlock/internal/llm/types"
)

type ollama struct {
	cfg     config.LLMConfig
	prompts *config.PromptConfig
	client  *http.Client
}

func NewOllama(cfg config.LLMConfig, prompts *config.PromptConfig) (types.Provider, error) {
	if prompts == nil {
		defaultPrompts := types.DefaultPrompts()
		prompts = &defaultPrompts
	}
	return &ollama{
		cfg:     cfg,
		prompts: prompts,
		client:  &http.Client{},
	}, nil
}

func (o *ollama) Check(ctx context.Context, diff, doc string) (bool, string, error) {
	prompt, err := types.RenderPrompt(o.prompts.Check, map[string]string{"Diff": diff, "Doc": doc})
	if err != nil {
		return false, "", err
	}
	resp, err := o.generate(ctx, prompt)
	if err != nil {
		return false, "", err
	}
	return parseCheckResponse(resp)
}

func (o *ollama) Fix(ctx context.Context, diff, doc string) (string, error) {
	prompt, err := types.RenderPrompt(o.prompts.Fix, map[string]string{"Diff": diff, "Doc": doc})
	if err != nil {
		return "", err
	}
	resp, err := o.generate(ctx, prompt)
	if err != nil {
		return "", err
	}
	return stripPreamble(resp), nil
}

func (o *ollama) generate(ctx context.Context, prompt string) (string, error) {
	body := map[string]interface{}{
		"model":  o.cfg.Model,
		"prompt": prompt,
		"stream": false,
	}
	if o.cfg.Options != nil {
		for k, v := range o.cfg.Options {
			body[k] = v
		}
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	// Use endpoint directly – no path concatenation.
	url := o.cfg.Endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var result struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return "", fmt.Errorf("failed to parse Ollama response: %w", err)
	}
	return result.Response, nil
}