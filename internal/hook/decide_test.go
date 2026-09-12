package hook

import (
	"testing"

	"github.com/Ksschkw/driftlock/internal/diff"
)

func added(name string) diff.StructuralChange {
	return diff.StructuralChange{Change: "added", Name: name, NewSig: "func " + name + "()"}
}

func removed(name string) diff.StructuralChange {
	return diff.StructuralChange{Change: "removed", Name: name, OldSig: "func " + name + "()"}
}

func modified(name string) diff.StructuralChange {
	return diff.StructuralChange{Change: "modified", Name: name, OldSig: "func " + name + "()", NewSig: "func " + name + "(x int)"}
}

// An added symbol the docs never mention is decided without a model.
func TestDeterministicAddedSymbolMissing(t *testing.T) {
	ok, decisive, reason := deterministicVerdict([]diff.StructuralChange{added("Repair")}, "# API\n\n## Greet\n")
	if ok || !decisive {
		t.Fatalf("ok=%v decisive=%v, want false/true", ok, decisive)
	}
	if reason == "" {
		t.Error("a decisive verdict must carry a reason")
	}
}

// An added symbol that is mentioned is not proof the docs are correct.
func TestDeterministicAddedSymbolMentionedIsIndeterminate(t *testing.T) {
	ok, decisive, reason := deterministicVerdict([]diff.StructuralChange{added("Greet")}, "# API\n\n## Greet\n")
	if !ok || !decisive {
		t.Fatalf("ok=%v decisive=%v, want true/true", ok, decisive)
	}
	if reason == "" {
		t.Error("a decisive pass must carry a reason")
	}
}

// A removed symbol still presented as existing is decided without a model.
func TestDeterministicRemovedSymbolStillDocumented(t *testing.T) {
	ok, decisive, _ := deterministicVerdict([]diff.StructuralChange{removed("Greet")}, "# API\n\n## Greet\n")
	if ok || !decisive {
		t.Fatalf("ok=%v decisive=%v, want false/true", ok, decisive)
	}
}

// A removed symbol that the docs no longer mention is fine.
func TestDeterministicRemovedSymbolAbsentIsFine(t *testing.T) {
	ok, decisive, _ := deterministicVerdict([]diff.StructuralChange{removed("Greet")}, "# API\n\n## Other\n")
	if !ok || !decisive {
		t.Fatalf("ok=%v decisive=%v, want true/true", ok, decisive)
	}
}

// A modified signature that IS mentioned cannot be judged by text matching.
func TestDeterministicModifiedSymbolIsIndeterminate(t *testing.T) {
	ok, decisive, reason := deterministicVerdict([]diff.StructuralChange{modified("Greet")}, "# API\n\n## Greet\n")
	if decisive {
		t.Fatalf("a mentioned modified signature must not be decided deterministically (ok=%v, reason=%q)", ok, reason)
	}
	if reason != "" {
		t.Errorf("an indeterminate verdict must not carry a reason, got %q", reason)
	}
}

// A modified signature that is not mentioned at all is definitely undocumented.
func TestDeterministicModifiedSymbolMissingIsDrift(t *testing.T) {
	ok, decisive, _ := deterministicVerdict([]diff.StructuralChange{modified("Repair")}, "# API\n\n## Greet\n")
	if ok || !decisive {
		t.Fatalf("ok=%v decisive=%v, want false/true", ok, decisive)
	}
}

// A decisive drift outranks any indeterminate change.
func TestDeterministicDriftOutranksIndeterminate(t *testing.T) {
	changes := []diff.StructuralChange{modified("Greet"), added("Repair")}
	ok, decisive, _ := deterministicVerdict(changes, "# API\n\n## Greet\n")
	if ok || !decisive {
		t.Fatalf("ok=%v decisive=%v, want false/true", ok, decisive)
	}
}

// Matching is case-insensitive, and a qualified name matches its bare form.
func TestDeterministicNameMatching(t *testing.T) {
	ok, decisive, _ := deterministicVerdict([]diff.StructuralChange{added("Repair")}, "# api\n\n`repair(id)` fixes things.\n")
	if !ok || !decisive {
		t.Fatalf("case-insensitive match failed: ok=%v decisive=%v", ok, decisive)
	}

	scoped := diff.StructuralChange{Change: "added", Name: "Widget.build", NewSig: "pub fn build()"}
	ok, decisive, _ = deterministicVerdict([]diff.StructuralChange{scoped}, "## build\n\nBuilds a widget.\n")
	if !ok || !decisive {
		t.Fatalf("qualified name should match its bare form: ok=%v decisive=%v", ok, decisive)
	}
}

// A change with no parser name falls back to the signature text.
func TestDeterministicFallsBackToSignatureName(t *testing.T) {
	change := diff.StructuralChange{Change: "added", NewSig: "func Repair(id int) error"}
	ok, decisive, _ := deterministicVerdict([]diff.StructuralChange{change}, "# API\n")
	if ok || !decisive {
		t.Fatalf("ok=%v decisive=%v, want false/true via the signature fallback", ok, decisive)
	}
}
