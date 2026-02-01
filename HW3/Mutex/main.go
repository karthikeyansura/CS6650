package main

import (
	"fmt"
	"sync"
	"time"
)

// SafeMap wraps a map with a Mutex for concurrent access
type SafeMap struct {
	mu sync.Mutex
	m  map[int]int
}

// Set acquires the lock, writes to the map, then releases the lock
func (sm *SafeMap) Set(key, value int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.m[key] = value
}

// Get acquires the lock, reads from the map, then releases the lock
func (sm *SafeMap) Get(key int) (int, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	val, ok := sm.m[key]
	return val, ok
}

// Len acquires the lock and returns the current map size
func (sm *SafeMap) Len() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return len(sm.m)
}

func main() {
	sm := &SafeMap{m: make(map[int]int)}
	var wg sync.WaitGroup

	start := time.Now()

	// Spawn 50 goroutines
	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func(gIndex int) {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				sm.Set(gIndex*1000+i, i)
			}
		}(g)
	}

	// Wait for all goroutines to finish
	wg.Wait()
	elapsed := time.Since(start)

	fmt.Printf("Map size [Mutex]: %d (Expected: 50000)\n", sm.Len())
	fmt.Printf("Time: %v\n", elapsed)
}
