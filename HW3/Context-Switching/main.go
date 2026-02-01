package main

import (
	"fmt"
	"runtime"
	"time"
)

const roundTrips = 1000000

// measureSwitchTime measures total time for channel-based round trip
// Each iteration results in two context switches
func measureSwitchTime(maxProcs int) time.Duration {
	runtime.GOMAXPROCS(maxProcs)

	// Unbuffered channels for synchronous handoff
	ch1 := make(chan struct{})
	ch2 := make(chan struct{})
	done := make(chan struct{})

	// Goroutine 1: receives from ch1, sends to ch2
	go func() {
		for i := 0; i < roundTrips; i++ {
			<-ch1
			ch2 <- struct{}{}
		}
		done <- struct{}{}
	}()

	// Goroutine 2: sends to ch1, receives from ch2
	go func() {
		for i := 0; i < roundTrips; i++ {
			ch1 <- struct{}{}
			<-ch2
		}
		done <- struct{}{}
	}()

	start := time.Now()

	// Wait for both goroutines to finish
	<-done
	<-done

	return time.Since(start)
}

func main() {
	fmt.Printf("Unbuffered channel round trips: %d (= %d context switches)\n\n", roundTrips, roundTrips*2)

	// Single OS thread (serial execution)
	fmt.Println("Single-threaded execution:")
	for i := 1; i <= 3; i++ {
		elapsed := measureSwitchTime(1)
		avgSwitch := elapsed / time.Duration(roundTrips*2)
		fmt.Printf("Iteration %d:\n", i)
		fmt.Printf("    Total time               = %v\n", elapsed)
		fmt.Printf("    Avg context switch time  = %v\n", avgSwitch)
	}

	// Multiple OS threads (parallel execution)
	numCPU := runtime.NumCPU()
	fmt.Printf("\nMulti-threaded execution (%d threads):\n", numCPU)
	for i := 1; i <= 3; i++ {
		elapsed := measureSwitchTime(numCPU)
		avgSwitch := elapsed / time.Duration(roundTrips*2)
		fmt.Printf("Iteration %d:\n", i)
		fmt.Printf("    Total time               = %v\n", elapsed)
		fmt.Printf("    Avg context switch time  = %v\n", avgSwitch)
	}
}
