package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/karthikeyansura/CS6650/Final/internal/model"
)

type DynamoStore struct {
	client        *dynamodb.Client
	albumsTable   string
	countersTable string
	photosTable   string
}

func NewDynamoStore(client *dynamodb.Client, albumsTable, countersTable, photosTable string) *DynamoStore {
	return &DynamoStore{
		client:        client,
		albumsTable:   albumsTable,
		countersTable: countersTable,
		photosTable:   photosTable,
	}
}

// UpsertAlbum creates or updates an album. Returns true if the item was newly created.
func (s *DynamoStore) UpsertAlbum(ctx context.Context, album *model.Album) (bool, error) {
	// attempt a conditional put that only succeeds if album_id does not exist
	item, err := attributevalue.MarshalMap(album)
	if err != nil {
		return false, fmt.Errorf("marshal album: %w", err)
	}

	_, putErr := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           &s.albumsTable,
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(album_id)"),
	})

	if putErr != nil {
		// if condition failed the album already exists so we update it
		var ccfe *types.ConditionalCheckFailedException
		if errors.As(putErr, &ccfe) {
			// update existing album
			_, updateErr := s.client.PutItem(ctx, &dynamodb.PutItemInput{
				TableName: &s.albumsTable,
				Item:      item,
			})
			if updateErr != nil {
				return false, fmt.Errorf("update album: %w", updateErr)
			}
			return false, nil
		}
		return false, fmt.Errorf("put album: %w", putErr)
	}

	return true, nil
}

// GetAlbum retrieves a single album by ID.
func (s *DynamoStore) GetAlbum(ctx context.Context, albumID string) (*model.Album, error) {
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &s.albumsTable,
		Key: map[string]types.AttributeValue{
			"album_id": &types.AttributeValueMemberS{Value: albumID},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("get album: %w", err)
	}
	if out.Item == nil {
		return nil, nil
	}

	var album model.Album
	if err := attributevalue.UnmarshalMap(out.Item, &album); err != nil {
		return nil, fmt.Errorf("unmarshal album: %w", err)
	}
	return &album, nil
}

// ListAlbums returns every album in the table using paginated Scan.
func (s *DynamoStore) ListAlbums(ctx context.Context) ([]model.Album, error) {
	var albums []model.Album
	var lastKey map[string]types.AttributeValue

	for {
		input := &dynamodb.ScanInput{
			TableName:         &s.albumsTable,
			ExclusiveStartKey: lastKey,
		}

		out, err := s.client.Scan(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("scan albums: %w", err)
		}

		var page []model.Album
		if err := attributevalue.UnmarshalListOfMaps(out.Items, &page); err != nil {
			return nil, fmt.Errorf("unmarshal albums page: %w", err)
		}
		albums = append(albums, page...)

		if out.LastEvaluatedKey == nil {
			break
		}
		lastKey = out.LastEvaluatedKey
	}

	return albums, nil
}

// AllocateSeq atomically increments and returns the next sequence number for an album.
// Uses DynamoDB UpdateItem with ADD to guarantee atomic increment under concurrency.
func (s *DynamoStore) AllocateSeq(ctx context.Context, albumID string) (int, error) {
	out, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &s.countersTable,
		Key: map[string]types.AttributeValue{
			"album_id": &types.AttributeValueMemberS{Value: albumID},
		},
		UpdateExpression: aws.String("SET next_seq = if_not_exists(next_seq, :zero) + :one"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":zero": &types.AttributeValueMemberN{Value: "0"},
			":one":  &types.AttributeValueMemberN{Value: "1"},
		},
		ReturnValues: types.ReturnValueUpdatedNew,
	})
	if err != nil {
		return 0, fmt.Errorf("allocate seq: %w", err)
	}

	seqAttr, ok := out.Attributes["next_seq"]
	if !ok {
		return 0, fmt.Errorf("allocate seq: next_seq missing from response")
	}

	var seq int
	if err := attributevalue.Unmarshal(seqAttr, &seq); err != nil {
		return 0, fmt.Errorf("unmarshal seq: %w", err)
	}

	return seq, nil
}

// CreatePhoto inserts a photo record with status processing.
func (s *DynamoStore) CreatePhoto(ctx context.Context, photo *model.Photo) error {
	item, err := attributevalue.MarshalMap(photo)
	if err != nil {
		return fmt.Errorf("marshal photo: %w", err)
	}
	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &s.photosTable,
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("put photo: %w", err)
	}
	return nil
}

// GetPhoto retrieves a photo by album_id and photo_id.
func (s *DynamoStore) GetPhoto(ctx context.Context, albumID, photoID string) (*model.Photo, error) {
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &s.photosTable,
		Key: map[string]types.AttributeValue{
			"album_id": &types.AttributeValueMemberS{Value: albumID},
			"photo_id": &types.AttributeValueMemberS{Value: photoID},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("get photo: %w", err)
	}
	if out.Item == nil {
		return nil, nil
	}

	var photo model.Photo
	if err := attributevalue.UnmarshalMap(out.Item, &photo); err != nil {
		return nil, fmt.Errorf("unmarshal photo: %w", err)
	}
	return &photo, nil
}

// CompletePhoto atomically updates a photo to completed status only if the record still exists.
// Prevents zombie records if the photo was deleted while the worker was processing.
func (s *DynamoStore) CompletePhoto(ctx context.Context, albumID, photoID, url string) error {
	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &s.photosTable,
		Key: map[string]types.AttributeValue{
			"album_id": &types.AttributeValueMemberS{Value: albumID},
			"photo_id": &types.AttributeValueMemberS{Value: photoID},
		},
		UpdateExpression:    aws.String("SET #s = :completed, #u = :url"),
		ConditionExpression: aws.String("attribute_exists(album_id)"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
			"#u": "url",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":completed": &types.AttributeValueMemberS{Value: "completed"},
			":url":       &types.AttributeValueMemberS{Value: url},
		},
	})
	if err != nil {
		return fmt.Errorf("complete photo: %w", err)
	}
	return nil
}

// FailPhoto marks a photo as failed.
func (s *DynamoStore) FailPhoto(ctx context.Context, albumID, photoID string) error {
	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &s.photosTable,
		Key: map[string]types.AttributeValue{
			"album_id": &types.AttributeValueMemberS{Value: albumID},
			"photo_id": &types.AttributeValueMemberS{Value: photoID},
		},
		UpdateExpression: aws.String("SET #s = :failed"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":failed": &types.AttributeValueMemberS{Value: "failed"},
		},
	})
	if err != nil {
		return fmt.Errorf("fail photo: %w", err)
	}
	return nil
}

// DeletePhoto removes a photo record from DynamoDB.
func (s *DynamoStore) DeletePhoto(ctx context.Context, albumID, photoID string) error {
	_, err := s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &s.photosTable,
		Key: map[string]types.AttributeValue{
			"album_id": &types.AttributeValueMemberS{Value: albumID},
			"photo_id": &types.AttributeValueMemberS{Value: photoID},
		},
	})
	if err != nil {
		return fmt.Errorf("delete photo: %w", err)
	}
	return nil
}

// IsConditionalCheckFailed returns true if the error is a DynamoDB ConditionalCheckFailedException.
func IsConditionalCheckFailed(err error) bool {
	if err == nil {
		return false
	}
	var ccfe *types.ConditionalCheckFailedException
	if errors.As(err, &ccfe) {
		return true
	}
	return strings.Contains(err.Error(), "ConditionalCheckFailedException")
}
