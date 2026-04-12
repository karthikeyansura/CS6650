package blob

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Client struct {
	client   *s3.Client
	uploader *manager.Uploader
	bucket   string
	region   string
}

func NewS3Client(client *s3.Client, bucket, region string) *S3Client {
	uploader := manager.NewUploader(client, func(u *manager.Uploader) {
		// 10MB part size for multipart uploads
		u.PartSize = 10 * 1024 * 1024
		// upload parts concurrently
		u.Concurrency = 5
	})
	return &S3Client{
		client:   client,
		uploader: uploader,
		bucket:   bucket,
		region:   region,
	}
}

// Upload streams a reader to S3 using the multipart upload manager.
// Does not buffer the entire file in memory.
func (s *S3Client) Upload(ctx context.Context, key string, body io.Reader, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err := s.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      &s.bucket,
		Key:         &key,
		Body:        body,
		ContentType: &contentType,
	})
	if err != nil {
		return fmt.Errorf("s3 upload %s: %w", key, err)
	}
	return nil
}

// Delete removes an object from S3. Treats a missing object as success
// because S3 DeleteObject is idempotent and returns 204 even if key does not exist.
func (s *S3Client) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
	if err != nil {
		return fmt.Errorf("s3 delete %s: %w", key, err)
	}
	return nil
}

// ObjectURL returns the public URL for an S3 object.
// Requires the bucket to have a public read policy or the object to have public-read ACL.
func (s *S3Client) ObjectURL(key string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, key)
}

// HeadObject checks whether an object exists in S3.
func (s *S3Client) HeadObject(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return false, nil
	}
	return true, nil
}
