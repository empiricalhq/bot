package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"

	"whatsbot/internal/actions"
	"whatsbot/internal/config"
	"whatsbot/internal/fsm"
	"whatsbot/internal/logger"
	"whatsbot/internal/message"
	"whatsbot/internal/session"
	"whatsbot/internal/state"
	"whatsbot/internal/templates"
)

type App struct {
	config        *config.Config
	logger        *logger.Logger
	botEngine     *fsm.Engine
	messageSender *message.Sender
	client        *whatsmeow.Client
	sessionStore  *session.DynamoDBStore
}

//nolint:gochecknoglobals // why: lambda requires a global handler
var app *App

func init() {
	var err error

	app, err = initializeApp(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to initialize app: %v\n", err)
		os.Exit(1)
	}
}

func initializeApp(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	logFactory, err := logger.NewFactory(logger.Config{Level: cfg.LogLevel})
	if err != nil {
		return nil, fmt.Errorf("failed to create logger factory: %w", err)
	}

	appLogger := logFactory.GetLogger("WhatsbotApp")

	// configure AWS SDK
	awsCfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS SDK config: %w", err)
	}

	// initialize AWS services
	s3Client := s3.NewFromConfig(awsCfg)
	dynamoClient := dynamodb.NewFromConfig(awsCfg)

	err = initializeDatabases(ctx, dynamoClient, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize databases: %w", err)
	}

	flow, err := loadConversationFlow(ctx, s3Client, cfg.S3FlowBucket, cfg.S3FlowKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load conversation flow: %w", err)
	}

	client, sessionStore, err := initializeWhatsAppClient(ctx, dynamoClient, cfg, logFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize WhatsApp client: %w", err)
	}

	userManager := state.NewDynamoDBManager(dynamoClient, cfg.UserTableName, cfg.HistoryTableName, logFactory.GetLogger("StateManager"))
	actionHandler := actions.NewHandler(userManager, logFactory.GetLogger("ActionHandler"))
	renderer := templates.NewTextRenderer()
	messageSender := message.NewSender(client)
	botEngine := fsm.NewEngine(flow, userManager, actionHandler, renderer, logFactory.GetLogger("FSMEngine"))

	app := &App{
		config:        cfg,
		logger:        appLogger,
		botEngine:     botEngine,
		messageSender: messageSender,
		client:        client,
		sessionStore:  sessionStore,
	}

	client.AddEventHandler(app.handleEvent)

	err = client.Connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect WhatsApp client: %w", err)
	}

	if !client.IsLoggedIn() {
		appLogger.Warn("Client not logged in. QR code scan required.", nil)
	}

	appLogger.Info("WhatsApp bot initialized successfully", nil)

	return app, nil
}

func initializeDatabases(ctx context.Context, client *dynamodb.Client, cfg *config.Config) error {
	err := state.InitTables(ctx, client, cfg.UserTableName, cfg.HistoryTableName)
	if err != nil {
		return fmt.Errorf("failed to init state tables: %w", err)
	}

	err = session.InitTable(ctx, client, cfg.SessionTableName)
	if err != nil {
		return fmt.Errorf("failed to init session table: %w", err)
	}

	return nil
}

func loadConversationFlow(ctx context.Context, s3Client *s3.Client, bucket, key string) (*fsm.Flow, error) {
	result, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get flow from S3: %w", err)
	}

	defer func() {
		err := result.Body.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to close S3 object body: %v\n", err)
		}
	}()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read flow data: %w", err)
	}

	var flow fsm.Flow

	err = json.Unmarshal(data, &flow)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal flow: %w", err)
	}

	return &flow, nil
}

func initializeWhatsAppClient(ctx context.Context, dynamoClient *dynamodb.Client, cfg *config.Config, logFactory *logger.Factory) (*whatsmeow.Client, *session.DynamoDBStore, error) {
	deviceStore := session.NewDynamoDBStore(dynamoClient, cfg.SessionTableName, cfg.SessionID, logFactory.GetLogger("SessionStore"))

	device, err := deviceStore.GetDevice(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get device from store: %w", err)
	}

	whatsmeowLogger := logger.NewWhatsmeowLogger(logFactory.GetLogger("WhatsmeowClient"), "whatsmeow")
	client := whatsmeow.NewClient(device, whatsmeowLogger)

	return client, deviceStore, nil
}

func (a *App) handleEvent(evt interface{}) {
	ctx := context.Background()

	switch event := evt.(type) {
	case *events.Message:
		a.handleMessage(ctx, event)
	case *events.QR:
		a.logger.Info("QR code received for login. Scan with WhatsApp.", nil)

		go func() {
			for code := range event.Codes {
				a.logger.Warn("QR code update", map[string]interface{}{"code": code})
			}

			a.logger.Info("QR channel closed.", nil)
		}()
	case *events.Connected:
		a.logger.Info("WhatsApp client connected", nil)

		if a.client.Store != nil {
			err := a.sessionStore.PutDevice(ctx, a.client.Store)
			if err != nil {
				a.logger.Error("Failed to save device after connection", map[string]interface{}{"error": err.Error()})
			}
		}
	case *events.Disconnected:
		a.logger.Warn("WhatsApp client disconnected", nil)
	}
}

func (a *App) handleMessage(ctx context.Context, evt *events.Message) {
	msg := message.New(evt)
	if msg == nil {
		return // Ignore group messages
	}

	a.logger.Info("Processing message", map[string]interface{}{"from": msg.GetSenderID()})

	response, err := a.botEngine.ProcessMessage(ctx, msg)
	if err != nil {
		a.logger.Error("Failed to process message", map[string]interface{}{
			"error":  err.Error(),
			"sender": msg.GetSenderID(),
		})

		err := a.messageSender.SendText(ctx, msg.Recipient, "Disculpa, hubo un error. Por favor intenta de nuevo.")
		if err != nil {
			a.logger.Error("Failed to send error message to user", map[string]interface{}{"error": err.Error()})
		}

		return
	}

	err = a.messageSender.SendText(ctx, msg.Recipient, response)
	if err != nil {
		a.logger.Error("Failed to send response", map[string]interface{}{
			"error":     err.Error(),
			"recipient": msg.GetSenderID(),
		})
	}
}

func Handler(ctx context.Context) error {
	if !app.client.IsConnected() {
		app.logger.Warn("Client disconnected, attempting reconnect", nil)

		err := app.client.Connect()
		if err != nil {
			app.logger.Error("Failed to reconnect", map[string]interface{}{"error": err.Error()})

			return fmt.Errorf("failed to reconnect client: %w", err)
		}
	}

	app.logger.Debug("Lambda keep-warm ping successful", nil)

	return nil
}

func main() {
	if os.Getenv("AWS_LAMBDA_RUNTIME_API") == "" {
		// local development mode
		app.logger.Info("Running in LOCAL mode. Press CTRL+C to exit.", nil)

		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c

		app.logger.Info("Shutting down...", nil)

		if app.client.Store != nil {
			err := app.sessionStore.PutDevice(context.Background(), app.client.Store)
			if err != nil {
				app.logger.Error("Failed to save device on shutdown", map[string]interface{}{"error": err.Error()})
			}
		}

		app.client.Disconnect()
	} else {
		lambda.Start(Handler)
	}
}
