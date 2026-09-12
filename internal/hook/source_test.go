package hook

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// fakeSource is an in-memory changeSource. It lets pipeline behaviour be tested
// without a repository, so cases that are awkward to build with real commits —
// a deleted file, an empty document, a source that yields no signatures — are
// expressed directly.
type fakeSource struct {
	files []string
	old   map[string]string
	new   map[string]string
	docs  map[string]string
}

func (f fakeSource) Changes(Options, bool) ([]string, func(string) string, func(string) string, error) {
	return f.files,
		func(p string) string { return f.old[p] },
		func(p string) string { return f.new[p] },
		nil
}

func (f fakeSource) DocContent(_, docPath, _ string, _ bool) (string, error) {
	content, ok := f.docs[docPath]
	if !ok {
		return "", errors.New("doc not found: " + docPath)
	}
	return content, nil
}

// fakeDocConfig prepares a directory for a fake-source run: a config the
// pipeline can load and a .git marker so FindProjectRoot resolves (no actual
// repository is created or used).
func fakeDocConfig(t *testing.T, dir, endpoint string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeDriftlockConfig(t, dir, endpoint, false, true)
}

// The whole pipeline runs against an in-memory source: no git repository at all.
func TestPipelineWithInjectedSource(t *testing.T) {
	unsetEnv(t, "DRIFTLOCK_SKIP")

	stub := &verdictServer{verdict: "FALSE. Drifted."}
	srv := stub.start(t)

	root := t.TempDir()
	write(t, filepath.Join(root, "README.md"), "# API\n\n## Greet\n\n`Greet(name)` greets.\n")
	fakeDocConfig(t, root, srv.URL)
	t.Chdir(root)

	src := fakeSource{
		files: []string{"src/greet.go"},
		old:   map[string]string{"src/greet.go": "package src\n\nfunc Greet(name string) string { return name }\n"},
		new:   map[string]string{"src/greet.go": "package src\n\nfunc Greet(name string, excited bool) string { return name }\n"},
		docs:  map[string]string{"README.md": "# API\n\n## Greet\n\n`Greet(name)` greets.\n"},
	}

	err := RunWith(context.Background(), Options{source: src})
	if !errors.Is(err, ErrDrift) {
		t.Fatalf("expected ErrDrift from the injected source, got %v", err)
	}
	if len(stub.prompts()) == 0 {
		t.Fatal("the model was never consulted")
	}
}

// A deleted source has no structural change to document and must be skipped.
func TestPipelineWithInjectedSourceSkipsDeletion(t *testing.T) {
	unsetEnv(t, "DRIFTLOCK_SKIP")

	stub := &verdictServer{verdict: "FALSE. Should never be asked."}
	srv := stub.start(t)

	root := t.TempDir()
	write(t, filepath.Join(root, "README.md"), "# API\n")
	fakeDocConfig(t, root, srv.URL)
	t.Chdir(root)

	src := fakeSource{
		files: []string{"src/gone.go"},
		old:   map[string]string{"src/gone.go": "package src\n\nfunc Gone() {}\n"},
		new:   map[string]string{"src/gone.go": ""},
		docs:  map[string]string{"README.md": "# API\n"},
	}

	if err := RunWith(context.Background(), Options{source: src}); err != nil {
		t.Fatalf("a deleted source should not fail the run, got %v", err)
	}
	if n := len(stub.prompts()); n != 0 {
		t.Errorf("a deleted source consulted the model %d time(s)", n)
	}
}

// An unparsable mapped source must be reported, not silently ignored.
func TestPipelineWithInjectedSourceReportsUnparsed(t *testing.T) {
	unsetEnv(t, "DRIFTLOCK_SKIP")

	stub := &verdictServer{verdict: "TRUE."}
	srv := stub.start(t)

	root := t.TempDir()
	write(t, filepath.Join(root, "README.md"), "# API\n")
	fakeDocConfig(t, root, srv.URL)
	t.Chdir(root)

	// Content that looks like code but yields no signatures.
	odd := "package src\n\nvalue := compute(1)\nother := compute(2)\n"
	src := fakeSource{
		files: []string{"src/odd.lang"},
		old:   map[string]string{"src/odd.lang": ""},
		new:   map[string]string{"src/odd.lang": odd},
		docs:  map[string]string{"README.md": "# API\n"},
	}

	out, err := captureHookStdout(t, func() error {
		return RunWith(context.Background(), Options{source: src, JSON: true})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var report Report
	if jsonErr := json.Unmarshal([]byte(out), &report); jsonErr != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", jsonErr, out)
	}
	found := false
	for _, u := range report.Unparsed {
		if u == "src/odd.lang" {
			found = true
		}
	}
	if !found {
		t.Errorf("unparsed source not reported in JSON: %v", report.Unparsed)
	}
}
