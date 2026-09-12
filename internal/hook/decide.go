package hook

import (
	"strings"

	"github.com/Ksschkw/driftlock/internal/diff"
)

// deterministicVerdict decides a check without a model when the change set
// allows it, and reports whether the decision is decisive.
//
// Two kinds of change are decidable by string matching alone:
//
//   - an ADDED symbol that the documentation never mentions is definitively
//     undocumented;
//   - a REMOVED symbol that the documentation still presents as existing is
//     definitively stale.
//
// A MODIFIED signature is only partly decidable: if the symbol is not mentioned
// at all it is definitively undocumented, but if it is mentioned, matching text
// cannot tell whether the description still fits the new signature — that needs
// the model.
//
// The result is (ok, decisive, reason). When decisive is false the caller must
// fall back to the model; reason is empty in that case.
//
// This is what makes the common case — "a new function was added and nobody
// documented it" — free, instant, and reproducible, instead of a billed API
// call.
func deterministicVerdict(changes []diff.StructuralChange, fullDoc string) (ok bool, decisive bool, reason string) {
	lower := strings.ToLower(fullDoc)

	var drifted []string
	var undecidable []string

	for _, change := range changes {
		name := changeName(change)
		mentioned := name != "" && strings.Contains(lower, strings.ToLower(name))

		switch change.Change {
		case "added":
			if !mentioned {
				drifted = append(drifted, describe(name, "is new and is not mentioned"))
			}
		case "removed":
			if mentioned {
				drifted = append(drifted, describe(name, "was removed but is still documented"))
			}
		default: // modified
			if !mentioned {
				drifted = append(drifted, describe(name, "changed and is not mentioned"))
			} else {
				undecidable = append(undecidable, name)
			}
		}
	}

	if len(drifted) > 0 {
		return false, true, "documentation is definitely out of date: " + strings.Join(drifted, "; ")
	}
	if len(undecidable) > 0 {
		return false, false, ""
	}
	return true, true, "every added and removed symbol matches the documentation"
}

// changeName returns the bare symbol name for a change, falling back to the
// signature text when the parser did not supply one.
func changeName(change diff.StructuralChange) string {
	if change.Name != "" {
		return bareName(change.Name)
	}
	sig := change.NewSig
	if sig == "" {
		sig = change.OldSig
	}
	return extractNameFromSignature(sig)
}

func describe(name, what string) string {
	if name == "" {
		return "an unnamed symbol " + what
	}
	return name + " " + what
}
