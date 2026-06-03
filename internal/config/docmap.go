package config

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolveDocMapping takes the doc_mapping config, a list of staged files, and the project root,
// and returns a map from documentation file path to the list of source files that map to it.
func ResolveDocMapping(entries []DocMapEntry, stagedFiles []string, root string) (map[string][]string, error) {
	docMap := make(map[string][]string)
	for _, entry := range entries {
		for _, srcGlob := range entry.Sources {
			for _, staged := range stagedFiles {
				match, err := filepath.Match(srcGlob, staged)
				if err != nil {
					return nil, err
				}
				if !match {
					if strings.HasPrefix(staged, strings.TrimSuffix(srcGlob, "**")) {
						match = true
					}
				}
				if match {
					for _, doc := range entry.Docs {
						resolvedDocs, err := ResolveDocPath(doc, root)
						if err != nil {
							return nil, err
						}
						for _, resolved := range resolvedDocs {
							docMap[resolved] = append(docMap[resolved], staged)
						}
					}
				}
			}
		}
	}
	return docMap, nil
}

// ResolveDocPath takes a doc path from config (which may be a directory or a glob) and returns
// the concrete .md file paths relative to the project root.
func ResolveDocPath(doc string, root string) ([]string, error) {
	// If it ends with a separator, treat as a directory and list *.md inside.
	if strings.HasSuffix(doc, string(filepath.Separator)) {
		dir := filepath.Join(root, doc)
		pattern := filepath.Join(dir, "*.md")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		var rel []string
		for _, m := range matches {
			r, err := filepath.Rel(root, m)
			if err == nil {
				rel = append(rel, r)
			}
		}
		return rel, nil
	}

	// Check if it's an existing directory without trailing slash
	fullPath := filepath.Join(root, doc)
	if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
		pattern := filepath.Join(fullPath, "*.md")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		var rel []string
		for _, m := range matches {
			r, err := filepath.Rel(root, m)
			if err == nil {
				rel = append(rel, r)
			}
		}
		return rel, nil
	}

	// Assume it's a direct file path.
	return []string{doc}, nil
}
