package diff

import (
	"sort"
	"strings"

	"github.com/Ksschkw/driftlock/internal/parser"
)

// StructuralChange describes a single structural change (function signature change).
type StructuralChange struct {
	FilePath string
	OldSig   string
	NewSig   string
	Change   string // "added", "removed", "modified"
}

// ExtractStructuralChanges compares old and new file content and returns a list
// of structural changes.
//
// Signatures are grouped by name and then compared as multisets within each
// group. The previous implementation stored one signature per name in a map, so
// a name declared more than once per file — Go methods on different receiver
// types, Java/C# overloads — silently overwrote itself. Deleting one overload
// produced NO change, and deleting `A.Close` while `B.Close` survived was
// reported as a bogus `modified` comparing the two unrelated methods.
//
// Within a name group, identical signatures are paired off first. What remains
// is a genuine removal/addition, except that exactly one old and one new
// leftover is reported as a single `modified` (the common edit case).
func ExtractStructuralChanges(filePath string, oldContent, newContent string) []StructuralChange {
	oldSigs := parser.ExtractSignatures(filePath, oldContent)
	newSigs := parser.ExtractSignatures(filePath, newContent)

	oldByName := make(map[string][]string)
	for _, s := range oldSigs {
		oldByName[s.Name] = append(oldByName[s.Name], s.Signature)
	}
	newByName := make(map[string][]string)
	for _, s := range newSigs {
		newByName[s.Name] = append(newByName[s.Name], s.Signature)
	}

	names := make([]string, 0, len(oldByName)+len(newByName))
	seen := make(map[string]bool, len(oldByName)+len(newByName))
	for name := range oldByName {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	for name := range newByName {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	// Deterministic output: map iteration order is random, and the formatted
	// diff is part of the LLM cache key, so an unstable order would defeat the
	// cache and make runs irreproducible.
	sort.Strings(names)

	var changes []StructuralChange
	for _, name := range names {
		oldLeft, newLeft := unpair(oldByName[name], newByName[name])
		if len(oldLeft) == 1 && len(newLeft) == 1 {
			changes = append(changes, StructuralChange{
				FilePath: filePath,
				OldSig:   oldLeft[0],
				NewSig:   newLeft[0],
				Change:   "modified",
			})
			continue
		}
		for _, oldSig := range oldLeft {
			changes = append(changes, StructuralChange{
				FilePath: filePath,
				OldSig:   oldSig,
				Change:   "removed",
			})
		}
		for _, newSig := range newLeft {
			changes = append(changes, StructuralChange{
				FilePath: filePath,
				NewSig:   newSig,
				Change:   "added",
			})
		}
	}
	return changes
}

// unpair removes signatures that appear identically on both sides, returning
// the leftovers from each side in their original order. It is a multiset
// difference, so two identical overloads are handled correctly.
func unpair(olds, news []string) (oldLeft, newLeft []string) {
	oldCount := make(map[string]int, len(olds))
	for _, o := range olds {
		oldCount[o]++
	}
	for _, n := range news {
		if oldCount[n] > 0 {
			oldCount[n]--
			continue
		}
		newLeft = append(newLeft, n)
	}

	newCount := make(map[string]int, len(news))
	for _, n := range news {
		newCount[n]++
	}
	for _, o := range olds {
		if newCount[o] > 0 {
			newCount[o]--
			continue
		}
		oldLeft = append(oldLeft, o)
	}
	return oldLeft, newLeft
}

// FormatStructuralChanges returns a human-readable string summarizing structural changes.
func FormatStructuralChanges(changes []StructuralChange) string {
	var sb strings.Builder
	for _, c := range changes {
		sb.WriteString(c.FilePath + ":\n")
		switch c.Change {
		case "added":
			sb.WriteString("  + added: " + c.NewSig + "\n")
		case "removed":
			sb.WriteString("  - removed: " + c.OldSig + "\n")
		case "modified":
			sb.WriteString("  ~ old: " + c.OldSig + "\n")
			sb.WriteString("  ~ new: " + c.NewSig + "\n")
		}
	}
	return sb.String()
}
