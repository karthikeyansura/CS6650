package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Order represents an e-commerce order.
type Order struct {
	OrderID    string    `json:"order_id"`
	CustomerID int       `json:"customer_id"`
	Status     string    `json:"status"`
	Items      []Item    `json:"items"`
	CreatedAt  time.Time `json:"created_at"`
}

// Item represents a single line item in an order.
type Item struct {
	ProductID int     `json:"product_id"`
	Name      string  `json:"name"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

// OrderRequest is the incoming JSON payload for creating an order.
type OrderRequest struct {
	CustomerID int    `json:"customer_id" binding:"required"`
	Items      []Item `json:"items" binding:"required"`
}

// paymentSemaphore enforces a strict concurrency limit of 1 for synchronous processing.
// Because time.Sleep does not block the underlying OS thread in Go, a buffered
// channel is required to prevent goroutines from processing concurrently.
var paymentSemaphore = make(chan struct{}, 1)

var snsClient *sns.Client
var snsTopicARN string

func initSNS() {
	snsTopicARN = os.Getenv("SNS_TOPIC_ARN")
	if snsTopicARN == "" {
		log.Println("WARNING: SNS_TOPIC_ARN not set; async endpoint will fail")
		return
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}
	snsClient = sns.NewFromConfig(cfg)
	log.Printf("SNS client initialized, topic: %s", snsTopicARN)
}

// simulatePayment acquires the semaphore, sleeps 3s (payment verification),
// then releases. This creates a genuine throughput bottleneck of ~0.33 orders/sec.
func simulatePayment() {
	paymentSemaphore <- struct{}{} // acquire slot (blocks if full)
	time.Sleep(3 * time.Second)    // simulate payment verification
	<-paymentSemaphore             // release slot
}

// syncOrderHandler handles POST /orders/sync.
// Processes payment synchronously: the client waits the full 3 seconds.
func syncOrderHandler(c *gin.Context) {
	var req OrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	order := Order{
		OrderID:    uuid.New().String(),
		CustomerID: req.CustomerID,
		Status:     "processing",
		Items:      req.Items,
		CreatedAt:  time.Now(),
	}

	// Synchronous payment: blocks until payment semaphore is available + 3s delay
	simulatePayment()

	order.Status = "completed"
	c.JSON(http.StatusOK, gin.H{
		"order_id": order.OrderID,
		"status":   order.Status,
		"message":  "Order processed successfully",
	})
}

// asyncOrderHandler handles POST /orders/async.
// Publishes the order to SNS and returns 202 Accepted immediately.
func asyncOrderHandler(c *gin.Context) {
	var req OrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	order := Order{
		OrderID:    uuid.New().String(),
		CustomerID: req.CustomerID,
		Status:     "pending",
		Items:      req.Items,
		CreatedAt:  time.Now(),
	}

	// Marshal order to JSON for SNS message
	orderJSON, err := json.Marshal(order)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize order"})
		return
	}

	// Publish to SNS
	_, err = snsClient.Publish(context.TODO(), &sns.PublishInput{
		TopicArn: aws.String(snsTopicARN),
		Message:  aws.String(string(orderJSON)),
	})
	if err != nil {
		log.Printf("SNS publish failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to queue order"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"order_id": order.OrderID,
		"status":   order.Status,
		"message":  "Order accepted for processing",
	})
}

func main() {
	initSNS()

	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.POST("/orders/sync", syncOrderHandler)
	router.POST("/orders/async", asyncOrderHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("Order Receiver starting on :%s\n", port)
	router.Run(":" + port)
}
