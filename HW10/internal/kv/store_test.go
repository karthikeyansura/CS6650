// Unit tests for the in-memory KV store.
package kv

import (
	"sync"
	"testing"
)

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

// TestGetMissingKey verifies that reading a non-existent key returns ok=false.
func TestGetMissingKey(t *testing.T) {
	s := NewStore()
	_, ok := s.Get("nonexistent")
	if ok {
		t.Fatal("expected missing key to return ok=false")
	}
}

// TestSetOverwritesSameKey verifies Set overwrites unconditionally, even
// with a lower version (contrasts with SetIfNewer).
func TestSetOverwritesSameKey(t *testing.T) {
	s := NewStore()
	s.Set("k", "v1", 10)
	s.Set("k", "v2", 20)
	rec, ok := s.Get("k")
	if !ok || rec.Value != "v2" || rec.Version != 20 {
		t.Fatalf("expected latest write: %#v", rec)
	}
	s.Set("k", "v3", 5)
	rec, _ = s.Get("k")
	if rec.Value != "v3" || rec.Version != 5 {
		t.Fatalf("Set should overwrite unconditionally even with lower version: %#v", rec)
	}
}

// TestSetEmptyValueAllowed verifies the assignment requirement that empty
// string is a valid value.
func TestSetEmptyValueAllowed(t *testing.T) {
	s := NewStore()
	s.Set("k", "", 1)
	rec, ok := s.Get("k")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if rec.Value != "" || rec.Version != 1 {
		t.Fatalf("expected empty value at version 1: %#v", rec)
	}
}

// TestSetIfNewerOnNewKey verifies that SetIfNewer writes successfully when
// the key does not yet exist.
func TestSetIfNewerOnNewKey(t *testing.T) {
	s := NewStore()
	rec := s.SetIfNewer("fresh", "val", 42)
	if rec.Value != "val" || rec.Version != 42 {
		t.Fatalf("expected new key to be written: %#v", rec)
	}
	stored, ok := s.Get("fresh")
	if !ok || stored.Value != "val" || stored.Version != 42 {
		t.Fatalf("stored record mismatch: %#v", stored)
	}
}

// TestSetIfNewerEqualVersionUpdates verifies the >= condition in
// SetIfNewer: an equal version should update the value (idempotent replay).
func TestSetIfNewerEqualVersionUpdates(t *testing.T) {
	s := NewStore()
	s.Set("k", "original", 10)
	rec := s.SetIfNewer("k", "updated", 10)
	if rec.Value != "updated" || rec.Version != 10 {
		t.Fatalf("equal version should update value: %#v", rec)
	}
}

// TestConcurrentSetSafety verifies that concurrent Set calls do not corrupt
// the store or panic under the race detector.
func TestConcurrentSetSafety(t *testing.T) {
	s := NewStore()
	var wg sync.WaitGroup
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				s.Set("shared", "v", int64(id*100+i))
			}
		}(g)
	}
	wg.Wait()

	rec, ok := s.Get("shared")
	if !ok {
		t.Fatal("shared key missing after concurrent writes")
	}
	if rec.Value != "v" {
		t.Fatalf("unexpected value after concurrent writes: %q", rec.Value)
	}
}

// TestConcurrentSetIfNewerHighestWins verifies that after concurrent
// SetIfNewer calls with unique versions 0..999, the stored version is
// always 999 (the global maximum).
func TestConcurrentSetIfNewerHighestWins(t *testing.T) {
	s := NewStore()
	var wg sync.WaitGroup
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				s.SetIfNewer("shared", "v", int64(id*100+i))
			}
		}(g)
	}
	wg.Wait()

	rec, ok := s.Get("shared")
	if !ok {
		t.Fatal("shared key missing")
	}
	if rec.Version != 999 {
		t.Fatalf("expected highest version 999, got %d", rec.Version)
	}
}
