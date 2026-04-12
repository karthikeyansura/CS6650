package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/karthikeyansura/CS6650/Final/internal/blob"
	"github.com/karthikeyansura/CS6650/Final/internal/config"
	"github.com/karthikeyansura/CS6650/Final/internal/handler"
	"github.com/karthikeyansura/CS6650/Final/internal/middleware"
	"github.com/karthikeyansura/CS6650/Final/internal/queue"
	"github.com/karthikeyansura/CS6650/Final/internal/store"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.Load()

	// load AWS SDK config with custom HTTP transport for connection pooling
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithHTTPClient(&http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        500,
				MaxIdleConnsPerHost: 200,
				MaxConnsPerHost:     200,
				IdleConnTimeout:     90 * time.Second,
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
			},
		}),
	)
	if err != nil {
		slog.Error("failed to load aws config", "error", err)
		os.Exit(1)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	s3Client := s3.NewFromConfig(awsCfg)
	sqsClient := sqs.NewFromConfig(awsCfg)

	dynamoStore := store.NewDynamoStore(dynamoClient, cfg.AlbumsTable, cfg.CountersTable, cfg.PhotosTable)
	blobClient := blob.NewS3Client(s3Client, cfg.S3Bucket, cfg.S3Region)
	sqsQueue := queue.NewSQSQueue(sqsClient, cfg.SQSQueueURL)

	h := &handler.Handler{
		Store: dynamoStore,
		Blob:  blobClient,
		Queue: sqsQueue,
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// wrap with middleware: recovery first (outermost), then logging
	var httpHandler http.Handler = mux
	httpHandler = middleware.Logging(httpHandler)
	httpHandler = middleware.Recovery(httpHandler)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.HTTPPort),
		Handler:      httpHandler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	// graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("api server starting", "port", cfg.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	slog.Info("shutting down api server")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}
	slog.Info("api server stopped")
}
