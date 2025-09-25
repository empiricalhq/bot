package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"

	"whatsbot/internal/actions"
	"whatsbot/internal/config"
	"whatsbot/internal/fsm"
	"whatsbot/internal/logger"
	"whatsbot/internal/message"
	"whatsbot/internal/state"
	"whatsbot/internal/templates"
)

type App struct {
	logger        *logger.Logger
	botEngine     *fsm.Engine
	messageSender *message.Sender
	client        *whatsmeow.Client
}

func main() {
	// Initialize context and logger
	ctx := context.Background()
	logFactory, err := logger.NewFactory(logger.Config{Level: logger.ParseLevel(os.Getenv("BOT_LOG_LEVEL"))})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to create logger factory: %v\n", err)
		os.Exit(1)
	}
	appLogger := logFactory.GetLogger("WhatsbotApp")

	// Load configuration from .env file
	cfg, err := config.Load()
	if err != nil {
		appLogger.Error("Failed to load configuration", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	appLogger.Info("Configuration loaded successfully", nil)

	// Load conversation flow from local file
	flow, err := loadConversationFlow(cfg.FlowFilePath)
	if err != nil {
		appLogger.Error("Failed to load conversation flow", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	appLogger.Info("Conversation flow loaded", map[string]interface{}{"path": cfg.FlowFilePath})

	// Initialize SQLite database and session store
	db, err := sql.Open("sqlite3", cfg.SQLiteDBPath)
	if err != nil {
		appLogger.Error("Failed to open SQLite database", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	container, err := sqlstore.NewWithDB(db, "sqlite3", logFactory.GetLogger("SQLStore"))
	if err != nil {
		appLogger.Error("Failed to create SQL session store", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	device, err := container.GetFirstDevice()
	if err != nil {
		appLogger.Error("Failed to get device from store", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	appLogger.Info("Database and session store initialized", map[string]interface{}{"path": cfg.SQLiteDBPath})

	// Initialize application components
	userManager, err := state.NewSQLiteManager(db, logFactory.GetLogger("StateManager"))
	if err != nil {
		appLogger.Error("Failed to initialize state manager", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	actionHandler := actions.NewHandler(userManager, logFactory.GetLogger("ActionHandler"))
	renderer := templates.NewTextRenderer()
	whatsmeowLogger := logger.NewWhatsmeowLogger(logFactory.GetLogger("WhatsmeowClient"), "whatsmeow")
	client := whatsmeow.NewClient(device, whatsmeowLogger)
	messageSender := message.NewSender(client)
	botEngine := fsm.NewEngine(flow, userManager, actionHandler, renderer, logFactory.GetLogger("FSMEngine"))

	app := &App{
		logger:        appLogger,
		botEngine:     botEngine,
		messageSender: messageSender,
		client:        client,
	}

	// Register event handler
	client.AddEventHandler(app.eventHandler)

	// Connect to WhatsApp
	err = client.Connect()
	if err != nil {
		appLogger.Error("Failed to connect WhatsApp client", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		appLogger.Info("Shutting down...", nil)
		client.Disconnect()
		_ = db.Close()
		os.Exit(0)
	}()

	appLogger.Info("WhatsApp bot is running. Press CTRL+C to exit.", nil)
	// Block forever
	select {}
}

func (a *App) eventHandler(evt interface{}) {
	ctx := context.Background()
	switch event := evt.(type) {
	case *events.Message:
		// Ignore group messages early
		if event.Info.IsGroup {
			return
		}
		a.handleMessage(ctx, event)
	case *events.QR:
		a.logger.Info("QR code received. Scan with WhatsApp.", nil)
		go func() {
			for code := range event.Codes {
				a.logger.Warn("QR code update. Please scan.", map[string]interface{}{"code": code})
			}
			a.logger.Info("QR channel closed.", nil)
		}()
	case *events.Connected:
		a.logger.Info("WhatsApp client connected", nil)
	case *events.Disconnected:
		a.logger.Warn("WhatsApp client disconnected.", nil)
	}
}

func (a *App) handleMessage(ctx context.Context, evt *events.Message) {
	msg := message.New(evt)
	// This check is slightly redundant because of the IsGroup check in eventHandler,
	// but it's good practice to ensure the message object is valid.
	if msg == nil {
		return
	}

	a.logger.Info("Processing message", map[string]interface{}{"from": msg.GetSenderID()})

	response, err := a.botEngine.ProcessMessage(ctx, msg)
	if err != nil {
		a.logger.Error("Failed to process message", map[string]interface{}{
			"error":  err.Error(),
			"sender": msg.GetSenderID(),
		})

		errSend := a.messageSender.SendText(ctx, msg.Recipient, "Disculpa, hubo un error. Por favor intenta de nuevo.")
		if errSend != nil {
			a.logger.Error("Failed to send error message to user", map[string]interface{}{"error": errSend.Error()})
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

func loadConversationFlow(path string) (*fsm.Flow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read flow file: %w", err)
	}

	var flow fsm.Flow
	err = json.Unmarshal(data, &flow)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal flow: %w", err)
	}

	return &flow, nil
}
