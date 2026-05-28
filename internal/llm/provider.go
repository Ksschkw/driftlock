package llm

import (
	"fmt"

	"github.com/Ksschkw/driftlock/internal/config"
	"github.com/Ksschkw/driftlock/internal/llm/adapters"
	"github.com/Ksschkw/driftlock/internal/llm/types"
)

// NewProvider creates a Provider based on the LLM configuration.
func NewProvider(cfg config.LLMConfig, prompts *config.PromptConfig) (types.Provider, error) {
	switch cfg.Driver {
	case "openai-compatible", "groq", "openrouter", "deepseek", "vllm":
		return adapters.NewOpenAICompatible(cfg, prompts)
	case "ollama":
		return adapters.NewOllama(cfg, prompts)
	default:
		return nil, fmt.Errorf("unsupported LLM driver: %s", cfg.Driver)
	}
}