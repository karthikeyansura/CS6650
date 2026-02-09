package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
)

// svc is a global S3 client used by all endpoints
var svc *s3.S3

// init initializes the AWS session and S3 client
func init() {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String("us-east-1"),
	}))
	svc = s3.New(sess)
}

// WordCount is a helper struct to store words and counts for sorted output
type WordCount struct {
	Word  string `json:"word"`
	Count int    `json:"count"`
}

func main() {
	r := gin.Default()

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "reducer"})
	})

	// Reduce endpoint
	// Usage: GET /reduce?bucket=BUCKET&keys=map_result_0.json,map_result_1.json,map_result_2.json
	r.GET("/reduce", func(c *gin.Context) {
		bucket := c.Query("bucket")
		keysStr := c.Query("keys")

		if bucket == "" || keysStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bucket and keys are required"})
			return
		}

		keys := strings.Split(keysStr, ",")
		finalCounts := make(map[string]int)

		// Read and aggregate all mapper results from S3
		for _, key := range keys {
			key = strings.TrimSpace(key)
			obj, err := svc.GetObject(&s3.GetObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read " + key + ": " + err.Error()})
				return
			}

			// Decode mapper JSON output
			var partialCounts map[string]int
			if err := json.NewDecoder(obj.Body).Decode(&partialCounts); err != nil {
				obj.Body.Close()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse " + key + ": " + err.Error()})
				return
			}
			obj.Body.Close()

			// Merge into finalCounts
			for word, count := range partialCounts {
				finalCounts[word] += count
			}
		}

		// Sort words by count descending for reporting
		var sorted []WordCount
		totalWords := 0
		for word, count := range finalCounts {
			sorted = append(sorted, WordCount{Word: word, Count: count})
			totalWords += count
		}
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Count > sorted[j].Count
		})

		top25 := sorted
		if len(top25) > 25 {
			top25 = top25[:25]
		}

		// JSON output including totals and top words
		output := map[string]interface{}{
			"total_unique_words": len(finalCounts),
			"total_words":        totalWords,
			"top_25":             top25,
			"all_counts":         finalCounts,
		}

		// Upload final result back to S3
		resultJSON, _ := json.MarshalIndent(output, "", "  ")
		outputKey := "final_result.json"

		_, err := svc.PutObject(&s3.PutObjectInput{
			Bucket:      aws.String(bucket),
			Key:         aws.String(outputKey),
			Body:        strings.NewReader(string(resultJSON)),
			ContentType: aws.String("application/json"),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload final result: " + err.Error()})
			return
		}

		// Log reducer stats for monitoring
		fmt.Printf("Reduced %d mapper outputs -> %d unique words, %d total\n", len(keys), len(finalCounts), totalWords)

		// Return response with metadata
		c.JSON(http.StatusOK, gin.H{
			"status":             "success",
			"result":             outputKey,
			"mapper_inputs":      len(keys),
			"total_unique_words": len(finalCounts),
			"total_words":        totalWords,
			"top_25":             top25,
		})
	})

	// Start HTTP server on port 8080
	r.Run(":8080")
}
