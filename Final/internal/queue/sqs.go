package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/karthikeyansura/CS6650/Final/internal/model"
)

type SQSQueue struct {
	client   *sqs.Client
	queueURL string
}

func NewSQSQueue(client *sqs.Client, queueURL string) *SQSQueue {
	return &SQSQueue{
		client:   client,
		queueURL: queueURL,
	}
}

// Enqueue sends a photo processing job to SQS.
func (q *SQSQueue) Enqueue(ctx context.Context, job *model.PhotoJob) error {
	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}

	_, err = q.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    &q.queueURL,
		MessageBody: aws.String(string(body)),
	})
	if err != nil {
		return fmt.Errorf("sqs send: %w", err)
	}
	return nil
}

// Receive polls SQS for messages using long polling.
// Returns up to maxMessages. Blocks for up to waitSeconds if no messages available.
func (q *SQSQueue) Receive(ctx context.Context, maxMessages int32, waitSeconds int32) ([]sqs.ReceiveMessageOutput, []ReceivedMessage, error) {
	out, err := q.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            &q.queueURL,
		MaxNumberOfMessages: maxMessages,
		WaitTimeSeconds:     waitSeconds,
		VisibilityTimeout:   120, // 2 minutes to process before message becomes visible again
	})
	if err != nil {
		return nil, nil, fmt.Errorf("sqs receive: %w", err)
	}

	var msgs []ReceivedMessage
	for _, m := range out.Messages {
		var job model.PhotoJob
		if err := json.Unmarshal([]byte(*m.Body), &job); err != nil {
			// skip malformed messages but still delete them
			msgs = append(msgs, ReceivedMessage{
				ReceiptHandle: *m.ReceiptHandle,
				Job:           nil,
				Malformed:     true,
			})
			continue
		}
		msgs = append(msgs, ReceivedMessage{
			ReceiptHandle: *m.ReceiptHandle,
			Job:           &job,
		})
	}
	return nil, msgs, nil
}

// Delete removes a processed message from SQS.
func (q *SQSQueue) Delete(ctx context.Context, receiptHandle string) error {
	_, err := q.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      &q.queueURL,
		ReceiptHandle: &receiptHandle,
	})
	if err != nil {
		return fmt.Errorf("sqs delete: %w", err)
	}
	return nil
}

// ReceivedMessage wraps a decoded SQS message.
type ReceivedMessage struct {
	ReceiptHandle string
	Job           *model.PhotoJob
	Malformed     bool
}
