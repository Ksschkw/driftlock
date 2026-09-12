package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// parseDotEnvValue extracts the value from the right-hand side of a .env line.
//
// It handles the two forms a real .env uses:
//
//	KEY="value with # hash"   quoted: the closing quote ends the value
//	KEY=value # a comment      unquoted: whitespace before '#' starts a comment
//
// A '#' immediately preceded by a non-space character is part of the value
// (`abc#def` is kept), and a line whose value is only a comment becomes empty.
// Without comment handling, copying a documented example such as
// `DRIFTLOCK_DEBUG=1 # enable debug` silently set the variable to the whole
// sentence — and for DRIFTLOCK_SKIP=true it would fail to match "true".
func parseDotEnvValue(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if raw[0] == '"' || raw[0] == '\'' {
		quote := raw[0]
		if end := strings.IndexByte(raw[1:], quote); end >= 0 {
			return raw[1 : 1+end]
		}
		return raw[1:] // unterminated quote: take the remainder
	}
	if i := strings.Index(raw, " #"); i >= 0 {
		raw = raw[:i]
	}
	if strings.HasPrefix(raw, "#") {
		return ""
	}
	return strings.TrimSpace(raw)
}

// LoadDotEnv reads a .env file from dir and populates the process environment
// with any variables it defines that are not already set. Real environment
// variables always win, so a value exported in the shell overrides the file.
//
// This exists so that api_key = "${DRIFTLOCK_API_KEY}" in .driftlock.toml
// resolves from a local .env file — letting teams commit the config (policy)
// while every developer keeps their secret out of version control.
//
// The parser is intentionally small: it supports KEY=VALUE lines, blank lines,
// full-line "#" comments, trailing "#" comments after a value, an optional
// leading "export ", and single/double quoted values. A missing .env file is
// not an error.
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
		val = parseDotEnvValue(val)
		// Never clobber a variable the developer set in their real shell.
		if _, present := os.LookupEnv(key); !present {
			_ = os.Setenv(key, val)
		}
	}
}
