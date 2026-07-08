// Package cache provides a persistent, content-addressed store for LLM check
// verdicts. Because a documentation check is a pure function of (model, diff,
// doc), an identical check never needs to hit the LLM twice. This directly
// bounds token spend across commit amends, rebases, CI re-runs, and repeated
// local commits — the LLM is the dominant cost in Driftlock, so caching it is
// the single highest-leverage economic optimization.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Entry is a cached check verdict.
type Entry struct {
	OK          bool   `json:"ok"`
	Explanation string `json:"explanation"`
}

// Cache is a JSON-backed key→verdict map. It is safe for concurrent use.
type Cache struct {
	mu      sync.Mutex
	path    string
	entries map[string]Entry
	dirty   bool
	enabled bool
}

// Key derives a stable cache key from the model name and the exact check
// inputs. Any change to the diff, the documentation, or the model produces a
// different key, so stale verdicts are never served.
func Key(model, diff, doc string) string {
	h := sha256.New()
	h.Write([]byte(model))
	h.Write([]byte{0})
	h.Write([]byte(diff))
	h.Write([]byte{0})
	h.Write([]byte(doc))
	return hex.EncodeToString(h.Sum(nil))
}

// Load opens (or initializes) the cache for a project root. When enabled is
// false, a no-op cache is returned so callers need no conditional logic.
func Load(root string, enabled bool) *Cache {
	c := &Cache{
		path:    filepath.Join(root, ".driftlock", "cache.json"),
		entries: map[string]Entry{},
		enabled: enabled,
	}
	if !enabled {
		return c
	}
	data, err := os.ReadFile(c.path)
	if err == nil {
		_ = json.Unmarshal(data, &c.entries) // corrupt cache is simply ignored
	}
	return c
}

// Get returns a cached verdict and whether it was present.
func (c *Cache) Get(key string) (Entry, bool) {
	if c == nil || !c.enabled {
		return Entry{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	return e, ok
}

// Set records a verdict in memory and marks the cache dirty.
func (c *Cache) Set(key string, e Entry) {
	if c == nil || !c.enabled {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = e
	c.dirty = true
}

// Save persists the cache to disk if it changed. It is a no-op for a disabled
// or unmodified cache.
func (c *Cache) Save() error {
	if c == nil || !c.enabled || !c.dirty {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c.entries, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(c.path, data, 0o644); err != nil {
		return err
	}
	c.dirty = false
	return nil
}
