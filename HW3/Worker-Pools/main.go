package main

import (
	"fmt"
	"time"
)

// worker represents a single worker in the worker pool.
// - id: identifies the worker
// - jobs: receive-only channel from which the worker gets jobs
// - results: send-only channel where the worker sends results
func worker(id int, jobs <-chan int, results chan<- int) {

	// Continuously read from jobs until the channel is closed.
	for j := range jobs {

		// Job start log
		fmt.Println("worker", id, "started job", j)

		// Simulate a time-intensive task
		time.Sleep(time.Second)

		// Job completion log
		fmt.Println("worker", id, "finished job", j)

		// Send processed result back to the results channel
		results <- j * 2
	}
}

func main() {

	// Number of jobs to be processed
	const numJobs = 5

	// Buffered channel used as a job queue
	jobs := make(chan int, numJobs)

	// Buffered channel used to collect job results
	results := make(chan int, numJobs)

	// Launch a fixed pool of worker goroutines.
	// Workers will block until jobs become available.
	for w := 1; w <= 3; w++ {
		go worker(w, jobs, results)
	}

	// Enqueue jobs for workers to consume
	for j := 1; j <= numJobs; j++ {
		jobs <- j
	}

	// Close jobs to indicate no further work will be submitted
	close(jobs)

	// Wait for all jobs to complete by receiving their results.
	// This prevents main from exiting early.
	for a := 1; a <= numJobs; a++ {
		<-results
	}
}
