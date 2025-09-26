package bot

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/signal"
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
	dbMaxOpenConns  = 10
	dbMaxIdleConns  = 5
)

var ErrQRLoginTimeout = errors.New("QR login timed out")

type Bot struct {
	logger        *logger.Logger
	botEngine     *fsm.Engine
	messageSender *message.Sender
	client        *whatsmeow.Client
	db            *sql.DB
}

func New(logFactory *logger.Factory) (*Bot, error) {
	appLogger := logFactory.GetLogger("app")

	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	appLogger.Info("Configuration loaded", map[string]interface{}{"flow_file": cfg.FlowFilePath})

	flow, err := fsm.Load(cfg.FlowFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load conversation flow: %w", err)
	}

	appLogger.Info("Conversation flow loaded and validated", nil)

	dbCtx, cancel := context.WithTimeout(context.Background(), dbPingTimeout)
	defer cancel()

	sqlDB, err := initDatabase(dbCtx, cfg.SQLiteDBPath, appLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	client, err := initWhatsAppClient(sqlDB, logFactory)
	if err != nil {
		sqlDB.Close()

		return nil, fmt.Errorf("failed to initialize WhatsApp client: %w", err)
	}

	userManager, err := state.NewSQLiteManager(sqlDB, logFactory.GetLogger("state"))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize state manager: %w", err)
	}

	actionHandler := actions.NewHandler(userManager, logFactory.GetLogger("actions"))
	renderer := templates.NewTextRenderer()
	messageSender := message.NewSender(client)
	botEngine := fsm.NewEngine(flow, userManager, actionHandler, renderer, logFactory.GetLogger("fsm"))

	return &Bot{
		logger:        appLogger,
		botEngine:     botEngine,
		messageSender: messageSender,
		client:        client,
		db:            sqlDB,
	}, nil
}

func (b *Bot) Run() error {
	b.client.AddEventHandler(b.eventHandler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if b.client.Store.ID == nil {
		b.logger.Info("No device stored, initiating QR login", nil)

		err := b.handleQRLogin(ctx)
		if err != nil {
			return fmt.Errorf("QR login failed: %w", err)
		}
	} else {
		b.logger.Info("Restoring existing session", nil)

		err := b.client.Connect()
		if err != nil {
			return fmt.Errorf("failed to restore existing session: %w", err)
		}
	}

	return b.waitForShutdown()
}

func (b *Bot) handleQRLogin(ctx context.Context) error {
	qrChan, err := b.client.GetQRChannel(ctx)
	if err != nil {
		return fmt.Errorf("failed to get QR channel: %w", err)
	}

	err = b.client.Connect()
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	for evt := range qrChan {
		switch evt.Event {
		case "code":
			b.logger.Info("Rendering QR code in terminal", nil)
			qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
		case "success":
			b.logger.Info("QR login successful", nil)

			return nil
		case "timeout":
			b.logger.Warn("QR login timed out", nil)

			return ErrQRLoginTimeout
		}
	}

	return nil
}

func (b *Bot) waitForShutdown() error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan
	b.logger.Info("Shutdown signal received", map[string]interface{}{"signal": sig.String()})

	return b.shutdown()
}

func (b *Bot) shutdown() error {
	b.logger.Info("Shutting down gracefully", nil)

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	done := make(chan struct{})

	go func() {
		defer close(done)

		b.client.Disconnect()

		err := b.db.Close()
		if err != nil {
			b.logger.Error("Database close error", map[string]interface{}{"error": err.Error()})
		}
	}()

	select {
	case <-done:
		b.logger.Info("Shutdown completed", nil)

		return nil
	case <-ctx.Done():
		b.logger.Warn("Shutdown timeout reached", nil)

		return fmt.Errorf("shutdown timed out: %w", ctx.Err())
	}
}

func (b *Bot) eventHandler(evt interface{}) {
	switch event := evt.(type) {
	case *events.Message:
		if event.Info.IsGroup {
			return
		}

		b.handleMessage(event)
	case *events.Connected:
		b.logger.Info("WhatsApp client connected", nil)
	case *events.Disconnected:
		b.logger.Warn("WhatsApp client disconnected", nil)
	}
}

func (b *Bot) handleMessage(evt *events.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), msgProcTimeout)
	defer cancel()

	msg := message.New(evt)
	if msg == nil {
		b.logger.Debug("Ignoring invalid message", nil)

		return
	}

	senderID := msg.GetSenderID()
	b.logger.Debug("Processing message", map[string]interface{}{
		"from": senderID,
		"text": msg.GetText(),
	})

	response, err := b.botEngine.ProcessMessage(ctx, msg)
	if err != nil {
		b.logger.Error("Message processing failed", map[string]interface{}{"error": err.Error(), "sender": senderID})

		sendErr := b.messageSender.SendText(ctx, msg.Recipient,
			"Disculpa, hubo un error procesando tu mensaje. Por favor intenta de nuevo en unos moments.")
		if sendErr != nil {
			b.logger.Error("Failed to send error message", map[string]interface{}{"error": sendErr.Error()})
		}

		return
	}

	if response == "" {
		b.logger.Debug("No response generated for message", map[string]interface{}{"from": senderID})

		return
	}

	err = b.messageSender.SendText(ctx, msg.Recipient, response)
	if err != nil {
		b.logger.Error("Failed to send response", map[string]interface{}{"error": err.Error(), "recipient": senderID})
	}
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
	container := sqlstore.NewWithDB(sqlDB, "sqlite3", logger.NewWhatsmeowLogger(logFactory.GetLogger("sqlstore"), "sqlstore"))

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
