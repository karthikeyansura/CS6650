package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/karthikeyansura/CS6650/Final/internal/blob"
	"github.com/karthikeyansura/CS6650/Final/internal/config"
	"github.com/karthikeyansura/CS6650/Final/internal/model"
	"github.com/karthikeyansura/CS6650/Final/internal/store"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.Load()

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-done
		slog.Info("worker received shutdown signal")
		cancel()
	}()

	slog.Info("worker starting", "concurrency", cfg.WorkerConcurrency, "queue_url", cfg.SQSQueueURL)

	// run multiple polling goroutines for throughput
	var wg sync.WaitGroup
	for i := 0; i < cfg.WorkerConcurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			pollLoop(ctx, workerID, sqsClient, cfg.SQSQueueURL, dynamoStore, blobClient)
		}(i)
	}

	wg.Wait()
	slog.Info("worker stopped")
}

func pollLoop(ctx context.Context, workerID int, sqsClient *sqs.Client, queueURL string, dynamoStore *store.DynamoStore, blobClient *blob.S3Client) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		out, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            &queueURL,
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     20,
			VisibilityTimeout:   120,
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("sqs receive error", "worker_id", workerID, "error", err)
			time.Sleep(1 * time.Second)
			continue
		}

		for _, msg := range out.Messages {
			processMessage(ctx, workerID, sqsClient, queueURL, msg, dynamoStore, blobClient)
		}
	}
}

func processMessage(ctx context.Context, workerID int, sqsClient *sqs.Client, queueURL string, msg sqstypes.Message, dynamoStore *store.DynamoStore, blobClient *blob.S3Client) {
	var job model.PhotoJob
	if err := json.Unmarshal([]byte(*msg.Body), &job); err != nil {
		slog.Error("malformed sqs message, deleting", "worker_id", workerID, "error", err)
		deleteMessage(ctx, sqsClient, queueURL, msg)
		return
	}

	slog.Info("processing photo", "worker_id", workerID, "album_id", job.AlbumID, "photo_id", job.PhotoID)

	// check if photo record still exists (handles delete race condition for S7/S8/S9)
	photo, err := dynamoStore.GetPhoto(ctx, job.AlbumID, job.PhotoID)
	if err != nil {
		slog.Error("get photo for processing", "photo_id", job.PhotoID, "error", err)
		// do not delete message; let visibility timeout retry
		return
	}
	if photo == nil {
		// photo was deleted while in queue, clean up S3 and ack
		slog.Info("photo deleted during processing, cleaning up", "photo_id", job.PhotoID)
		_ = blobClient.Delete(ctx, job.S3Key)
		deleteMessage(ctx, sqsClient, queueURL, msg)
		return
	}

	// idempotency: if already completed, just ack
	if photo.Status == "completed" {
		slog.Info("photo already completed, skipping", "photo_id", job.PhotoID)
		deleteMessage(ctx, sqsClient, queueURL, msg)
		return
	}

	// the file is already in S3 (uploaded by the API handler)
	// generate the public URL
	objectURL := blobClient.ObjectURL(job.S3Key)

	// conditional update: only set completed if the record still exists
	// prevents zombie records if the photo was deleted between our GetPhoto and this update
	err = dynamoStore.CompletePhoto(ctx, job.AlbumID, job.PhotoID, objectURL)
	if err != nil {
		if store.IsConditionalCheckFailed(err) {
			// photo was deleted after our initial check, clean up S3
			slog.Info("photo deleted during completion, cleaning up", "photo_id", job.PhotoID)
			_ = blobClient.Delete(ctx, job.S3Key)
			deleteMessage(ctx, sqsClient, queueURL, msg)
			return
		}
		slog.Error("complete photo failed", "photo_id", job.PhotoID, "error", err)
		// do not delete message; let visibility timeout retry
		return
	}

	slog.Info("photo completed", "worker_id", workerID, "photo_id", job.PhotoID, "url", objectURL)
	deleteMessage(ctx, sqsClient, queueURL, msg)
}

func deleteMessage(ctx context.Context, sqsClient *sqs.Client, queueURL string, msg sqstypes.Message) {
	_, err := sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      &queueURL,
		ReceiptHandle: msg.ReceiptHandle,
	})
	if err != nil {
		slog.Error("sqs delete message failed", "error", err)
	}
}
