package main

import (
	"fmt"
	"sync"
	"time"
)

// SafeMap wraps a map with an RWMutex for concurrent access
type SafeMap struct {
	mu sync.RWMutex
	m  map[int]int
}

// Set acquires a write lock (exclusive), writes to the map, then releases the lock
func (sm *SafeMap) Set(key, value int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.m[key] = value
}

// Get acquires a read lock (shared), reads from the map, then releases the lock
func (sm *SafeMap) Get(key int) (int, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	val, ok := sm.m[key]
	return val, ok
}

// Len acquires a read lock and returns the current map size
func (sm *SafeMap) Len() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
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

	fmt.Printf("Map size [RWMutex]: %d (Expected: 50000)\n", sm.Len())
	fmt.Printf("Time: %v\n", elapsed)
}
