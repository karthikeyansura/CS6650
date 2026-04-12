package model

// Album represents an album record in DynamoDB.
type Album struct {
	AlbumID     string `json:"album_id" dynamodbav:"album_id"`
	Title       string `json:"title" dynamodbav:"title"`
	Description string `json:"description" dynamodbav:"description"`
	Owner       string `json:"owner" dynamodbav:"owner"`
}

// Photo represents a photo record in DynamoDB.
type Photo struct {
	AlbumID string `json:"album_id" dynamodbav:"album_id"`
	PhotoID string `json:"photo_id" dynamodbav:"photo_id"`
	Seq     int    `json:"seq" dynamodbav:"seq"`
	Status  string `json:"status" dynamodbav:"status"`
	S3Key   string `json:"-" dynamodbav:"s3_key"`
	URL     string `json:"url,omitempty" dynamodbav:"url,omitempty"`
}

// PhotoAccepted is the 202 response returned immediately on upload.
type PhotoAccepted struct {
	PhotoID string `json:"photo_id"`
	Seq     int    `json:"seq"`
	Status  string `json:"status"`
}

// PhotoJob is the message payload enqueued to SQS.
type PhotoJob struct {
	AlbumID string `json:"album_id"`
	PhotoID string `json:"photo_id"`
	S3Key   string `json:"s3_key"`
}

// ErrorResponse is returned on 404 and 400 errors.
type ErrorResponse struct {
	Error string `json:"error"`
}

// HealthResponse is returned from GET /health.
type HealthResponse struct {
	Status string `json:"status"`
}
