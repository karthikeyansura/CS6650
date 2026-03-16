package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// Order mirrors the structure published by the receiver.
type Order struct {
	OrderID    string            `json:"order_id"`
	CustomerID int               `json:"customer_id"`
	Status     string            `json:"status"`
	Items      []json.RawMessage `json:"items"`
	CreatedAt  time.Time         `json:"created_at"`
}

// SNSMessage wraps the actual message inside the SNS notification envelope.
type SNSMessage struct {
	Message string `json:"Message"`
}

var (
	sqsClient   *sqs.Client
	queueURL    string
	workerCount int
	processed   atomic.Int64
)

func initSQS() {
	queueURL = os.Getenv("SQS_QUEUE_URL")
	if queueURL == "" {
		log.Fatal("SQS_QUEUE_URL environment variable is required")
	}

	wc := os.Getenv("WORKER_COUNT")
	if wc == "" {
		workerCount = 1
	} else {
		var err error
		workerCount, err = strconv.Atoi(wc)
		if err != nil || workerCount < 1 {
			log.Fatalf("Invalid WORKER_COUNT: %s", wc)
		}
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}
	sqsClient = sqs.NewFromConfig(cfg)
	log.Printf("SQS processor initialized: queue=%s, workers=%d", queueURL, workerCount)
}

// processOrder simulates the 3-second payment verification.
func processOrder(order Order) {
	log.Printf("Processing order %s for customer %d", order.OrderID, order.CustomerID)
	time.Sleep(3 * time.Second)
	processed.Add(1)
	log.Printf("Completed order %s (total processed: %d)", order.OrderID, processed.Load())
}

// pollAndProcess continuously polls SQS and dispatches messages to worker goroutines.
// workerWg tracks in-flight order processing tasks during graceful shutdown.
func pollAndProcess(ctx context.Context, sem chan struct{}, workerWg *sync.WaitGroup) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		result, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            &queueURL,
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     20, // long polling
		})
		if err != nil {
			if ctx.Err() != nil {
				return // context cancelled, shutting down
			}
			log.Printf("SQS receive error: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		for _, msg := range result.Messages {
			// Check context before blocking on semaphore to avoid accepting
			// new work after shutdown signal
			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}: // acquire worker slot
			}

			workerWg.Add(1)
			go func(m types.Message) {
				defer workerWg.Done()
				defer func() { <-sem }() // release worker slot

				// Unwrap SNS envelope
				var snsMsg SNSMessage
				if err := json.Unmarshal([]byte(*m.Body), &snsMsg); err != nil {
					log.Printf("Failed to parse SNS envelope: %v", err)
					// Use background context for delete since main ctx may be cancelled
					deleteMessage(context.Background(), m)
					return
				}

				var order Order
				if err := json.Unmarshal([]byte(snsMsg.Message), &order); err != nil {
					log.Printf("Failed to parse order: %v", err)
					deleteMessage(context.Background(), m)
					return
				}

				processOrder(order)
				// Use background context so delete succeeds even during shutdown
				deleteMessage(context.Background(), m)
			}(msg)
		}
	}
}

func deleteMessage(ctx context.Context, msg types.Message) {
	_, err := sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      &queueURL,
		ReceiptHandle: msg.ReceiptHandle,
	})
	if err != nil {
		log.Printf("Failed to delete message: %v", err)
	}
}

func main() {
	initSQS()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Semaphore channel controls max concurrent worker goroutines
	sem := make(chan struct{}, workerCount)

	// pollerWg tracks polling goroutines; workerWg tracks in-flight order processors
	var pollerWg sync.WaitGroup
	var workerWg sync.WaitGroup

	pollerCount := 2
	if workerCount >= 20 {
		pollerCount = 4
	}
	for i := 0; i < pollerCount; i++ {
		pollerWg.Add(1)
		go func(id int) {
			defer pollerWg.Done()
			log.Printf("Poller %d started", id)
			pollAndProcess(ctx, sem, &workerWg)
		}(i)
	}

	// Periodic stats logging
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				log.Printf("Stats: processed=%d, active_workers=%d/%d",
					processed.Load(), len(sem), workerCount)
			}
		}
	}()

	fmt.Printf("Order Processor running with %d workers\n", workerCount)

	// Graceful shutdown on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down: stopping pollers...")
	cancel()
	pollerWg.Wait()

	log.Println("Waiting for in-flight workers to finish...")
	workerWg.Wait()

	log.Printf("Shutdown complete. Total orders processed: %d", processed.Load())
}
