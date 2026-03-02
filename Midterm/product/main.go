package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

// InventoryInfo is the response from the inventory service.
type InventoryInfo struct {
	ProductID int    `json:"product_id"`
	Stock     int    `json:"stock"`
	Warehouse string `json:"warehouse"`
	UpdatedAt string `json:"updated_at"`
}

// ProductWithInventory enriches a SearchProduct with live inventory data.
// Source indicates where the inventory data came from: "live", "circuit-open", "bulkhead-rejected", or "error".
type ProductWithInventory struct {
	SearchProduct
	Stock     int    `json:"stock"`
	Warehouse string `json:"warehouse"`
	InStock   bool   `json:"in_stock"`
	Source    string `json:"inventory_source"`
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

// CircuitState represents the three states of a circuit breaker.
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // Normal operation, requests pass through
	CircuitOpen                         // Dependency broken, requests rejected immediately
	CircuitHalfOpen                     // Testing recovery, limited requests allowed
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	}
	return "unknown"
}

// CircuitBreaker tracks consecutive failures and transitions between
// closed, open, and half-open states to protect against cascading failures.
type CircuitBreaker struct {
	mu               sync.Mutex
	state            CircuitState
	failureCount     int
	successCount     int
	failureThreshold int
	successThreshold int
	openTimeout      time.Duration
	lastFailureTime  time.Time

	totalRequests  atomic.Int64
	totalFailures  atomic.Int64
	totalSuccesses atomic.Int64
	totalRejected  atomic.Int64
}

// NewCircuitBreaker returns a circuit breaker that opens after failureThreshold
// consecutive failures, waits openTimeout before half-opening, and closes after
// successThreshold consecutive successes in half-open state.
func NewCircuitBreaker(failureThreshold, successThreshold int, openTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            CircuitClosed,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		openTimeout:      openTimeout,
	}
}

// Allow checks if a request should be allowed through the circuit breaker.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.totalRequests.Add(1)

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.lastFailureTime) > cb.openTimeout {
			cb.state = CircuitHalfOpen
			cb.successCount = 0
			fmt.Printf("[CIRCUIT BREAKER] OPEN -> HALF-OPEN (testing recovery)\n")
			return true
		}
		cb.totalRejected.Add(1)
		return false
	case CircuitHalfOpen:
		return true
	}
	return true
}

// RecordSuccess records a successful downstream call.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.totalSuccesses.Add(1)

	switch cb.state {
	case CircuitHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.successThreshold {
			cb.state = CircuitClosed
			cb.failureCount = 0
			fmt.Printf("[CIRCUIT BREAKER] HALF-OPEN -> CLOSED (recovered)\n")
		}
	case CircuitClosed:
		cb.failureCount = 0
	case CircuitOpen:
		// Success recorded but circuit remains open until timeout expires
	}
}

// RecordFailure records a failed downstream call.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.totalFailures.Add(1)
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case CircuitClosed:
		cb.failureCount++
		if cb.failureCount >= cb.failureThreshold {
			cb.state = CircuitOpen
			fmt.Printf("[CIRCUIT BREAKER] CLOSED -> OPEN (%d failures)\n", cb.failureCount)
		}
	case CircuitHalfOpen:
		cb.state = CircuitOpen
		fmt.Printf("[CIRCUIT BREAKER] HALF-OPEN -> OPEN (test failed)\n")
	case CircuitOpen:
		// Already open, failure count not tracked
	}
}

// Stats returns the current circuit breaker metrics as a JSON-friendly map.
func (cb *CircuitBreaker) Stats() gin.H {
	cb.mu.Lock()
	state := cb.state
	fc := cb.failureCount
	cb.mu.Unlock()
	return gin.H{
		"state":                state.String(),
		"consecutive_failures": fc,
		"total_requests":       cb.totalRequests.Load(),
		"total_successes":      cb.totalSuccesses.Load(),
		"total_failures":       cb.totalFailures.Load(),
		"total_rejected":       cb.totalRejected.Load(),
	}
}

// Bulkhead limits concurrent calls to a downstream service using a semaphore.
// When the semaphore is full, excess requests are rejected immediately.
type Bulkhead struct {
	sem           chan struct{}
	maxConcurrent int
	totalRejected atomic.Int64
	totalAccepted atomic.Int64
}

// NewBulkhead creates a bulkhead allowing at most maxConcurrent in-flight calls.
func NewBulkhead(maxConcurrent int) *Bulkhead {
	return &Bulkhead{
		sem:           make(chan struct{}, maxConcurrent),
		maxConcurrent: maxConcurrent,
	}
}

// Acquire tries to get a semaphore slot within timeout. Returns false if full.
func (b *Bulkhead) Acquire(timeout time.Duration) bool {
	select {
	case b.sem <- struct{}{}:
		b.totalAccepted.Add(1)
		return true
	case <-time.After(timeout):
		b.totalRejected.Add(1)
		return false
	}
}

// Release frees a semaphore slot.
func (b *Bulkhead) Release() {
	<-b.sem
}

// Stats returns bulkhead metrics.
func (b *Bulkhead) Stats() gin.H {
	return gin.H{
		"max_concurrent": b.maxConcurrent,
		"current_in_use": len(b.sem),
		"total_accepted": b.totalAccepted.Load(),
		"total_rejected": b.totalRejected.Load(),
	}
}

// Inventory client configuration
var (
	inventoryBaseURL string
	resilienceMode   string // "none", "circuit-breaker", "bulkhead"
	cb               *CircuitBreaker
	bh               *Bulkhead

	// No protection: 30s timeout means requests hang when dependency is slow
	httpClientNone = &http.Client{Timeout: 30 * time.Second}
	// Fail-fast: 500ms timeout catches slow dependencies quickly
	httpClientFast = &http.Client{Timeout: 500 * time.Millisecond}
)

// fetchInventory calls the inventory service for a product.
// Behavior depends on RESILIENCE_MODE: none, circuit-breaker, or bulkhead.
func fetchInventory(productID int) (*InventoryInfo, string, error) {
	url := fmt.Sprintf("%s/inventory/%d", inventoryBaseURL, productID)

	switch resilienceMode {
	case "none":
		return fetchInventoryRaw(url, httpClientNone)

	case "circuit-breaker":
		if !cb.Allow() {
			return nil, "circuit-open", fmt.Errorf("circuit breaker is open")
		}
		info, source, err := fetchInventoryRaw(url, httpClientFast)
		if err != nil {
			cb.RecordFailure()
			return nil, source, err
		}
		cb.RecordSuccess()
		return info, source, nil

	case "bulkhead":
		if !cb.Allow() {
			return nil, "circuit-open", fmt.Errorf("circuit breaker is open")
		}
		if !bh.Acquire(100 * time.Millisecond) {
			return nil, "bulkhead-rejected", fmt.Errorf("bulkhead full")
		}
		defer bh.Release()
		info, source, err := fetchInventoryRaw(url, httpClientFast)
		if err != nil {
			cb.RecordFailure()
			return nil, source, err
		}
		cb.RecordSuccess()
		return info, source, nil
	}

	return fetchInventoryRaw(url, httpClientNone)
}

// fetchInventoryRaw makes the HTTP GET to the inventory service.
func fetchInventoryRaw(url string, client *http.Client) (*InventoryInfo, string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, "error", fmt.Errorf("inventory request failed: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Printf("failed to close response body: %v", cerr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		if _, derr := io.ReadAll(resp.Body); derr != nil {
			log.Printf("failed to drain response body: %v", derr)
		}
		return nil, "error", fmt.Errorf("inventory returned status %d", resp.StatusCode)
	}

	var info InventoryInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, "error", fmt.Errorf("decode failed: %w", err)
	}
	return &info, "live", nil
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
// Returns 200 with product JSON if found, 404 if not, or 400 if invalid.
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

// getProductWithInventory handles GET /products/:productId/inventory.
// Enriches a catalog product with live inventory data from the inventory service.
// On inventory failure, returns product data with degraded inventory info.
func getProductWithInventory(c *gin.Context) {
	id, ok := parseProductID(c)
	if !ok {
		return
	}

	val, exists := catalog.Load(id - 1)
	if !exists {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "NOT_FOUND",
			Message: "Product not found in catalog",
		})
		return
	}
	product := val.(SearchProduct)

	inv, source, err := fetchInventory(id)
	result := ProductWithInventory{SearchProduct: product}
	if err != nil {
		result.Stock = -1
		result.Warehouse = "unknown"
		result.InStock = false
		result.Source = source
	} else {
		result.Stock = inv.Stock
		result.Warehouse = inv.Warehouse
		result.InStock = inv.Stock > 0
		result.Source = source
	}

	c.JSON(http.StatusOK, result)
}

// searchWithInventory handles GET /products/search/inventory?q=term.
// Enriches search results with live inventory data for each product.
func searchWithInventory(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "INVALID_INPUT",
			Message: "Missing search query",
			Details: "Query parameter 'q' is required",
		})
		return
	}

	searchResult := searchProducts(query)
	enriched := make([]ProductWithInventory, 0, len(searchResult.Products))

	for _, p := range searchResult.Products {
		inv, source, err := fetchInventory(p.ID)
		item := ProductWithInventory{SearchProduct: p}
		if err != nil {
			item.Stock = -1
			item.Warehouse = "unknown"
			item.InStock = false
			item.Source = source
		} else {
			item.Stock = inv.Stock
			item.Warehouse = inv.Warehouse
			item.InStock = inv.Stock > 0
			item.Source = source
		}
		enriched = append(enriched, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"products":    enriched,
		"total_found": searchResult.TotalFound,
		"search_time": searchResult.SearchTime,
	})
}

// metricsHandler handles GET /metrics.
// Returns circuit breaker and bulkhead statistics.
func metricsHandler(c *gin.Context) {
	response := gin.H{"resilience_mode": resilienceMode}
	if cb != nil {
		response["circuit_breaker"] = cb.Stats()
	}
	if bh != nil {
		response["bulkhead"] = bh.Stats()
	}
	c.JSON(http.StatusOK, response)
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

	// Configuration via environment variables
	inventoryBaseURL = os.Getenv("INVENTORY_URL")
	if inventoryBaseURL == "" {
		inventoryBaseURL = "http://localhost:8081"
	}
	resilienceMode = os.Getenv("RESILIENCE_MODE")
	if resilienceMode == "" {
		resilienceMode = "none"
	}
	fmt.Printf("Resilience mode: %s\n", resilienceMode)
	fmt.Printf("Inventory URL: %s\n", inventoryBaseURL)

	// Initialize resilience components
	if resilienceMode == "circuit-breaker" || resilienceMode == "bulkhead" {
		cb = NewCircuitBreaker(5, 3, 10*time.Second)
		fmt.Println("Circuit breaker initialized: threshold=5, recovery=3, timeout=10s")
	}
	if resilienceMode == "bulkhead" {
		bh = NewBulkhead(10)
		fmt.Println("Bulkhead initialized: max_concurrent=10")
	}

	router := gin.Default()

	// Search endpoints (registered first for exact match priority)
	router.GET("/products/search", searchHandler)
	router.GET("/products/search/inventory", searchWithInventory)

	// Product endpoints
	router.GET("/products/:productId", getProduct)
	router.POST("/products/:productId/details", addProductDetails)
	router.GET("/products/:productId/inventory", getProductWithInventory)

	// Resilience metrics endpoint
	router.GET("/metrics", metricsHandler)

	// Health check endpoint for ALB
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":          "ok",
			"resilience_mode": resilienceMode,
		})
	})

	// Start server on port 8080
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Failed to start product service: %v", err)
	}
}
