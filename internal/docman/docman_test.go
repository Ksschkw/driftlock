package docman

import "strings"

import "testing"

const sampleDoc = `# Title

Intro text.

## Installation

Run the installer.

## API

### Connect

The Connect function opens a session.

### Disconnect

The Disconnect function closes a session.
`

func TestExtractRelevantSectionsPicksDeepest(t *testing.T) {
	out, note := ExtractRelevantSections(sampleDoc, []string{"Connect"})
	if !strings.Contains(out, "### Connect") {
		t.Errorf("expected the Connect section, got:\n%s", out)
	}
	// The Installation section is unrelated and must be excluded.
	if strings.Contains(out, "Run the installer") {
		t.Errorf("unrelated section leaked into chunk:\n%s", out)
	}
	// The full document must never be returned wholesale.
	if strings.Contains(out, "Intro text.") {
		t.Errorf("ancestor/intro content should be excluded:\n%s", out)
	}
	if note != "" {
		t.Errorf("documented symbol should produce no note, got %q", note)
	}
}

func TestExtractReportsUndocumentedSymbols(t *testing.T) {
	out, note := ExtractRelevantSections(sampleDoc, []string{"BrandNewThing"})
	// The note must carry the symbol, and it must NOT be embedded in the
	// section content (that caused the auto-fix to write Driftlock metadata
	// verbatim into user documentation).
	if !strings.Contains(note, "BrandNewThing") {
		t.Errorf("expected undocumented symbol in note, got note=%q", note)
	}
	if strings.Contains(out, "BrandNewThing") {
		t.Errorf("note text leaked into section content:\n%s", out)
	}
}

func TestMergeSectionUpdatesReplacesByHeading(t *testing.T) {
	updated := "### Connect\n\nThe Connect function now takes a timeout argument.\n"
	merged := MergeSectionUpdates(sampleDoc, updated)

	if !strings.Contains(merged, "now takes a timeout argument") {
		t.Errorf("update was not merged:\n%s", merged)
	}
	// Unrelated sections must be preserved verbatim.
	if !strings.Contains(merged, "The Disconnect function closes a session.") {
		t.Errorf("merge clobbered an unrelated section:\n%s", merged)
	}
	if !strings.Contains(merged, "Run the installer.") {
		t.Errorf("merge dropped the Installation section:\n%s", merged)
	}
}

func TestMergeIsNoOpForEmptyUpdate(t *testing.T) {
	if got := MergeSectionUpdates(sampleDoc, ""); got != sampleDoc {
		t.Error("empty update should return the document unchanged")
	}
}
