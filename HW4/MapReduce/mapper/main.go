package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode"

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

func main() {
	r := gin.Default()

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "mapper"})
	})

	// Map endpoint
	// Usage: GET /map?bucket=BUCKET&key=CHUNK_KEY&id=MAPPER_ID
	r.GET("/map", func(c *gin.Context) {
		bucket := c.Query("bucket")
		key := c.Query("key")
		mapperID := c.Query("id")

		if bucket == "" || key == "" || mapperID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bucket, key, and id are required"})
			return
		}

		// Fetch the chunk from S3
		obj, err := svc.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get chunk: " + err.Error()})
			return
		}
		defer obj.Body.Close()

		// Read the chunk content into a string builder
		buf := new(strings.Builder)
		_, err = io.Copy(buf, obj.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read chunk: " + err.Error()})
			return
		}

		// Count words
		// Split on non-letter/non-digit boundaries
		// Convert all words to lowercase
		wordCounts := make(map[string]int)
		splitter := func(c rune) bool {
			return !unicode.IsLetter(c) && !unicode.IsNumber(c)
		}
		words := strings.FieldsFunc(buf.String(), splitter)
		for _, word := range words {
			wordCounts[strings.ToLower(word)]++
		}

		// Serialize result to JSON and upload to S3
		resultJSON, _ := json.Marshal(wordCounts)
		outputKey := fmt.Sprintf("map_result_%s.json", mapperID)

		_, err = svc.PutObject(&s3.PutObjectInput{
			Bucket:      aws.String(bucket),
			Key:         aws.String(outputKey),
			Body:        strings.NewReader(string(resultJSON)),
			ContentType: aws.String("application/json"),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload result: " + err.Error()})
			return
		}

		// Log mapper stats for monitoring
		fmt.Printf("Mapper %s: %d total words -> %d unique from %s\n", mapperID, len(words), len(wordCounts), key)

		// Return response with metadata
		c.JSON(http.StatusOK, gin.H{
			"status":       "success",
			"mapper_id":    mapperID,
			"input_key":    key,
			"result":       outputKey,
			"total_words":  len(words),
			"unique_words": len(wordCounts),
		})
	})

	// Start HTTP server on port 8080
	r.Run(":8080")
}
