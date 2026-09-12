package hook

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Ksschkw/driftlock/internal/config"
)

// failingProvider always fails, counting calls, so retry behaviour can be
// observed without a network or a real model.
type failingProvider struct{ calls int }

func (f *failingProvider) Check(ctx context.Context, diff, doc string) (bool, string, error) {
	f.calls++
	return false, "", errors.New("provider unavailable")
}

func (f *failingProvider) Fix(ctx context.Context, diff, doc string) (string, error) {
	return "", errors.New("provider unavailable")
}

// A cancelled context must return promptly instead of sleeping out the
// exponential backoff. The old implementation used time.Sleep, so a drained
// deadline still blocked the commit for the full 2s + 4s + 8s.
func TestCheckDocWithRetryHonoursCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := &failingProvider{}
	start := time.Now()
	result := checkDocWithRetry(ctx, p, 3, "diff", "doc")
	elapsed := time.Since(start)

	if result.err == nil {
		t.Fatal("expected an error from a cancelled context")
	}
	if p.calls != 0 {
		t.Errorf("made %d call(s) on a cancelled context, want 0", p.calls)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("returned after %v; should be immediate", elapsed)
	}
}

// A context that expires during backoff must abort the wait.
func TestCheckDocWithRetryAbortsDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	p := &failingProvider{}
	start := time.Now()
	result := checkDocWithRetry(ctx, p, 3, "diff", "doc")
	elapsed := time.Since(start)

	if result.err == nil {
		t.Fatal("expected an error once the context expired")
	}
	// First attempt is immediate; the 2s backoff must be cut short.
	if elapsed > 1500*time.Millisecond {
		t.Errorf("backoff was not interrupted: waited %v", elapsed)
	}
	if p.calls != 1 {
		t.Errorf("provider called %d time(s), want 1", p.calls)
	}
}

func TestPerCheckBudgetGrowsWithRetries(t *testing.T) {
	base := &config.Config{}
	base.LLM.TimeoutSeconds = 10
	base.Behavior.MaxRetries = 0
	noRetry := perCheckBudget(base)

	base.Behavior.MaxRetries = 3
	withRetry := perCheckBudget(base)

	if noRetry <= 0 || withRetry <= 0 {
		t.Fatalf("budgets must be positive: %v, %v", noRetry, withRetry)
	}
	if withRetry <= noRetry {
		t.Errorf("budget with retries (%v) should exceed the no-retry budget (%v)", withRetry, noRetry)
	}
	// 4 attempts x 10s + (2+4+8)s backoff + 1s slack = 55s
	want := 55 * time.Second
	if withRetry != want {
		t.Errorf("perCheckBudget = %v, want %v", withRetry, want)
	}
}
