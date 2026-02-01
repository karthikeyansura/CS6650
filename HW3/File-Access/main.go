package main

import (
	"bufio"
	"fmt"
	"os"
	"time"
)

const (
	iterations  = 100000
	lineContent = "This is a line of text for testing I/O performance.\n"
)

// writeUnbuffered performs a direct file write on every iteration
// Each write results in a syscall to the OS
func writeUnbuffered(filename string) time.Duration {
	f, err := os.Create(filename)
	if err != nil {
		panic(err)
	}

	start := time.Now()
	for i := 0; i < iterations; i++ {
		f.Write([]byte(lineContent))
	}
	f.Close()

	return time.Since(start)
}

// writeBuffered wraps the file with a buffered writer
// Writes accumulate in memory and flush once at the end
func writeBuffered(filename string) time.Duration {
	f, err := os.Create(filename)
	if err != nil {
		panic(err)
	}

	w := bufio.NewWriter(f)

	start := time.Now()
	for i := 0; i < iterations; i++ {
		w.WriteString(lineContent)
	}
	w.Flush() // Flush remaining buffered data to disk
	f.Close()

	return time.Since(start)
}

func main() {
	fmt.Printf("Writing %d lines (%d bytes each)\n", iterations, len(lineContent))
	fmt.Printf("Total data: %.2f MB\n\n", float64(iterations*len(lineContent))/(1024*1024))

	fmt.Println("Unbuffered writes:")
	for i := 1; i <= 3; i++ {
		elapsed := writeUnbuffered("unbuffered_output.txt")
		fmt.Printf("Iteration %d: %v\n", i, elapsed)
	}

	fmt.Println("\nBuffered writes:")
	for i := 1; i <= 3; i++ {
		elapsed := writeBuffered("buffered_output.txt")
		fmt.Printf("Iteration %d: %v\n", i, elapsed)
	}

	// Cleanup
	os.Remove("unbuffered_output.txt")
	os.Remove("buffered_output.txt")
}
