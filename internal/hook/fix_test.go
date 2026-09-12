package hook

import (
	"strings"
	"testing"
)

const fixDoc = "# API\n\n## Connect\n\nConnects to the server.\n"

// An empty model reply must be treated as "no change", not as a successful fix.
func TestMergeFixEmptyReplyIsNoChange(t *testing.T) {
	merged, changed := mergeFix(fixDoc, "")
	if changed {
		t.Errorf("empty reply reported as changed: %q", merged)
	}
	if merged != fixDoc {
		t.Errorf("document modified by an empty reply: %q", merged)
	}
}

// A reply with no headings cannot be stitched back, so it is a no-op rather
// than a silent whole-document replacement.
func TestMergeFixHeadinglessReplyIsNoChange(t *testing.T) {
	merged, changed := mergeFix(fixDoc, "Here is some prose with no heading.\n")
	if changed {
		t.Errorf("headingless reply reported as changed: %q", merged)
	}
	if merged != fixDoc {
		t.Errorf("document modified by a headingless reply: %q", merged)
	}
}

// A reply that updates a matching section is a real change.
func TestMergeFixMatchingSectionChanges(t *testing.T) {
	updated := "## Connect\n\nConnects to the server with a timeout.\n"
	merged, changed := mergeFix(fixDoc, updated)
	if !changed {
		t.Fatal("matching section update reported as no change")
	}
	if merged == fixDoc {
		t.Fatal("matching section update did not alter the document")
	}
	if !strings.Contains(merged, "with a timeout") {
		t.Errorf("updated text missing from merged doc: %q", merged)
	}
}
