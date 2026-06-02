package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// AuditEntry represents a logged commit check.
type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Hash      string    `json:"hash"`
	Files     []string  `json:"files"`
}

// LogHash writes a hash entry to the local audit log.
func LogHash(root string, hash string, files []string) error {
	entry := AuditEntry{
		Timestamp: time.Now().UTC(),
		Hash:      hash,
		Files:     files,
	}
	logDir := filepath.Join(root, ".driftlock")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return err
	}
	logPath := filepath.Join(logDir, "audit.jsonl")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(entry)
}

// ComputeHash returns SHA-256(diff + docContent).
func ComputeHash(diff, docContent string) string {
	h := sha256.New()
	h.Write([]byte(diff))
	h.Write([]byte(docContent))
	return hex.EncodeToString(h.Sum(nil))
}

// ReadLog returns the last n entries from the audit log.
func ReadLog(n int) ([]string, error) {
	// We need a project root to find log, but for simplicity we assume PWD is project root.
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	logPath := filepath.Join(root, ".driftlock", "audit.jsonl")
	// just read whole file and take last n lines
	data, err := os.ReadFile(logPath)
	if err != nil {
		return nil, err
	}
	lines := splitLines(string(data))
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}

func splitLines(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}