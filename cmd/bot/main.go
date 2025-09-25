package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	_ "modernc.org/sqlite"

	"whatsbot/internal/actions"
	"whatsbot/internal/config"
	"whatsbot/internal/fsm"
	"whatsbot/internal/logger"
	"whatsbot/internal/message"
	"whatsbot/internal/state"
	"whatsbot/internal/templates"
)

const (
	shutdownTimeout = 30 * time.Second
	dbPingTimeout   = 30 * time.Second
	msgProcTimeout  = 30 * time.Second

	dbMaxOpenConns = 10
	dbMaxIdleConns = 5
)

type App struct {
	logger        *logger.Logger
	botEngine     *fsm.Engine
	messageSender *message.Sender
	client        *whatsmeow.Client
	db            *sql.DB
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Initialize logger early for better error reporting
	logFactory, err := logger.NewFactory(logger.Config{
		Level: logger.ParseLevel(os.Getenv("BOT_LOG_LEVEL")),
	})
	if err != nil {
		return fmt.Errorf("failed to create logger factory: %w", err)
	}

	appLogger := logFactory.GetLogger("WhatsbotApp")
	appLogger.Info("Starting WhatsApp bot", nil)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		appLogger.Error("Configuration load failed", map[string]interface{}{"error": err.Error()})

		return fmt.Errorf("failed to load configuration: %w", err)
	}

	appLogger.Info("Configuration loaded", map[string]interface{}{"flow_file": cfg.FlowFilePath})

	// Load conversation flow
	flow, err := loadConversationFlow(cfg.FlowFilePath)
	if err != nil {
		appLogger.Error("Flow load failed", map[string]interface{}{"error": err.Error()})

		return fmt.Errorf("failed to load conversation flow: %w", err)
	}

	// Validate flow early
	if err := validateFlow(flow); err != nil {
		appLogger.Error("Flow validation failed", map[string]interface{}{"error": err.Error()})

		return fmt.Errorf("invalid conversation flow: %w", err)
	}

	appLogger.Info("Conversation flow loaded and validated", nil)

	// Initialize database with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), dbPingTimeout)
	defer cancel()

	db, err := initDatabase(ctx, cfg.SQLiteDBPath, appLogger)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			appLogger.Error("Database close error", map[string]interface{}{"error": closeErr.Error()})
		}
	}()

	// Initialize WhatsApp client
	client, err := initWhatsAppClient(db, logFactory)
	if err != nil {
		return fmt.Errorf("failed to initialize WhatsApp client: %w", err)
	}

	// Initialize application components
	app, err := initApp(db, flow, client, logFactory)
	if err != nil {
		return fmt.Errorf("failed to initialize application: %w", err)
	}

	// Start the application
	if err := app.start(ctx); err != nil {
		return fmt.Errorf("failed to start application: %w", err)
	}

	// Wait for shutdown signal
	return app.waitForShutdown()
}

func loadConversationFlow(path string) (*fsm.Flow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read flow file %s: %w", path, err)
	}

	var flow fsm.Flow
	if err := json.Unmarshal(data, &flow); err != nil {
		return nil, fmt.Errorf("failed to unmarshal flow: %w", err)
	}

	return &flow, nil
}

func validateFlow(flow *fsm.Flow) error {
	if flow.StartNode == "" {
		return fmt.Errorf("start_node cannot be empty")
	}

	if _, exists := flow.Nodes[flow.StartNode]; !exists {
		return fmt.Errorf("start_node '%s' not found in nodes", flow.StartNode)
	}

	// Validate all transition targets exist
	for nodeID, node := range flow.Nodes {
		for _, transition := range node.Transitions {
			if _, exists := flow.Nodes[transition.Target]; !exists {
				return fmt.Errorf("node '%s' has transition to non-existent target '%s'", nodeID, transition.Target)
			}
		}
	}

	return nil
}

func initDatabase(ctx context.Context, dbPath string, logger *logger.Logger) (*sql.DB, error) {
	dsn := dbPath + "?_pragma=journal_mode=WAL&_pragma=busy_timeout=30000"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(dbMaxOpenConns)
	db.SetMaxIdleConns(dbMaxIdleConns)
	db.SetConnMaxLifetime(time.Hour)

	// Test connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()

		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("Database initialized", map[string]interface{}{"path": dbPath})

	return db, nil
}

func initWhatsAppClient(db *sql.DB, logFactory *logger.Factory) (*whatsmeow.Client, error) {
	container := sqlstore.NewWithDB(
		db,
		"sqlite",
		logger.NewWhatsmeowLogger(logFactory.GetLogger("SQLStore"), "sqlstore"),
	)

	device, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get device from store: %w", err)
	}

	whatsmeowLogger := logger.NewWhatsmeowLogger(logFactory.GetLogger("WhatsmeowClient"), "whatsmeow")
	client := whatsmeow.NewClient(device, whatsmeowLogger)

	return client, nil
}

func initApp(db *sql.DB, flow *fsm.Flow, client *whatsmeow.Client, logFactory *logger.Factory) (*App, error) {
	userManager, err := state.NewSQLiteManager(db, logFactory.GetLogger("StateManager"))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize state manager: %w", err)
	}

	actionHandler := actions.NewHandler(userManager, logFactory.GetLogger("ActionHandler"))
	renderer := templates.NewTextRenderer()
	messageSender := message.NewSender(client)
	botEngine := fsm.NewEngine(flow, userManager, actionHandler, renderer, logFactory.GetLogger("FSMEngine"))

	return &App{
		logger:        logFactory.GetLogger("WhatsbotApp"),
		botEngine:     botEngine,
		messageSender: messageSender,
		client:        client,
		db:            db,
	}, nil
}

func (a *App) start(_ context.Context) error {
	// Register event handler before connecting
	a.client.AddEventHandler(a.eventHandler)

	// Connect to WhatsApp
	if err := a.client.Connect(); err != nil {
		return fmt.Errorf("failed to connect WhatsApp client: %w", err)
	}

	a.logger.Info("WhatsApp bot started successfully", nil)

	return nil
}

func (a *App) waitForShutdown() error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan
	a.logger.Info("Shutdown signal received", map[string]interface{}{"signal": sig.String()})

	return a.shutdown()
}

func (a *App) shutdown() error {
	a.logger.Info("Shutting down gracefully", nil)

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		a.client.Disconnect()
	}()

	select {
	case <-done:
		a.logger.Info("Shutdown completed", nil)

		return nil
	case <-ctx.Done():
		a.logger.Warn("Shutdown timeout reached", nil)

		return ctx.Err()
	}
}

func (a *App) eventHandler(evt interface{}) {
	switch event := evt.(type) {
	case *events.Message:
		if event.Info.IsGroup {
			return
		}
		a.handleMessage(event)

	case *events.QR:
		a.handleQRCode(event)

	case *events.Connected:
		a.logger.Info("WhatsApp client connected", nil)

	case *events.Disconnected:
		a.logger.Warn("WhatsApp client disconnected", nil)
	}
}

func (a *App) handleMessage(evt *events.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), msgProcTimeout)
	defer cancel()

	msg := message.New(evt)
	if msg == nil {
		a.logger.Debug("Ignoring invalid message", nil)

		return
	}

	senderID := msg.GetSenderID()
	a.logger.Debug("Processing message", map[string]interface{}{
		"from": senderID,
		"text": msg.GetText(),
	})

	response, err := a.botEngine.ProcessMessage(ctx, msg)
	if err != nil {
		a.logger.Error("Message processing failed", map[string]interface{}{
			"error":  err.Error(),
			"sender": senderID,
		})

		// Send user-friendly error message
		if sendErr := a.messageSender.SendText(ctx, msg.Recipient,
			"Disculpa, hubo un error procesando tu mensaje. Por favor intenta de nuevo en unos moments."); sendErr != nil {
			a.logger.Error("Failed to send error message", map[string]interface{}{"error": sendErr.Error()})
		}

		return
	}

	if err := a.messageSender.SendText(ctx, msg.Recipient, response); err != nil {
		a.logger.Error("Failed to send response", map[string]interface{}{
			"error":     err.Error(),
			"recipient": senderID,
		})
	}
}

func (a *App) handleQRCode(event *events.QR) {
	a.logger.Info("QR code received - scan with WhatsApp", nil)
	go func() {
		for code := range event.Codes {
			a.logger.Info("QR code updated", map[string]interface{}{"qr_code": code})
		}
		a.logger.Info("QR channel closed", nil)
	}()
}
