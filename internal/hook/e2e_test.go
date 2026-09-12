package hook

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// verdictServer is a stub OpenAI-compatible endpoint. It lets the whole
// pipeline run deterministically — real git, real parser, real diff, real doc
// chunking, real HTTP adapter and JSON parsing — without a provider.
type verdictServer struct {
	mu       sync.Mutex
	requests []string
	verdict  string
}

func (v *verdictServer) start(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		var prompt strings.Builder
		for _, m := range body.Messages {
			prompt.WriteString(m.Content)
		}
		v.mu.Lock()
		v.requests = append(v.requests, prompt.String())
		verdict := v.verdict
		v.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": verdict}},
			},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (v *verdictServer) prompts() []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return append([]string(nil), v.requests...)
}

func boolLiteral(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"-C", dir, "-c", "user.email=test@example.com", "-c", "user.name=Test"}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// driftFixture builds a real git repository with a committed API change and a
// documentation file that maps to it, and returns (root, baseSHA, headSHA).
func driftFixture(t *testing.T, endpoint string, blockOnLLMError bool) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")

	write(t, filepath.Join(dir, "src", "greet.go"),
		"package src\n\nfunc Greet(name string) string { return name }\n")
	write(t, filepath.Join(dir, "README.md"),
		"# API\n\n## Greet\n\n`Greet(name)` returns a greeting.\n")
	write(t, filepath.Join(dir, ".driftlock.toml"), strings.Join([]string{
		`[[doc_mapping]]`,
		`sources = ["src/**"]`,
		`docs = ["README.md"]`,
		``,
		`[llm]`,
		`driver = "openai-compatible"`,
		`endpoint = "` + endpoint + `"`,
		`model = "test-model"`,
		`api_key = "test-key"`,
		`timeout_seconds = 5`,
		``,
		`[behavior]`,
		`auto_fix = false`,
		`block_on_false = true`,
		`block_on_llm_error = ` + boolLiteral(blockOnLLMError),
		`max_retries = 0`,
		`cache = false`,
		``,
	}, "\n"))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "base")
	base := runGit(t, dir, "rev-parse", "HEAD")

	// A structural change: Greet gains a parameter.
	write(t, filepath.Join(dir, "src", "greet.go"),
		"package src\n\nfunc Greet(name string, excited bool) string { return name }\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "change signature")
	head := runGit(t, dir, "rev-parse", "HEAD")

	return dir, base, head
}

// A drifted range must produce the drift decision end to end.
func TestEndToEndDriftIsDetected(t *testing.T) {
	unsetEnv(t, "DRIFTLOCK_SKIP")

	stub := &verdictServer{verdict: "FALSE. Greet now takes an excited flag that the documentation does not mention."}
	srv := stub.start(t)

	dir, base, head := driftFixture(t, srv.URL, true)
	t.Chdir(dir)

	err := RunWith(context.Background(), Options{BaseRef: base, HeadRef: head})
	if !errors.Is(err, ErrDrift) {
		t.Fatalf("expected ErrDrift, got %v", err)
	}

	prompts := stub.prompts()
	if len(prompts) == 0 {
		t.Fatal("the LLM was never called")
	}
	// The prompt must carry the structural change, not just the doc.
	if !strings.Contains(prompts[0], "Greet") || !strings.Contains(prompts[0], "added") && !strings.Contains(prompts[0], "modified") {
		t.Errorf("prompt does not describe the structural change:\n%s", prompts[0])
	}
}

// A consistent range must pass end to end.
func TestEndToEndConsistentDocsPass(t *testing.T) {
	unsetEnv(t, "DRIFTLOCK_SKIP")

	stub := &verdictServer{verdict: "TRUE. The documentation matches the current signatures."}
	srv := stub.start(t)

	dir, base, head := driftFixture(t, srv.URL, true)
	t.Chdir(dir)

	if err := RunWith(context.Background(), Options{BaseRef: base, HeadRef: head}); err != nil {
		t.Fatalf("expected a clean pass, got %v", err)
	}
	if len(stub.prompts()) == 0 {
		t.Fatal("the LLM was never called")
	}
}

// A provider outage with block_on_llm_error must fail the range.
func TestEndToEndLLMErrorBlocks(t *testing.T) {
	unsetEnv(t, "DRIFTLOCK_SKIP")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	dir, base, head := driftFixture(t, srv.URL, true)
	t.Chdir(dir)

	// Dry-run reports the error rather than exiting; range mode is inherently
	// dry-run, so the pipeline returns instead of calling os.Exit.
	err := RunWith(context.Background(), Options{BaseRef: base, HeadRef: head})
	if !errors.Is(err, ErrLLMUnreachable) {
		t.Fatalf("expected ErrLLMUnreachable, got %v", err)
	}
}

// A body-only edit must not call the model at all: this is the silence that
// makes Driftlock usable.
func TestEndToEndBodyOnlyEditIsSilent(t *testing.T) {
	unsetEnv(t, "DRIFTLOCK_SKIP")

	stub := &verdictServer{verdict: "TRUE."}
	srv := stub.start(t)

	dir, _, _ := driftFixture(t, srv.URL, true)
	// Change only the function body and commit on top of head.
	write(t, filepath.Join(dir, "src", "greet.go"),
		"package src\n\nfunc Greet(name string, excited bool) string { return name + \"!\" }\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "body only")

	base := runGit(t, dir, "rev-parse", "HEAD~1")
	head := runGit(t, dir, "rev-parse", "HEAD")
	t.Chdir(dir)

	if err := RunWith(context.Background(), Options{BaseRef: base, HeadRef: head}); err != nil {
		t.Fatalf("body-only edit should pass, got %v", err)
	}
	if n := len(stub.prompts()); n != 0 {
		t.Errorf("body-only edit called the LLM %d time(s); it must be silent", n)
	}
}

// DRIFTLOCK_STRICT_LLM must override a config that permits proceeding on an
// unreachable provider. This is the CI guarantee: a committed config cannot
// downgrade the gate.
func TestEndToEndStrictLLMOverridesLenientConfig(t *testing.T) {
	unsetEnv(t, "DRIFTLOCK_SKIP")
	t.Setenv("DRIFTLOCK_STRICT_LLM", "1")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	dir, base, head := driftFixture(t, srv.URL, false)
	t.Chdir(dir)

	if err := RunWith(context.Background(), Options{BaseRef: base, HeadRef: head}); !errors.Is(err, ErrLLMUnreachable) {
		t.Fatalf("expected ErrLLMUnreachable under strict mode, got %v", err)
	}
}

// Without strict mode the same lenient config allows the run to pass, which is
// the documented local-development trade.
func TestEndToEndLenientConfigAllowsProviderError(t *testing.T) {
	unsetEnv(t, "DRIFTLOCK_SKIP")
	unsetEnv(t, "DRIFTLOCK_STRICT_LLM")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	dir, base, head := driftFixture(t, srv.URL, false)
	t.Chdir(dir)

	if err := RunWith(context.Background(), Options{BaseRef: base, HeadRef: head}); err != nil {
		t.Fatalf("a lenient config should allow the run, got %v", err)
	}
}
