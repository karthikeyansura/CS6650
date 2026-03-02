package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ChaosConfig controls failure injection modes for the inventory service.
type ChaosConfig struct {
	mu      sync.RWMutex
	mode    string // "off", "slow", "error", "partial"
	delayMs int
}

var chaos = &ChaosConfig{mode: "off", delayMs: 3000}

func (c *ChaosConfig) getMode() (string, int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.mode, c.delayMs
}

func (c *ChaosConfig) setMode(mode string, delayMs int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mode = mode
	c.delayMs = delayMs
}

func main() {
	router := gin.Default()

	// Returns stock info for a given product ID
	router.GET("/inventory/:productId", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("productId"))
		if err != nil || id < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product ID"})
			return
		}

		mode, delayMs := chaos.getMode()
		switch mode {
		case "slow":
			time.Sleep(time.Duration(delayMs) * time.Millisecond)
		case "error":
			c.JSON(http.StatusInternalServerError, gin.H{"error": "inventory database unavailable"})
			return
		case "partial":
			if rand.Float64() < 0.5 {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "intermittent failure"})
				return
			}
			time.Sleep(time.Duration(rand.Intn(delayMs)) * time.Millisecond)
		}

		// Deterministic stock based on product ID
		stock := (id * 17) % 500
		c.JSON(http.StatusOK, gin.H{
			"product_id": id,
			"stock":      stock,
			"warehouse":  "US-EAST-1",
			"updated_at": time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Toggle chaos mode: off, slow, error, partial
	router.POST("/chaos/:mode", func(c *gin.Context) {
		mode := c.Param("mode")
		delayMs := 3000
		if d := c.Query("delay_ms"); d != "" {
			if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
				delayMs = parsed
			}
		}
		switch mode {
		case "off", "slow", "error", "partial":
			chaos.setMode(mode, delayMs)
			c.JSON(http.StatusOK, gin.H{
				"chaos_mode": mode,
				"delay_ms":   delayMs,
				"message":    fmt.Sprintf("Chaos mode set to '%s'", mode),
			})
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"error":       "invalid mode",
				"valid_modes": []string{"off", "slow", "error", "partial"},
			})
		}
	})

	// Check current chaos status
	router.GET("/chaos", func(c *gin.Context) {
		mode, delayMs := chaos.getMode()
		c.JSON(http.StatusOK, gin.H{"chaos_mode": mode, "delay_ms": delayMs})
	})

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "inventory"})
	})

	if err := router.Run(":8081"); err != nil {
		log.Fatalf("Failed to start inventory service: %v", err)
	}
}
