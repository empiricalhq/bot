package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mdp/qrterminal/v3"
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

var (
	ErrQRLoginTimeout           = errors.New("QR login timed out")
	ErrStartNodeEmpty           = errors.New("start_node cannot be empty")
	ErrStartNodeNotFound        = errors.New("start_node not found in nodes")
	ErrTransitionTargetNotFound = errors.New("transition to non-existent target")
)

type App struct {
	logger        *logger.Logger
	botEngine     *fsm.Engine
	messageSender *message.Sender
	client        *whatsmeow.Client
	db            *sql.DB
}

func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	logFactory, logFile, err := logger.NewFactory(logger.Config{
		Level: logger.ParseLevel(os.Getenv("BOT_LOG_LEVEL")),
	})
	if err != nil {
		return fmt.Errorf("failed to create logger factory: %w", err)
	}

	defer func() {
		err := logFile.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: failed to close log file: %v\n", err)
		}
	}()

	appLogger := logFactory.GetLogger("app")
	appLogger.Info("Starting WhatsApp bot", nil)

	cfg, err := config.Load()
	if err != nil {
		appLogger.Error("Configuration load failed", map[string]interface{}{"error": err.Error()})

		return fmt.Errorf("failed to load configuration: %w", err)
	}

	appLogger.Info("Configuration loaded", map[string]interface{}{"flow_file": cfg.FlowFilePath})

	flow, err := loadConversationFlow(cfg.FlowFilePath)
	if err != nil {
		appLogger.Error("Flow load failed", map[string]interface{}{"error": err.Error()})

		return fmt.Errorf("failed to load conversation flow: %w", err)
	}

	err = validateFlow(flow)
	if err != nil {
		appLogger.Error("Flow validation failed", map[string]interface{}{"error": err.Error()})

		return fmt.Errorf("invalid conversation flow: %w", err)
	}

	appLogger.Info("Conversation flow loaded and validated", nil)

	ctx, cancel := context.WithTimeout(context.Background(), dbPingTimeout)
	defer cancel()

	sqlDB, err := initDatabase(ctx, cfg.SQLiteDBPath, appLogger)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	defer func() {
		closeErr := sqlDB.Close()
		if closeErr != nil {
			appLogger.Error("Database close error", map[string]interface{}{"error": closeErr.Error()})
		}
	}()

	client, err := initWhatsAppClient(sqlDB, logFactory)
	if err != nil {
		return fmt.Errorf("failed to initialize WhatsApp client: %w", err)
	}

	app, err := initApp(sqlDB, flow, client, logFactory)
	if err != nil {
		return fmt.Errorf("failed to initialize application: %w", err)
	}

	err = app.start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start application: %w", err)
	}

	return app.waitForShutdown()
}

func loadConversationFlow(path string) (*fsm.Flow, error) {
	cleanPath := filepath.Clean(path)

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read flow file %s: %w", path, err)
	}

	var flow fsm.Flow

	err = json.Unmarshal(data, &flow)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal flow: %w", err)
	}

	return &flow, nil
}

func validateFlow(flow *fsm.Flow) error {
	if flow.StartNode == "" {
		return ErrStartNodeEmpty
	}

	if _, exists := flow.Nodes[flow.StartNode]; !exists {
		return fmt.Errorf("%w: '%s'", ErrStartNodeNotFound, flow.StartNode)
	}

	// validate all transition targets exist
	for nodeID, node := range flow.Nodes {
		for _, transition := range node.Transitions {
			if _, exists := flow.Nodes[transition.Target]; !exists {
				return fmt.Errorf("%w: from '%s' to '%s'", ErrTransitionTargetNotFound, nodeID, transition.Target)
			}
		}
	}

	return nil
}

func initDatabase(ctx context.Context, dbPath string, log *logger.Logger) (*sql.DB, error) {
	dsn := dbPath + "?_pragma=journal_mode=WAL&_pragma=busy_timeout=30000&_pragma=foreign_keys=ON"

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	sqlDB.SetMaxOpenConns(dbMaxOpenConns)
	sqlDB.SetMaxIdleConns(dbMaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Hour)

	err = sqlDB.PingContext(ctx)
	if err != nil {
		closeErr := sqlDB.Close()
		if closeErr != nil {
			log.Warn("Failed to close database after ping failure", map[string]interface{}{"error": closeErr.Error()})
		}

		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info("Database initialized", map[string]interface{}{"path": dbPath})

	return sqlDB, nil
}

func initWhatsAppClient(sqlDB *sql.DB, logFactory *logger.Factory) (*whatsmeow.Client, error) {
	container := sqlstore.NewWithDB(
		sqlDB,
		"sqlite3",
		logger.NewWhatsmeowLogger(logFactory.GetLogger("sqlstore"), "sqlstore"),
	)

	err := container.Upgrade(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade whatsmeow database schema: %w", err)
	}

	device, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get device from store: %w", err)
	}

	whatsmeowLogger := logger.NewWhatsmeowLogger(logFactory.GetLogger("whatsmeow"), "whatsmeow")
	client := whatsmeow.NewClient(device, whatsmeowLogger)

	return client, nil
}

func initApp(sqlDB *sql.DB, flow *fsm.Flow, client *whatsmeow.Client, logFactory *logger.Factory) (*App, error) {
	userManager, err := state.NewSQLiteManager(sqlDB, logFactory.GetLogger("state"))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize state manager: %w", err)
	}

	actionHandler := actions.NewHandler(userManager, logFactory.GetLogger("actions"))
	renderer := templates.NewTextRenderer()
	messageSender := message.NewSender(client)
	botEngine := fsm.NewEngine(flow, userManager, actionHandler, renderer, logFactory.GetLogger("fsm"))

	return &App{
		logger:        logFactory.GetLogger("app"),
		botEngine:     botEngine,
		messageSender: messageSender,
		client:        client,
		db:            sqlDB,
	}, nil
}

func (a *App) start(ctx context.Context) error {
	// register event handler before connecting
	a.client.AddEventHandler(a.eventHandler)

	if a.client.Store.ID == nil {
		a.logger.Info("No device stored, initiating QR login", nil)

		return a.handleQRLogin(ctx)
	}

	a.logger.Info("Restoring existing session", nil)

	err := a.client.Connect()
	if err != nil {
		return fmt.Errorf("failed to restore existing session: %w", err)
	}

	return nil
}

func (a *App) handleQRLogin(ctx context.Context) error {
	qrChan, err := a.client.GetQRChannel(ctx)
	if err != nil {
		return fmt.Errorf("failed to get QR channel: %w", err)
	}

	err = a.client.Connect()
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	for evt := range qrChan {
		switch evt.Event {
		case "code":
			a.logger.Info("Rendering QR code in terminal", nil)
			qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)

		case "success":
			a.logger.Info("QR login successful", nil)

			return nil

		case "timeout":
			a.logger.Warn("QR login timed out", nil)

			return ErrQRLoginTimeout
		}
	}

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

		return fmt.Errorf("shutdown timed out: %w", ctx.Err())
	}
}

func (a *App) eventHandler(evt interface{}) {
	switch event := evt.(type) {
	case *events.Message:
		if event.Info.IsGroup {
			return
		}

		a.handleMessage(event)

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

		sendErr := a.messageSender.SendText(ctx, msg.Recipient,
			"Disculpa, hubo un error procesando tu mensaje. Por favor intenta de nuevo en unos moments.")
		if sendErr != nil {
			a.logger.Error("Failed to send error message", map[string]interface{}{"error": sendErr.Error()})
		}

		return
	}

	err = a.messageSender.SendText(ctx, msg.Recipient, response)
	if err != nil {
		a.logger.Error("Failed to send response", map[string]interface{}{
			"error":     err.Error(),
			"recipient": senderID,
		})
	}
}
