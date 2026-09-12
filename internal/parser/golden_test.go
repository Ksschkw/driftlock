package parser_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Ksschkw/driftlock/internal/parser"
)

var update = flag.Bool("update", false, "rewrite the golden signature files from current parser output")

// TestGoldenCorpus runs every file in testdata through the extractor and
// compares the result with a committed golden file. It is the safety net for
// parser changes: a new pattern that regresses one language shows up as a diff
// here rather than as a silent false negative in production.
//
// A corpus file with no golden fails loudly, so adding a language sample
// without recording its expected output is impossible.
func TestGoldenCorpus(t *testing.T) {
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	checked := 0
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".golden") {
			continue
		}
		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			got := formatGolden(parser.ExtractSignatures(name, string(src)))
			goldenPath := filepath.Join("testdata", name+".golden")

			if *update {
				if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}

			wantBytes, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("missing golden file %s (run: go test ./internal/parser -update)", goldenPath)
			}
			want := string(wantBytes)
			if got != want {
				t.Errorf("extracted signatures changed for %s\n--- want ---\n%s\n--- got ---\n%s", name, want, got)
			}
		})
		checked++
	}
	if checked == 0 {
		t.Fatal("no corpus files found in testdata")
	}
}

// formatGolden renders signatures one per line as "Name\tSignature", sorted so
// the file is stable regardless of pattern iteration order.
func formatGolden(sigs []parser.Signature) string {
	lines := make([]string, 0, len(sigs))
	for _, s := range sigs {
		lines = append(lines, fmt.Sprintf("%s\t%s", s.Name, s.Signature))
	}
	sort.Strings(lines)
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}
