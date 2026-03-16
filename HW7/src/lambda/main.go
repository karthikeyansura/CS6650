package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// Order mirrors the order structure from the receiver.
type Order struct {
	OrderID    string    `json:"order_id"`
	CustomerID int       `json:"customer_id"`
	Status     string    `json:"status"`
	Items      []json.RawMessage `json:"items"`
	CreatedAt  time.Time `json:"created_at"`
}

// handler processes SNS events triggered by order publications.
func handler(ctx context.Context, snsEvent events.SNSEvent) error {
	for _, record := range snsEvent.Records {
		var order Order
		if err := json.Unmarshal([]byte(record.SNS.Message), &order); err != nil {
			log.Printf("Failed to parse order: %v", err)
			continue
		}

		log.Printf("Processing order %s for customer %d", order.OrderID, order.CustomerID)

		// Simulate 3-second payment verification
		time.Sleep(3 * time.Second)

		log.Printf("Completed order %s", order.OrderID)
	}
	return nil
}

func main() {
	lambda.Start(handler)
}
