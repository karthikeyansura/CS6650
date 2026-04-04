// Package kv: thread-safe in-memory KV store with versioned records.
package kv

import "sync"

type Record struct {
	Value   string `json:"value"`
	Version int64  `json:"version"`
}

// Store wraps an in-memory map with read/write locking.
type Store struct {
	mu   sync.RWMutex
	data map[string]Record
}

// NewStore creates an empty key-value store.
func NewStore() *Store {
	return &Store{data: make(map[string]Record)}
}

// SetIfNewer updates a key only when the incoming version is newer or equal.
func (s *Store) SetIfNewer(key, value string, version int64) Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.data[key]
	if !ok || version >= cur.Version {
		next := Record{Value: value, Version: version}
		s.data[key] = next
		return next
	}
	return cur
}

// Set writes a value and version for a key unconditionally.
func (s *Store) Set(key, value string, version int64) Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := Record{Value: value, Version: version}
	s.data[key] = rec
	return rec
}

// Get returns the stored record for a key, if present.
func (s *Store) Get(key string) (Record, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.data[key]
	return rec, ok
}
