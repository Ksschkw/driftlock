package config

import (
	"path/filepath"
	"strings"
)

// ResolveDocMapping takes the doc_mapping config and a list of staged files,
// and returns a map from documentation file path to the list of source files that map to it.
func ResolveDocMapping(entries []DocMapEntry, stagedFiles []string) (map[string][]string, error) {
	docMap := make(map[string][]string)
	for _, entry := range entries {
		for _, srcGlob := range entry.Sources {
			for _, staged := range stagedFiles {
				match, err := filepath.Match(srcGlob, staged)
				if err != nil {
					return nil, err
				}
				if !match {
					// Check if the staged file is within the glob's directory tree.
					// For example, "src/**" should match "src/sub/file.go".
					if strings.HasPrefix(staged, strings.TrimSuffix(srcGlob, "**")) {
						match = true
					}
				}
				if match {
					for _, doc := range entry.Docs {
						// docs can be directories; for simplicity we treat them as exact file paths.
						// In a future version, we'll expand directories to all *.md files inside.
						docMap[doc] = append(docMap[doc], staged)
					}
				}
			}
		}
	}
	return docMap, nil
}