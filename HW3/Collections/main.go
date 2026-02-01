package main

import (
	"fmt"
	"sync"
)

func main() {
	// Create a plain map
	m := make(map[int]int)
	var wg sync.WaitGroup

	fmt.Println("Starting concurrent map writes...")

	// Spawn 50 goroutines
	for g := 0; g < 50; g++ {
		wg.Add(1)

		go func(gIndex int) {
			defer wg.Done()

			// Each goroutine writes to 1000 unique keys
			for i := 0; i < 1000; i++ {
				// Concurrent write to map
				// Create a unique key for each entry to avoid overlap
				// Even so, concurrent writes can still corrupt the map
				m[gIndex*1000+i] = i
			}
		}(g)
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// Print the map size if program survives
	fmt.Println("Map size:", len(m))
}
