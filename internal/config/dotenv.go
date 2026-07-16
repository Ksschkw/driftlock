package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// LoadDotEnv reads a .env file from dir and populates the process environment
// with any variables it defines that are not already set. Real environment
// variables always win, so a value exported in the shell overrides the file.
//
// This exists so that api_key = "${DRIFTLOCK_API_KEY}" in .driftlock.toml
// resolves from a local .env file — letting teams commit the config (policy)
// while every developer keeps their secret out of version control.
//
// The parser is intentionally small: it supports KEY=VALUE lines, blank lines,
// "#" comments, an optional leading "export ", and single/double quoted values.
// A missing .env file is not an error.
func LoadDotEnv(dir string) {
	path := filepath.Join(dir, ".env")
	f, err := os.Open(path)
	if err != nil {
		return // no .env is the common case, never an error
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		val = strings.TrimSpace(val)
		// Strip a matching pair of surrounding quotes.
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') ||
				(val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		// Never clobber a variable the developer set in their real shell.
		if _, present := os.LookupEnv(key); !present {
			_ = os.Setenv(key, val)
		}
	}
}
