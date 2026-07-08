package cache

import (
	"path/filepath"
	"testing"
)

func TestKeyStability(t *testing.T) {
	a := Key("model-x", "diff", "doc")
	b := Key("model-x", "diff", "doc")
	if a != b {
		t.Fatal("identical inputs must produce identical keys")
	}
	if a == Key("model-y", "diff", "doc") {
		t.Error("model change must change the key")
	}
	if a == Key("model-x", "diff2", "doc") {
		t.Error("diff change must change the key")
	}
	if a == Key("model-x", "diff", "doc2") {
		t.Error("doc change must change the key")
	}
}

func TestRoundTripPersistence(t *testing.T) {
	dir := t.TempDir()
	c := Load(dir, true)
	k := Key("m", "d", "doc")
	c.Set(k, Entry{OK: false, Explanation: "outdated"})
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}

	// A freshly loaded cache must see the persisted verdict.
	c2 := Load(dir, true)
	e, ok := c2.Get(k)
	if !ok || e.OK || e.Explanation != "outdated" {
		t.Fatalf("round-trip failed: got %+v ok=%v", e, ok)
	}
	if _, err := filepath.Abs(filepath.Join(dir, ".driftlock", "cache.json")); err != nil {
		t.Fatal(err)
	}
}

func TestDisabledCacheIsNoOp(t *testing.T) {
	dir := t.TempDir()
	c := Load(dir, false)
	c.Set("k", Entry{OK: true})
	if _, ok := c.Get("k"); ok {
		t.Error("disabled cache must not store entries")
	}
	if err := c.Save(); err != nil {
		t.Errorf("disabled cache Save should be a no-op, got %v", err)
	}
}
