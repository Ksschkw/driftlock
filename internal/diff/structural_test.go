package diff

import "testing"

func changeByKind(changes []StructuralChange, kind string) []StructuralChange {
	var out []StructuralChange
	for _, c := range changes {
		if c.Change == kind {
			out = append(out, c)
		}
	}
	return out
}

func TestAddedRemovedModified(t *testing.T) {
	oldSrc := `package p
func Kept(a int) {}
func Removed(b int) {}
func Changed(c int) {}
`
	newSrc := `package p
func Kept(a int) {}
func Changed(c int, d string) {}
func Added(e int) {}
`
	changes := ExtractStructuralChanges("p.go", oldSrc, newSrc)

	if got := changeByKind(changes, "added"); len(got) != 1 || got[0].NewSig == "" {
		t.Errorf("expected 1 added, got %v", got)
	}
	if got := changeByKind(changes, "removed"); len(got) != 1 {
		t.Errorf("expected 1 removed, got %v", got)
	}
	if got := changeByKind(changes, "modified"); len(got) != 1 {
		t.Errorf("expected 1 modified, got %v", got)
	}
}

// A body-only edit changes no signatures and must produce zero structural
// changes — this silence is the whole point of Driftlock.
func TestBodyOnlyEditIsSilent(t *testing.T) {
	oldSrc := `package p
func F(a int) int { return a }
`
	newSrc := `package p
func F(a int) int { return a * 2 }
`
	if changes := ExtractStructuralChanges("p.go", oldSrc, newSrc); len(changes) != 0 {
		t.Errorf("body-only edit should be silent, got %v", changes)
	}
}

func TestRenameIsRemoveAndAdd(t *testing.T) {
	oldSrc := "package p\nfunc OldName(a int) {}\n"
	newSrc := "package p\nfunc NewName(a int) {}\n"
	changes := ExtractStructuralChanges("p.go", oldSrc, newSrc)
	if len(changeByKind(changes, "added")) != 1 || len(changeByKind(changes, "removed")) != 1 {
		t.Errorf("rename should be one add + one remove, got %v", changes)
	}
}
