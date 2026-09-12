package hook

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// symbolFixture builds a repository whose head adds `Repair` to a file that
// already declared `Greet`, with a README that documents only Greet. The change
// set is therefore purely additive — the case a string comparison settles.
//
// includeLLM controls whether a provider is configured at all, so a
// deterministic run can be shown to need none.
func symbolFixture(t *testing.T, endpoint, checkMode string, includeLLM bool) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")

	base := "package src\n\nfunc Greet(name string) string { return name }\n"
	write(t, filepath.Join(dir, "src", "api.go"), base)
	write(t, filepath.Join(dir, "README.md"), "# API\n\n## Greet\n\n`Greet(name)` greets.\n")

	lines := []string{
		`[[doc_mapping]]`,
		`sources = ["src/**"]`,
		`docs = ["README.md"]`,
		``,
	}
	if includeLLM {
		lines = append(lines,
			`[llm]`,
			`driver = "openai-compatible"`,
			`endpoint = "`+endpoint+`"`,
			`model = "test-model"`,
			`api_key = "test-key"`,
			`timeout_seconds = 5`,
			``,
		)
	}
	lines = append(lines,
		`[behavior]`,
		`auto_fix = false`,
		`block_on_false = true`,
		`block_on_llm_error = true`,
		`max_retries = 0`,
		`cache = false`,
		`check_mode = "`+checkMode+`"`,
		``,
	)
	write(t, filepath.Join(dir, ".driftlock.toml"), strings.Join(lines, "\n"))

	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "base")
	baseSHA := runGit(t, dir, "rev-parse", "HEAD")

	write(t, filepath.Join(dir, "src", "api.go"),
		base+"\nfunc Repair(id int) error { return nil }\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add Repair")
	headSHA := runGit(t, dir, "rev-parse", "HEAD")

	return dir, baseSHA, headSHA
}

// The common case — a new symbol nobody documented — must be decided without
// spending a token. This is the whole point of the deterministic path.
func TestDeterministicPipelineSkipsModelForAddedSymbol(t *testing.T) {
	unsetEnv(t, "DRIFTLOCK_SKIP")

	stub := &verdictServer{verdict: "FALSE. Should never be asked."}
	srv := stub.start(t)
	dir, base, head := symbolFixture(t, srv.URL, "auto", true)
	t.Chdir(dir)

	err := RunWith(context.Background(), Options{BaseRef: base, HeadRef: head})
	if !errors.Is(err, ErrDrift) {
		t.Fatalf("an undocumented new symbol must block, got %v", err)
	}
	if n := len(stub.prompts()); n != 0 {
		t.Errorf("the model was called %d time(s) for a decision a string comparison settles", n)
	}
}

// A modified signature that the documentation does mention needs the model.
func TestDeterministicPipelineUsesModelForModifiedSymbol(t *testing.T) {
	unsetEnv(t, "DRIFTLOCK_SKIP")

	stub := &verdictServer{verdict: "FALSE. The description is stale."}
	srv := stub.start(t)
	dir, base, head := driftFixture(t, srv.URL, true)
	t.Chdir(dir)

	if err := RunWith(context.Background(), Options{BaseRef: base, HeadRef: head}); !errors.Is(err, ErrDrift) {
		t.Fatalf("expected ErrDrift, got %v", err)
	}
	if len(stub.prompts()) == 0 {
		t.Fatal("a mentioned modified signature must be judged by the model")
	}
}

// check_mode=deterministic with no [llm] section at all: the forbidden case for
// any design that constructs a provider unconditionally.
func TestDeterministicModeWorksWithoutAnyLLMConfig(t *testing.T) {
	unsetEnv(t, "DRIFTLOCK_SKIP")

	dir, base, head := symbolFixture(t, "http://127.0.0.1:1", "deterministic", false)
	t.Chdir(dir)

	if err := RunWith(context.Background(), Options{BaseRef: base, HeadRef: head}); !errors.Is(err, ErrDrift) {
		t.Fatalf("deterministic mode should still catch an undocumented symbol, got %v", err)
	}
}

// check_mode=deterministic cannot judge a mentioned modified signature, so it
// reports the change as unjudged rather than guessing, and does not fail.
func TestDeterministicModeSkipsModifiedSignatures(t *testing.T) {
	unsetEnv(t, "DRIFTLOCK_SKIP")

	stub := &verdictServer{verdict: "FALSE. Should never be asked."}
	srv := stub.start(t)
	dir, base, head := driftFixture(t, srv.URL, true)
	t.Chdir(dir)

	// Switch the fixture to deterministic mode.
	cfgPath := filepath.Join(dir, ".driftlock.toml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	write(t, cfgPath, string(data)+"\ncheck_mode = \"deterministic\"\n")

	if err := RunWith(context.Background(), Options{BaseRef: base, HeadRef: head}); err != nil {
		t.Fatalf("deterministic mode should not fail on an unjudgeable change, got %v", err)
	}
	if n := len(stub.prompts()); n != 0 {
		t.Errorf("deterministic mode called the model %d time(s)", n)
	}
}

// check_mode=llm must ignore the deterministic shortcut entirely, so a repository
// that wants model judgement always gets it.
func TestLLMModeAlwaysUsesTheModel(t *testing.T) {
	unsetEnv(t, "DRIFTLOCK_SKIP")

	stub := &verdictServer{verdict: "TRUE. Adding a function is fine."}
	srv := stub.start(t)
	dir, base, head := symbolFixture(t, srv.URL, "llm", true)
	t.Chdir(dir)

	if err := RunWith(context.Background(), Options{BaseRef: base, HeadRef: head}); err != nil {
		t.Fatalf("llm mode returned an error: %v", err)
	}
	if len(stub.prompts()) == 0 {
		t.Fatal("check_mode=llm skipped the model")
	}
}
