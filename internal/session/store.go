package session

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.mau.fi/whatsmeow/store"

	"whatsbot/internal/logger"
)

type SessionItem struct {
	SessionID string `dynamodbav:"SessionID"`
	Session   []byte `dynamodbav:"Session"`
}

type DynamoDBStore struct {
	client    *dynamodb.Client
	tableName string
	sessionID string
	logger    *logger.Logger
}

func NewDynamoDBStore(client *dynamodb.Client, tableName, sessionID string, log *logger.Logger) *DynamoDBStore {
	return &DynamoDBStore{
		client:    client,
		tableName: tableName,
		sessionID: sessionID,
		logger:    log,
	}
}

func (s *DynamoDBStore) GetDevice(ctx context.Context) (*store.Device, error) {
	key, err := attributevalue.MarshalMap(map[string]string{"SessionID": s.sessionID})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session ID: %w", err)
	}

	result, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key:       key,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get session from DynamoDB: %w", err)
	}

	if result.Item == nil {
		s.logger.Info("No session found, creating new device", nil)

		return new(store.Device), nil
	}

	var item SessionItem

	err = attributevalue.UnmarshalMap(result.Item, &item)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	var device store.Device

	decoder := gob.NewDecoder(bytes.NewReader(item.Session))

	decodeErr := decoder.Decode(&device)
	if decodeErr != nil {
		s.logger.Warn("Failed to decode session, returning new device", map[string]interface{}{
			"error": decodeErr.Error(),
		})

		return new(store.Device), nil
	}

	s.logger.Info("Session loaded from DynamoDB", nil)

	return &device, nil
}

func (s *DynamoDBStore) PutDevice(ctx context.Context, device *store.Device) error {
	var buf bytes.Buffer

	encoder := gob.NewEncoder(&buf)

	err := encoder.Encode(device)
	if err != nil {
		return fmt.Errorf("failed to encode session: %w", err)
	}

	item := SessionItem{
		SessionID: s.sessionID,
		Session:   buf.Bytes(),
	}

	attributeValues, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("failed to marshal session item: %w", err)
	}

	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      attributeValues,
	})
	if err != nil {
		return fmt.Errorf("failed to save session to DynamoDB: %w", err)
	}

	s.logger.Info("Session saved to DynamoDB", nil)

	return nil
}

func InitTable(ctx context.Context, client *dynamodb.Client, tableName string) error {
	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName:   aws.String(tableName),
		BillingMode: types.BillingModePayPerRequest,
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("SessionID"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("SessionID"), KeyType: types.KeyTypeHash},
		},
	})
	if err != nil {
		var resourceInUseException *types.ResourceInUseException
		if errors.As(err, &resourceInUseException) {
			return nil // table already exists
		}

		return fmt.Errorf("failed to create session table %s: %w", tableName, err)
	}

	return nil
}
