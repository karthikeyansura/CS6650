// Basic unit tests for the in-memory KV store.
package kv

import "testing"

// TestSetAndGet verifies basic write/read behavior.
func TestSetAndGet(t *testing.T) {
	s := NewStore()
	s.Set("a", "v1", 1)
	rec, ok := s.Get("a")
	if !ok {
		t.Fatalf("expected key to exist")
	}
	if rec.Value != "v1" || rec.Version != 1 {
		t.Fatalf("unexpected record: %#v", rec)
	}
}

// TestSetIfNewer verifies stale versions are ignored and newer versions win.
func TestSetIfNewer(t *testing.T) {
	s := NewStore()
	s.Set("a", "v1", 10)

	rec := s.SetIfNewer("a", "old", 5)
	if rec.Value != "v1" || rec.Version != 10 {
		t.Fatalf("expected older write to be ignored: %#v", rec)
	}

	rec = s.SetIfNewer("a", "new", 11)
	if rec.Value != "new" || rec.Version != 11 {
		t.Fatalf("expected newer write to win: %#v", rec)
	}
}
