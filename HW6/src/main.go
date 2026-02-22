package main

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Product represents the JSON structure stored and returned by the product API.
type Product struct {
	ProductID    int    `json:"product_id"`
	SKU          string `json:"sku"`
	Manufacturer string `json:"manufacturer"`
	CategoryID   int    `json:"category_id"`
	Weight       int    `json:"weight"`
	SomeOtherID  int    `json:"some_other_id"`
}

// SearchProduct represents the JSON structure used by the search endpoint.
type SearchProduct struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Brand       string `json:"brand"`
}

// SearchResponse is the JSON response for the search endpoint.
type SearchResponse struct {
	Products   []SearchProduct `json:"products"`
	TotalFound int             `json:"total_found"`
	SearchTime string          `json:"search_time"`
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

const (
	totalProducts    = 100_000
	productsToCheck  = 100
	maxResultsReturn = 20
)

var (
	store = NewProductStore()
	// key: int, value: SearchProduct
	catalog sync.Map
)

var (
	brands = []string{
		"Alpha", "Beta", "Gamma", "Delta", "Epsilon",
		"Zeta", "Eta", "Theta", "Iota", "Kappa",
	}
	categories = []string{
		"Electronics", "Books", "Home", "Garden", "Sports",
		"Clothing", "Toys", "Automotive", "Health", "Food",
	}
)

// generateCatalog creates 100,000 products with deterministic variety.
func generateCatalog() {
	for i := 0; i < totalProducts; i++ {
		brand := brands[i%len(brands)]
		cat := categories[i%len(categories)]
		catalog.Store(i, SearchProduct{
			ID:          i + 1,
			Name:        fmt.Sprintf("Product %s %d", brand, i+1),
			Category:    cat,
			Description: fmt.Sprintf("A high-quality %s product by %s", cat, brand),
			Brand:       brand,
		})
	}
}

// searchProducts checks productsToCheck products starting at a
// random offset, matches on name or category (case-insensitive), and
// returns at most maxResultsReturn results.
func searchProducts(query string) SearchResponse {
	start := time.Now()
	q := strings.ToLower(query)

	results := make([]SearchProduct, 0)
	totalFound := 0
	checked := 0

	offset := rand.Intn(totalProducts)

	for checked < productsToCheck {
		idx := (offset + checked) % totalProducts
		checked++

		val, ok := catalog.Load(idx)
		if !ok {
			continue
		}
		p := val.(SearchProduct)

		nameMatch := strings.Contains(strings.ToLower(p.Name), q)
		catMatch := strings.Contains(strings.ToLower(p.Category), q)

		if nameMatch || catMatch {
			totalFound++
			if len(results) < maxResultsReturn {
				results = append(results, p)
			}
		}
	}

	return SearchResponse{
		Products:   results,
		TotalFound: totalFound,
		SearchTime: time.Since(start).String(),
	}
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
	if err := validateProduct(product); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "INVALID_INPUT",
			Message: "Missing or invalid required fields",
			Details: err.Error(),
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

// searchHandler handles GET /products/search?q=term.
func searchHandler(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "INVALID_INPUT",
			Message: "Missing search query",
			Details: "Query parameter 'q' is required",
		})
		return
	}
	c.JSON(http.StatusOK, searchProducts(query))
}

// validateProduct validates required fields and constraints.
// Returns an error if validation fails; or nil on success.
func validateProduct(p Product) error {
	if p.ProductID < 1 {
		return errors.New("product_id must be a positive integer")
	}
	if p.SKU == "" {
		return errors.New("sku is required and cannot be empty")
	}
	if len(p.SKU) > 100 {
		return errors.New("sku must be at most 100 characters")
	}
	if p.Manufacturer == "" {
		return errors.New("manufacturer is required and cannot be empty")
	}
	if len(p.Manufacturer) > 200 {
		return errors.New("manufacturer must be at most 200 characters")
	}
	if p.CategoryID < 1 {
		return errors.New("category_id must be a positive integer")
	}
	if p.Weight < 0 {
		return errors.New("weight must be a non-negative integer")
	}
	if p.SomeOtherID < 1 {
		return errors.New("some_other_id must be a positive integer")
	}
	return nil
}

func main() {
	generateCatalog()
	fmt.Printf("Loaded %d products into search catalog\n", totalProducts)

	router := gin.Default()

	// Product endpoints
	router.GET("/products/:productId", getProduct)
	router.POST("/products/:productId/details", addProductDetails)

	// Search endpoint
	router.GET("/products/search", searchHandler)

	// Health check endpoint for ALB
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Start server on port 8080
	router.Run(":8080")
}
