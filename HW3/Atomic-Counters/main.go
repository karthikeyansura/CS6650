package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

func main() {
	// Thread-Safe Counter using atomic operations
	var atomicOps atomic.Uint64

	// Unsafe Counter using a regular integer
	var standardOps int

	var wg sync.WaitGroup

	// Spawn 50 goroutines
	for i := 0; i < 50; i++ {
		wg.Add(1) // Increment WaitGroup counter for each goroutine

		go func() {
			defer wg.Done() // Decrement WaitGroup counter when goroutine finishes

			// Each goroutine increments both counters 1000 times
			for c := 0; c < 1000; c++ {

				// Atomic operation ensures correct increment even with multiple goroutines
				atomicOps.Add(1)

				// Standard increment is prone to race conditions
				standardOps++
			}
		}()
	}

	// Wait until all goroutines have finished
	wg.Wait()

	fmt.Printf("Atomic Counter:   %d (Expected: 50000)\n", atomicOps.Load())
	fmt.Printf("Standard Counter: %d (Expected: 50000)\n", standardOps)
}
