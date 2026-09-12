package hook

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeDriftlockConfig writes a minimal config pointing at the stub provider.
func writeDriftlockConfig(t *testing.T, dir, endpoint string, autoFix, blockOnLLMError bool) {
	t.Helper()
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
		`auto_fix = ` + boolLiteral(autoFix),
		`block_on_false = true`,
		`block_on_llm_error = ` + boolLiteral(blockOnLLMError),
		`max_retries = 0`,
		`cache = false`,
		``,
	}, "\n"))
}

// stagedFixture builds a repository whose HEAD holds the original signature and
// whose INDEX holds a changed one — the local pre-commit situation, as opposed
// to the range mode CI uses.
func stagedFixture(t *testing.T, endpoint string, autoFix bool) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")

	write(t, filepath.Join(dir, "src", "greet.go"),
		"package src\n\nfunc Greet(name string) string { return name }\n")
	write(t, filepath.Join(dir, "README.md"),
		"# API\n\n## Greet\n\n`Greet(name)` returns a greeting.\n")
	writeDriftlockConfig(t, dir, endpoint, autoFix, true)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "base")

	write(t, filepath.Join(dir, "src", "greet.go"),
		"package src\n\nfunc Greet(name string, excited bool) string { return name }\n")
	runGit(t, dir, "add", "src/greet.go")
	return dir
}

func captureHookStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(r)
		done <- string(data)
	}()
	runErr := fn()
	_ = w.Close()
	os.Stdout = old
	return <-done, runErr
}

// The local staged path must block, which is what the pre-commit hook relies on.
func TestPipelineStagedCommitIsBlocked(t *testing.T) {
	unsetEnv(t, "DRIFTLOCK_SKIP")

	stub := &verdictServer{verdict: "FALSE. Greet gained an excited parameter."}
	srv := stub.start(t)
	dir := stagedFixture(t, srv.URL, false)
	t.Chdir(dir)

	if err := RunWith(context.Background(), Options{}); !errors.Is(err, ErrDrift) {
		t.Fatalf("expected ErrDrift from the staged path, got %v", err)
	}
	if len(stub.prompts()) == 0 {
		t.Fatal("the model was never consulted")
	}
}

// With auto_fix on, the staged path rewrites the doc and still blocks so the
// author reviews the result.
func TestPipelineStagedAutoFixRewritesDocAndBlocks(t *testing.T) {
	unsetEnv(t, "DRIFTLOCK_SKIP")

	stub := &verdictServer{
		verdict: "FALSE. Greet gained an excited parameter.",
		fix:     "## Greet\n\n`Greet(name, excited)` returns a greeting.\n",
	}
	srv := stub.start(t)
	dir := stagedFixture(t, srv.URL, true)
	t.Chdir(dir)

	if err := RunWith(context.Background(), Options{}); !errors.Is(err, ErrDrift) {
		t.Fatalf("a reviewed fix must still block, got %v", err)
	}

	doc, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc), "excited") {
		t.Errorf("auto-fix did not rewrite the document:\n%s", doc)
	}
	// The heading must survive the stitch-back, or the section would be lost.
	if !strings.Contains(string(doc), "## Greet") {
		t.Errorf("auto-fix destroyed the section heading:\n%s", doc)
	}
}

// Report mode is informational: drift is reported but never blocks.
func TestPipelineReportModeNeverFails(t *testing.T) {
	unsetEnv(t, "DRIFTLOCK_SKIP")

	stub := &verdictServer{verdict: "FALSE. Drifted."}
	srv := stub.start(t)
	dir, base, head := driftFixture(t, srv.URL, true)
	t.Chdir(dir)

	if err := RunWith(context.Background(), Options{BaseRef: base, HeadRef: head, Report: true}); err != nil {
		t.Fatalf("report mode must not fail, got %v", err)
	}
}

// The JSON report is the CI contract; it must be valid JSON on stdout and
// separately describe drift and the per-document result.
func TestPipelineJSONReport(t *testing.T) {
	unsetEnv(t, "DRIFTLOCK_SKIP")

	stub := &verdictServer{verdict: "FALSE. Greet gained a parameter."}
	srv := stub.start(t)
	dir, base, head := driftFixture(t, srv.URL, true)
	t.Chdir(dir)

	out, err := captureHookStdout(t, func() error {
		return RunWith(context.Background(), Options{BaseRef: base, HeadRef: head, JSON: true})
	})
	if !errors.Is(err, ErrDrift) {
		t.Fatalf("JSON output must not disable blocking, got %v", err)
	}

	var report Report
	if jsonErr := json.Unmarshal([]byte(out), &report); jsonErr != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", jsonErr, out)
	}
	if !report.Drift {
		t.Error("report.drift is false for a drifted range")
	}
	if report.Mode != "range" {
		t.Errorf("report.mode = %q, want range", report.Mode)
	}
	if len(report.Results) != 1 || report.Results[0].Doc != "README.md" {
		t.Fatalf("unexpected results: %+v", report.Results)
	}
	if report.Results[0].Status != "outdated" {
		t.Errorf("status = %q, want outdated", report.Results[0].Status)
	}
	if len(report.Results[0].Changes) == 0 {
		t.Error("report does not list the structural changes")
	}
}

// A clean JSON run reports drift false and no unparsed files.
func TestPipelineJSONReportClean(t *testing.T) {
	unsetEnv(t, "DRIFTLOCK_SKIP")

	stub := &verdictServer{verdict: "TRUE. Consistent."}
	srv := stub.start(t)
	dir, base, head := driftFixture(t, srv.URL, true)
	t.Chdir(dir)

	out, err := captureHookStdout(t, func() error {
		return RunWith(context.Background(), Options{BaseRef: base, HeadRef: head, JSON: true})
	})
	if err != nil {
		t.Fatalf("clean JSON run failed: %v", err)
	}

	var report Report
	if jsonErr := json.Unmarshal([]byte(out), &report); jsonErr != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", jsonErr, out)
	}
	if report.Drift {
		t.Error("report.drift is true for a consistent range")
	}
	if len(report.Unparsed) != 0 {
		t.Errorf("unexpected unparsed sources: %v", report.Unparsed)
	}
}
