package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

// Data models

// CartItem represents a single item inside a shopping cart.
type CartItem struct {
	ProductID int `json:"product_id"`
	Quantity  int `json:"quantity"`
}

// ShoppingCart is the API response for a cart retrieval.
type ShoppingCart struct {
	ShoppingCartID int        `json:"shopping_cart_id"`
	CustomerID     int        `json:"customer_id"`
	Items          []CartItem `json:"items"`
	CreatedAt      string     `json:"created_at"`
}

// ErrorResponse is the standard error payload.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// Global state

var (
	db          *sql.DB
	dynamoDBSvc *dynamodb.Client
	dbMode      string // "mysql" or "dynamodb"
	tableName   string
)

// Initialisation

func initMySQL() {
	dsn := os.Getenv("MYSQL_DSN") // user:pass@tcp(host:3306)/dbname?parseTime=true
	if dsn == "" {
		log.Fatal("MYSQL_DSN env variable is required")
	}

	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("failed to open mysql: %v", err)
	}

	// Connection pool tuning
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify connectivity with Retry / Backoff Logic
	maxRetries := 6
	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = db.PingContext(ctx)
		cancel()

		if err == nil {
			break
		}

		log.Printf("MySQL not ready (attempt %d/%d): %v. Retrying in 10 seconds...", i+1, maxRetries, err)
		time.Sleep(10 * time.Second)
	}

	// If it still fails after 60 seconds of retrying, then crash
	if err != nil {
		log.Fatalf("mysql ping failed after %d attempts: %v", maxRetries, err)
	}

	// Run schema migration
	if err := migrateMySQL(); err != nil {
		log.Fatalf("mysql migration failed: %v", err)
	}
	log.Println("MySQL connected and schema migrated")
}

func migrateMySQL() error {
	// Create shopping_carts table
	schemaCarts := `
	CREATE TABLE IF NOT EXISTS shopping_carts (
		id         INT AUTO_INCREMENT PRIMARY KEY,
		customer_id INT NOT NULL,
		created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_customer (customer_id)
	) ENGINE=InnoDB;`

	if _, err := db.Exec(schemaCarts); err != nil {
		return fmt.Errorf("failed to create shopping_carts table: %v", err)
	}

	// Create cart_items table (with the foreign key)
	schemaItems := `
	CREATE TABLE IF NOT EXISTS cart_items (
		id          INT AUTO_INCREMENT PRIMARY KEY,
		cart_id     INT NOT NULL,
		product_id  INT NOT NULL,
		quantity    INT NOT NULL DEFAULT 1,
		UNIQUE KEY uq_cart_product (cart_id, product_id),
		CONSTRAINT fk_cart FOREIGN KEY (cart_id) REFERENCES shopping_carts(id) ON DELETE CASCADE,
		INDEX idx_cart (cart_id)
	) ENGINE=InnoDB;`

	if _, err := db.Exec(schemaItems); err != nil {
		return fmt.Errorf("failed to create cart_items table: %v", err)
	}

	return nil
}

func initDynamoDB() {
	tableName = os.Getenv("DYNAMODB_TABLE")
	if tableName == "" {
		tableName = "shopping-carts"
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(os.Getenv("AWS_REGION")))
	if err != nil {
		log.Fatalf("unable to load AWS config: %v", err)
	}
	dynamoDBSvc = dynamodb.NewFromConfig(cfg)
	log.Printf("DynamoDB client initialised for table %s", tableName)
}

// MySQL handlers

func mysqlCreateCart(c *gin.Context) {
	var req struct {
		CustomerID int `json:"customer_id" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "INVALID_INPUT", Message: "Invalid request body", Details: err.Error()})
		return
	}

	res, err := db.ExecContext(c.Request.Context(),
		"INSERT INTO shopping_carts (customer_id) VALUES (?)", req.CustomerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "DB_ERROR", Message: "Failed to create cart"})
		return
	}

	id, _ := res.LastInsertId()
	c.JSON(http.StatusCreated, gin.H{"shopping_cart_id": id})
}

func mysqlGetCart(c *gin.Context) {
	cartID, err := strconv.Atoi(c.Param("id"))
	if err != nil || cartID < 1 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "INVALID_INPUT", Message: "Invalid cart ID"})
		return
	}

	ctx := c.Request.Context()

	// Fetch cart header
	var cart ShoppingCart
	var createdAt time.Time
	err = db.QueryRowContext(ctx,
		"SELECT id, customer_id, created_at FROM shopping_carts WHERE id = ?", cartID).
		Scan(&cart.ShoppingCartID, &cart.CustomerID, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "NOT_FOUND", Message: "Shopping cart not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "DB_ERROR", Message: "Failed to retrieve cart"})
		return
	}
	cart.CreatedAt = createdAt.Format(time.RFC3339)

	// Fetch items via JOIN-free indexed lookup
	rows, err := db.QueryContext(ctx,
		"SELECT product_id, quantity FROM cart_items WHERE cart_id = ?", cartID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "DB_ERROR", Message: "Failed to retrieve items"})
		return
	}

	// Better defer pattern: log error if closing fails
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("error closing rows: %v", closeErr)
		}
	}()

	cart.Items = make([]CartItem, 0)
	for rows.Next() {
		var item CartItem
		if err := rows.Scan(&item.ProductID, &item.Quantity); err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "DB_ERROR", Message: "Failed to scan item"})
			return
		}
		cart.Items = append(cart.Items, item)
	}

	c.JSON(http.StatusOK, cart)
}

func mysqlAddItems(c *gin.Context) {
	cartID, err := strconv.Atoi(c.Param("id"))
	if err != nil || cartID < 1 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "INVALID_INPUT", Message: "Invalid cart ID"})
		return
	}

	var req struct {
		ProductID int `json:"product_id" binding:"required,min=1"`
		Quantity  int `json:"quantity" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "INVALID_INPUT", Message: "Invalid request body", Details: err.Error()})
		return
	}

	ctx := c.Request.Context()

	// Verify cart exists
	var exists int
	err = db.QueryRowContext(ctx, "SELECT 1 FROM shopping_carts WHERE id = ?", cartID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "NOT_FOUND", Message: "Shopping cart not found"})
		return
	}

	// Upsert item (INSERT ... ON DUPLICATE KEY UPDATE)
	_, err = db.ExecContext(ctx,
		`INSERT INTO cart_items (cart_id, product_id, quantity)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE quantity = quantity + VALUES(quantity)`,
		cartID, req.ProductID, req.Quantity)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "DB_ERROR", Message: "Failed to add item"})
		return
	}

	c.Status(http.StatusNoContent)
}

// DynamoDB handlers

type DynamoCart struct {
	CartID     string     `dynamodbav:"cart_id"`
	CustomerID int        `dynamodbav:"customer_id"`
	Items      []CartItem `dynamodbav:"items"`
	CreatedAt  string     `dynamodbav:"created_at"`
}

func dynamoCreateCart(c *gin.Context) {
	var req struct {
		CustomerID int `json:"customer_id" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "INVALID_INPUT", Message: "Invalid request body", Details: err.Error()})
		return
	}

	cart := DynamoCart{
		CartID:     uuid.New().String(),
		CustomerID: req.CustomerID,
		Items:      []CartItem{},
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	av, err := attributevalue.MarshalMap(cart)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "MARSHAL_ERROR", Message: "Failed to marshal cart"})
		return
	}

	_, err = dynamoDBSvc.PutItem(c.Request.Context(), &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      av,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "DB_ERROR", Message: "Failed to create cart", Details: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"shopping_cart_id": cart.CartID})
}

func dynamoGetCart(c *gin.Context) {
	cartID := c.Param("id")
	if cartID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "INVALID_INPUT", Message: "Cart ID required"})
		return
	}

	result, err := dynamoDBSvc.GetItem(c.Request.Context(), &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]dbtypes.AttributeValue{
			"cart_id": &dbtypes.AttributeValueMemberS{Value: cartID},
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "DB_ERROR", Message: "Failed to get cart", Details: err.Error()})
		return
	}
	if result.Item == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "NOT_FOUND", Message: "Shopping cart not found"})
		return
	}

	var cart DynamoCart
	if err := attributevalue.UnmarshalMap(result.Item, &cart); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "UNMARSHAL_ERROR", Message: "Failed to unmarshal cart"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"shopping_cart_id": cart.CartID,
		"customer_id":      cart.CustomerID,
		"items":            cart.Items,
		"created_at":       cart.CreatedAt,
	})
}

func dynamoAddItems(c *gin.Context) {
	cartID := c.Param("id")
	if cartID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "INVALID_INPUT", Message: "Cart ID required"})
		return
	}

	var req struct {
		ProductID int `json:"product_id" binding:"required,min=1"`
		Quantity  int `json:"quantity" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "INVALID_INPUT", Message: "Invalid request body", Details: err.Error()})
		return
	}

	ctx := c.Request.Context()

	// First, get the current cart
	result, err := dynamoDBSvc.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]dbtypes.AttributeValue{
			"cart_id": &dbtypes.AttributeValueMemberS{Value: cartID},
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "DB_ERROR", Message: "Failed to get cart"})
		return
	}
	if result.Item == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "NOT_FOUND", Message: "Shopping cart not found"})
		return
	}

	var cart DynamoCart
	if err := attributevalue.UnmarshalMap(result.Item, &cart); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "UNMARSHAL_ERROR", Message: "Failed to unmarshal cart"})
		return
	}

	// Upsert item in the items list
	found := false
	for i, item := range cart.Items {
		if item.ProductID == req.ProductID {
			cart.Items[i].Quantity += req.Quantity
			found = true
			break
		}
	}
	if !found {
		cart.Items = append(cart.Items, CartItem{ProductID: req.ProductID, Quantity: req.Quantity})
	}

	// Write updated cart back
	av, err := attributevalue.MarshalMap(cart)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "MARSHAL_ERROR", Message: "Failed to marshal cart"})
		return
	}
	_, err = dynamoDBSvc.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      av,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "DB_ERROR", Message: "Failed to update cart"})
		return
	}

	c.Status(http.StatusNoContent)
}

// Main

func main() {
	dbMode = os.Getenv("DB_MODE") // "mysql" or "dynamodb"
	if dbMode == "" {
		dbMode = "mysql"
	}
	fmt.Printf("Starting shopping cart service in %s mode\n", dbMode)

	switch dbMode {
	case "mysql":
		initMySQL()
	case "dynamodb":
		initDynamoDB()
	default:
		log.Fatalf("unsupported DB_MODE: %s (use 'mysql' or 'dynamodb')", dbMode)
	}

	router := gin.Default()

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "db_mode": dbMode})
	})

	// Shopping cart endpoints — route to the correct backend
	switch dbMode {
	case "mysql":
		router.POST("/shopping-carts", mysqlCreateCart)
		router.GET("/shopping-carts/:id", mysqlGetCart)
		router.POST("/shopping-carts/:id/items", mysqlAddItems)
	case "dynamodb":
		router.POST("/shopping-carts", dynamoCreateCart)
		router.GET("/shopping-carts/:id", dynamoGetCart)
		router.POST("/shopping-carts/:id/items", dynamoAddItems)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Safely start server
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
