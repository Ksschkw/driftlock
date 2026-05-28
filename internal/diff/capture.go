package diff

import (
	"fmt"
	"regexp"
	"strings"
)

// FileDiff represents the changes to a single file.
type FileDiff struct {
	FilePath string
	Hunks    []Hunk
}

// Hunk represents a single hunk of changes.
type Hunk struct {
	Header   string
	OldLines string
	NewLines string
	OldStart int
	NewStart int
}

var (
	fileHeaderRe = regexp.MustCompile(`^diff --git a/(.+) b/(.+)$`)
	hunkHeaderRe = regexp.MustCompile(`^@@ -(\d+),?\d* \+(\d+),?\d* @@(.*)`)
)

// ParseDiff parses a unified diff string into a slice of FileDiff.
func ParseDiff(raw string) []FileDiff {
	var files []FileDiff
	var current *FileDiff
	lines := strings.Split(raw, "\n")

	for _, line := range lines {
		if matches := fileHeaderRe.FindStringSubmatch(line); matches != nil {
			if current != nil {
				files = append(files, *current)
			}
			current = &FileDiff{FilePath: matches[2]}
			continue
		}
		if current == nil {
			continue
		}
		if matches := hunkHeaderRe.FindStringSubmatch(line); matches != nil {
			var oldStart, newStart int
			fmt.Sscanf(matches[1], "%d", &oldStart)
			fmt.Sscanf(matches[2], "%d", &newStart)
			hunk := Hunk{
				Header:   matches[0],
				OldStart: oldStart,
				NewStart: newStart,
			}
			current.Hunks = append(current.Hunks, hunk)
			continue
		}
		if len(current.Hunks) == 0 {
			continue
		}
		last := &current.Hunks[len(current.Hunks)-1]
		if strings.HasPrefix(line, "+") {
			last.NewLines += line + "\n"
		} else if strings.HasPrefix(line, "-") {
			last.OldLines += line + "\n"
		} else {
			last.OldLines += line + "\n"
			last.NewLines += line + "\n"
		}
	}
	if current != nil {
		files = append(files, *current)
	}
	return files
}