package hook

import (
	"strings"
	"testing"
)

// A fail-open gate that is quiet is indistinguishable from a pass. The message
// must say the commit went through unverified, name the documents, and offer a
// next step.
func TestLLMErrorMessageFailOpen(t *testing.T) {
	msg := llmErrorMessage([]string{"README.md", "docs/api.md"}, false)
	for _, want := range []string{"2 document(s)", "README.md", "docs/api.md", "proceeding UNCHECKED", "block_on_llm_error", "timeout_seconds", "DRIFTLOCK_DEBUG"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q: %s", want, msg)
		}
	}
}

func TestLLMErrorMessageBlocked(t *testing.T) {
	msg := llmErrorMessage([]string{"README.md"}, true)
	if !strings.Contains(msg, "blocked") {
		t.Errorf("blocked outcome not stated: %s", msg)
	}
	if strings.Contains(msg, "UNCHECKED") {
		t.Errorf("blocked message must not claim the commit proceeded: %s", msg)
	}
}

func TestLLMErrorMessageHandlesNoDocs(t *testing.T) {
	msg := llmErrorMessage(nil, false)
	if !strings.Contains(msg, "0 document(s)") || !strings.Contains(msg, "unknown") {
		t.Errorf("unexpected message for no docs: %s", msg)
	}
}
