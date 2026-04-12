package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	// server
	HTTPPort string
	Region   string

	// dynamodb
	AlbumsTable   string
	CountersTable string
	PhotosTable   string

	// s3
	S3Bucket string
	S3Region string

	// sqs
	SQSQueueURL string

	// timeouts
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	// worker
	WorkerConcurrency int
}

func Load() *Config {
	c := &Config{
		HTTPPort:      envOrDefault("HTTP_PORT", "8080"),
		Region:        envOrDefault("AWS_REGION", "us-west-2"),
		AlbumsTable:   envOrDefault("ALBUMS_TABLE", "albums"),
		CountersTable: envOrDefault("COUNTERS_TABLE", "album_counters"),
		PhotosTable:   envOrDefault("PHOTOS_TABLE", "photos"),
		S3Bucket:      envOrDefault("S3_BUCKET", "album-store-photos"),
		S3Region:      envOrDefault("S3_REGION", "us-west-2"),
		SQSQueueURL:   envOrDefault("SQS_QUEUE_URL", ""),
		ReadTimeout:   durationOrDefault("READ_TIMEOUT_SEC", 60*time.Second),
		WriteTimeout:  durationOrDefault("WRITE_TIMEOUT_SEC", 60*time.Second),
		IdleTimeout:   durationOrDefault("IDLE_TIMEOUT_SEC", 120*time.Second),
	}

	concurrency, err := strconv.Atoi(envOrDefault("WORKER_CONCURRENCY", "10"))
	if err != nil {
		concurrency = 10
	}
	c.WorkerConcurrency = concurrency

	return c
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func durationOrDefault(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	secs, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return time.Duration(secs) * time.Second
}
