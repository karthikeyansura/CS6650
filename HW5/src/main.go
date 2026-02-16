package main

import (
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
)

// Product represents the JSON structure stored and returned by the API.
type Product struct {
	ProductID    int    `json:"product_id"`
	SKU          string `json:"sku"`
	Manufacturer string `json:"manufacturer"`
	CategoryID   int    `json:"category_id"`
	Weight       int    `json:"weight"`
	SomeOtherID  int    `json:"some_other_id"`
}

// ErrorResponse is the standard error payload returned by the API.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// ProductStore wraps an in-memory map with a mutex
// so concurrent requests can safely read and write products.
type ProductStore struct {
	mu       sync.RWMutex
	products map[int]Product
}

// NewProductStore creates an empty product store.
func NewProductStore() *ProductStore {
	return &ProductStore{
		products: make(map[int]Product),
	}
}

// Get returns a product by ID.
func (s *ProductStore) Get(id int) (Product, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.products[id]
	return p, ok
}

// Set inserts or updates a product by ID.
func (s *ProductStore) Set(id int, p Product) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.products[id] = p
}

var store = NewProductStore()

func main() {
	router := gin.Default()

	// Register API routes
	router.GET("/products/:productId", getProduct)
	router.POST("/products/:productId/details", addProductDetails)

	// Health check endpoint for load balancer
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Start server on port 8080
	router.Run(":8080")
}

// parseProductID extracts and validates the productId path parameter.
// Returns the parsed int and true on success, or writes an error response and returns false.
func parseProductID(c *gin.Context) (int, bool) {
	raw := c.Param("productId")
	id, err := strconv.Atoi(raw)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "INVALID_INPUT",
			Message: "Invalid product ID",
			Details: "Product ID must be a positive integer",
		})
		return 0, false
	}
	return id, true
}

// getProduct handles GET /products/:productId.
// Returns 200 with product JSON if found, 404 if product does not exist, or 400 if productId is invalid.
func getProduct(c *gin.Context) {
	id, ok := parseProductID(c)
	if !ok {
		return
	}

	product, found := store.Get(id)
	if !found {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "NOT_FOUND",
			Message: "Product not found",
			Details: "No product exists with ID " + strconv.Itoa(id),
		})
		return
	}

	c.JSON(http.StatusOK, product)
}

// addProductDetails handles POST /products/:productId/details.
// Returns 204 on success, 400 for validation errors.
func addProductDetails(c *gin.Context) {
	id, ok := parseProductID(c)
	if !ok {
		return
	}

	var product Product
	if err := c.ShouldBindJSON(&product); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "INVALID_INPUT",
			Message: "Invalid request body",
			Details: err.Error(),
		})
		return
	}

	// Validate required fields and constraints
	if err := validateProduct(product); err != "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "INVALID_INPUT",
			Message: "Missing or invalid required fields",
			Details: err,
		})
		return
	}

	// Ensure URL productId matches body product_id
	if product.ProductID != id {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "INVALID_INPUT",
			Message: "Product ID mismatch",
			Details: "Path product ID does not match body product_id",
		})
		return
	}

	store.Set(id, product)
	c.Status(http.StatusNoContent)
}

// validateProduct validates required fields and constraints.
// Returns an error message if validation fails; or an empty string on success.
func validateProduct(p Product) string {
	if p.ProductID < 1 {
		return "product_id must be a positive integer"
	}
	if p.SKU == "" {
		return "sku is required and cannot be empty"
	}
	if len(p.SKU) > 100 {
		return "sku must be at most 100 characters"
	}
	if p.Manufacturer == "" {
		return "manufacturer is required and cannot be empty"
	}
	if len(p.Manufacturer) > 200 {
		return "manufacturer must be at most 200 characters"
	}
	if p.CategoryID < 1 {
		return "category_id must be a positive integer"
	}
	if p.Weight < 0 {
		return "weight must be a non-negative integer"
	}
	if p.SomeOtherID < 1 {
		return "some_other_id must be a positive integer"
	}
	return ""
}
