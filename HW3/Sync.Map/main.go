package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {
	var m sync.Map
	var wg sync.WaitGroup

	start := time.Now()

	// Spawn 50 goroutines
	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func(gIndex int) {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				m.Store(gIndex*1000+i, i)
			}
		}(g)
	}

	// Wait for all goroutines to finish
	wg.Wait()
	elapsed := time.Since(start)

	// Count entries since sync.Map does not provide Len()
	count := 0
	m.Range(func(key, value interface{}) bool {
		count++
		return true
	})

	fmt.Printf("Map size [Sync.Map]: %d (Expected: 50000)\n", count)
	fmt.Printf("Time: %v\n", elapsed)
}
