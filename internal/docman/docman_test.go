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

func TestMergeAppendsNewSections(t *testing.T) {
	updated := "### Connect\n\nUpdated Connect docs.\n### Farewell\n\nBrand-new Farewell docs.\n"
	merged := MergeSectionUpdates(sampleDoc, updated)
	if !strings.Contains(merged, "Updated Connect docs.") {
		t.Errorf("existing-section update lost:\n%s", merged)
	}
	if !strings.Contains(merged, "### Farewell") || !strings.Contains(merged, "Brand-new Farewell docs.") {
		t.Errorf("new section was dropped instead of appended:\n%s", merged)
	}
	// New sections go at the end.
	if strings.Index(merged, "### Farewell") < strings.Index(merged, "### Disconnect") {
		t.Errorf("new section should be appended after existing content:\n%s", merged)
	}
}

// LLMs emit headings with trailing whitespace; matching and dedup must be
// whitespace-insensitive (E2E: '## Usage ' vs '## Usage' duplicated a section).
func TestMergeMatchesHeadingsDespiteTrailingWhitespace(t *testing.T) {
	doc := "## Usage \n\nOld usage text.\n"
	updated := "## Usage\n\nNew usage text.\n### Extras \n\nExtra section.\n### Extras\n\nExtra section again.\n"
	merged := MergeSectionUpdates(doc, updated)
	if !strings.Contains(merged, "New usage text.") {
		t.Errorf("trailing-whitespace heading failed to match for replacement:\n%s", merged)
	}
	if strings.Contains(merged, "Old usage text.") {
		t.Errorf("old content survived a matched replacement:\n%s", merged)
	}
	if strings.Count(merged, "### Extras") != 1 {
		t.Errorf("whitespace-variant new sections were not deduplicated:\n%s", merged)
	}
}
