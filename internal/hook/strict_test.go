package hook

import (
	"testing"

	"github.com/Ksschkw/driftlock/internal/config"
)

func TestStrictLLMForcesBlocking(t *testing.T) {
	for _, value := range []string{"1", "true", "TRUE", "yes", " yes "} {
		t.Run(value, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Behavior.BlockOnLLMError = false
			cfg.Behavior.BlockOnFalse = false
			t.Setenv("DRIFTLOCK_STRICT_LLM", value)

			applyStrictLLMOverrides(cfg)

			if !cfg.Behavior.BlockOnLLMError {
				t.Error("block_on_llm_error was not forced on")
			}
			if !cfg.Behavior.BlockOnFalse {
				t.Error("block_on_false was not forced on")
			}
		})
	}
}

func TestStrictLLMLeavesConfigAloneWhenUnset(t *testing.T) {
	unsetEnv(t, "DRIFTLOCK_STRICT_LLM")

	cfg := config.DefaultConfig()
	cfg.Behavior.BlockOnLLMError = false
	cfg.Behavior.BlockOnFalse = false

	applyStrictLLMOverrides(cfg)

	if cfg.Behavior.BlockOnLLMError || cfg.Behavior.BlockOnFalse {
		t.Error("config was modified without DRIFTLOCK_STRICT_LLM set")
	}
}

func TestStrictLLMIgnoresNonBooleanValues(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Behavior.BlockOnLLMError = false
	t.Setenv("DRIFTLOCK_STRICT_LLM", "maybe")

	applyStrictLLMOverrides(cfg)

	if cfg.Behavior.BlockOnLLMError {
		t.Error("a non-boolean value enabled strict mode")
	}
}
