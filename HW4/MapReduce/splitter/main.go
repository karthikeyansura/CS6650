package main

import (
	"bufio"
	"fmt"
	"net/http"
	"strconv"
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

func main() {
	r := gin.Default()

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "splitter"})
	})

	// Split endpoint
	// Usage: GET /split?bucket=BUCKET&key=FILE_KEY&n=NUMBER_OF_CHUNKS
	r.GET("/split", func(c *gin.Context) {
		bucket := c.Query("bucket")
		key := c.Query("key")
		splitCount, _ := strconv.Atoi(c.DefaultQuery("n", "3"))

		if bucket == "" || key == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bucket and key are required"})
			return
		}

		// Fetch the object from S3
		obj, err := svc.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get input file: " + err.Error()})
			return
		}
		defer obj.Body.Close()

		// Read the file line by line
		scanner := bufio.NewScanner(obj.Body)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		var lines []string
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file: " + err.Error()})
			return
		}

		// Determine size of each chunk and split lines
		chunkSize := (len(lines) + splitCount - 1) / splitCount
		var outputKeys []string

		for i := 0; i < splitCount; i++ {
			start := i * chunkSize
			end := start + chunkSize
			if end > len(lines) {
				end = len(lines)
			}
			if start >= len(lines) {
				break
			}

			// Join the lines for this chunk
			chunkContent := strings.Join(lines[start:end], "\n")
			chunkKey := fmt.Sprintf("chunk_%d.txt", i)

			// Upload chunk to S3
			_, err := svc.PutObject(&s3.PutObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(chunkKey),
				Body:   strings.NewReader(chunkContent),
			})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("Failed to upload chunk %d: %s", i, err.Error()),
				})
				return
			}
			outputKeys = append(outputKeys, chunkKey)
		}

		// Log splitting summary for monitoring
		fmt.Printf("Split %d lines into %d chunks\n", len(lines), len(outputKeys))

		// Return the list of S3 keys for the chunks
		c.JSON(http.StatusOK, gin.H{
			"status":      "success",
			"total_lines": len(lines),
			"chunks":      outputKeys,
		})
	})

	// Start the HTTP server on port 8080
	r.Run(":8080")
}
