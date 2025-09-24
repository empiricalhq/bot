package state

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"whatsbot/internal/logger"
)

type Manager interface {
	GetUserState(ctx context.Context, userID string) (*UserState, error)
	SaveUserState(ctx context.Context, state *UserState) error
	SaveMessage(ctx context.Context, msg *ConversationMessage) error
}

type DynamoDBManager struct {
	client           *dynamodb.Client
	userTableName    string
	historyTableName string
	logger           *logger.Logger
}

func NewDynamoDBManager(
	client *dynamodb.Client,
	userTableName string,
	historyTableName string,
	log *logger.Logger,
) *DynamoDBManager {
	return &DynamoDBManager{
		client:           client,
		userTableName:    userTableName,
		historyTableName: historyTableName,
		logger:           log,
	}
}

func (m *DynamoDBManager) GetUserState(ctx context.Context, userID string) (*UserState, error) {
	key, err := attributevalue.MarshalMap(map[string]string{"UserID": userID})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal UserID: %w", err)
	}

	result, err := m.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(m.userTableName),
		Key:       key,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get user state for %s: %w", userID, err)
	}

	if result.Item == nil {
		return &UserState{UserID: userID}, nil
	}

	var userState UserState
	if err = attributevalue.UnmarshalMap(result.Item, &userState); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user state for %s: %w", userID, err)
	}

	return &userState, nil
}

func (m *DynamoDBManager) SaveUserState(ctx context.Context, userState *UserState) error {
	userState.LastUpdated = time.Now()

	item, err := attributevalue.MarshalMap(userState)
	if err != nil {
		return fmt.Errorf("failed to marshal user state for %s: %w", userState.UserID, err)
	}

	_, err = m.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(m.userTableName),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("failed to save user state for %s: %w", userState.UserID, err)
	}

	m.logger.Debug("User state saved", map[string]interface{}{
		"userID":      userState.UserID,
		"currentNode": userState.CurrentNode,
	})

	return nil
}

func (m *DynamoDBManager) SaveMessage(ctx context.Context, msg *ConversationMessage) error {
	// Set TTL for 90 days
	msg.TTL = time.Now().Add(90 * 24 * time.Hour).Unix()

	item, err := attributevalue.MarshalMap(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message for %s: %w", msg.UserID, err)
	}

	_, err = m.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(m.historyTableName),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("failed to save message for %s: %w", msg.UserID, err)
	}

	m.logger.Debug("Message saved", map[string]interface{}{
		"userID":    msg.UserID,
		"direction": msg.Direction,
		"nodeID":    msg.NodeID,
	})

	return nil
}

func InitTables(ctx context.Context, client *dynamodb.Client, userTableName, historyTableName string) error {
	tables := map[string]struct {
		PK           string
		PKType       types.ScalarAttributeType
		SK           string
		SKType       types.ScalarAttributeType
		TTLEnabled   bool
		TTLAttribute string
	}{
		userTableName: {
			PK:     "UserID",
			PKType: types.ScalarAttributeTypeS,
		},
		historyTableName: {
			PK:           "UserID",
			PKType:       types.ScalarAttributeTypeS,
			SK:           "Timestamp",
			SKType:       types.ScalarAttributeTypeN,
			TTLEnabled:   true,
			TTLAttribute: "TTL",
		},
	}

	for tableName, tableConfig := range tables {
		err := createTable(ctx, client, tableName, tableConfig)
		if err != nil {
			return fmt.Errorf("failed to create table %s: %w", tableName, err)
		}
	}

	return nil
}

func createTable(ctx context.Context, client *dynamodb.Client, tableName string, config struct {
	PK           string
	PKType       types.ScalarAttributeType
	SK           string
	SKType       types.ScalarAttributeType
	TTLEnabled   bool
	TTLAttribute string
},
) error {
	input := &dynamodb.CreateTableInput{
		TableName:   aws.String(tableName),
		BillingMode: types.BillingModePayPerRequest,
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String(config.PK),
				AttributeType: config.PKType,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String(config.PK),
				KeyType:       types.KeyTypeHash,
			},
		},
	}

	if config.SK != "" {
		input.AttributeDefinitions = append(input.AttributeDefinitions, types.AttributeDefinition{
			AttributeName: aws.String(config.SK),
			AttributeType: config.SKType,
		})
		input.KeySchema = append(input.KeySchema, types.KeySchemaElement{
			AttributeName: aws.String(config.SK),
			KeyType:       types.KeyTypeRange,
		})
	}

	_, err := client.CreateTable(ctx, input)
	if err != nil {
		var resourceInUseException *types.ResourceInUseException
		if errors.As(err, &resourceInUseException) {
			return nil // Table already exists
		}

		return err
	}

	// Enable TTL if configured
	if config.TTLEnabled {
		_, err = client.UpdateTimeToLive(ctx, &dynamodb.UpdateTimeToLiveInput{
			TableName: aws.String(tableName),
			TimeToLiveSpecification: &types.TimeToLiveSpecification{
				AttributeName: aws.String(config.TTLAttribute),
				Enabled:       aws.Bool(true),
			},
		})
		if err != nil {
			return fmt.Errorf("failed to enable TTL: %w", err)
		}
	}

	return nil
}
