package diff

import (
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

// ExtractStructuralChanges compares old and new file content and returns a list of structural changes.
func ExtractStructuralChanges(filePath string, oldContent, newContent string) []StructuralChange {
	oldSigs := parser.ExtractSignatures(filePath, oldContent)
	newSigs := parser.ExtractSignatures(filePath, newContent)

	// Map signatures for easy lookup (keyed by name)
	oldMap := make(map[string]string)
	for _, s := range oldSigs {
		oldMap[s.Name] = s.Signature
	}
	newMap := make(map[string]string)
	for _, s := range newSigs {
		newMap[s.Name] = s.Signature
	}

	var changes []StructuralChange

	// Detected removed/added/modified
	for name, oldSig := range oldMap {
		if newSig, ok := newMap[name]; !ok {
			changes = append(changes, StructuralChange{
				FilePath: filePath,
				OldSig:   oldSig,
				Change:   "removed",
			})
		} else if oldSig != newSig {
			changes = append(changes, StructuralChange{
				FilePath: filePath,
				OldSig:   oldSig,
				NewSig:   newSig,
				Change:   "modified",
			})
		}
	}
	for name, newSig := range newMap {
		if _, ok := oldMap[name]; !ok {
			changes = append(changes, StructuralChange{
				FilePath: filePath,
				NewSig:   newSig,
				Change:   "added",
			})
		}
	}

	return changes
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