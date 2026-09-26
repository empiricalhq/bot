package service

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"whatsbot/internal/config"
	"whatsbot/internal/logger"
	"whatsbot/internal/platform/database"
	"whatsbot/internal/platform/whatsapp"
	"whatsbot/internal/repository"
	"whatsbot/internal/template"
)

const shutdownTimeout = 30 * time.Second

// ControllerCallbacks defines hooks the BotController can call
// to notify the host (GUI, CLI, etc.) about events like QR codes or messages.
type ControllerCallbacks struct {
	OnQRCode    func(qrCode string) // fired when a login QR code is generated
	OnConnected func()              // fired when the bot successfully connects to WhatsApp
	OnMessage   OnMessageFunc       // fired for each inbound/outbound message
}

// BotController owns the full lifecycle of the WhatsApp bot:
// config loading, logging, DB, WhatsApp client, FSM, actions, and shutdown.
type BotController struct {
	cfg        *config.Config
	logger     *slog.Logger
	logFile    io.Closer
	db         *sql.DB
	waClient   *whatsapp.Client
	shutdownCh chan struct{}
}

// NewController loads config, sets up logging, and returns a ready BotController.
// Extra slog.Handlers (e.g. GUI handler) can be injected into the logger.
func NewController(extraHandlers ...slog.Handler) (*BotController, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	logger, logFile, err := logger.New(cfg.LogLevel, extraHandlers...)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	slog.SetDefault(logger)

	return &BotController{
		cfg:        cfg,
		logger:     logger,
		logFile:    logFile,
		shutdownCh: make(chan struct{}),
	}, nil
}

// Config exposes the loaded config for external use.
func (c *BotController) Config() *config.Config {
	return c.cfg
}

// Start initializes all components (DB, repo, FSM, actions, renderer, WA client),
// logs in (via QR if needed), attaches event handlers, and blocks until shutdown signal.
// Returns error if init fails at any step.
func (c *BotController) Start(ctx context.Context, callbacks ControllerCallbacks) error {
	c.logger.Info("Starting bot", "env", c.cfg.Environment)

	db, err := database.NewSQLite(ctx, c.cfg.SQLiteDBPath, c.logger)
	if err != nil {
		return fmt.Errorf("database init failed: %w", err)
	}

	c.db = db

	repo := repository.New(db, c.logger)

	err = repo.InitSchema(ctx)
	if err != nil {
		return fmt.Errorf("schema init failed: %w", err)
	}

	waLogger := c.logger.With("component", "whatsmeow")

	waClient, err := whatsapp.NewClient(ctx, db, waLogger)
	if err != nil {
		return fmt.Errorf("WhatsApp client init failed: %w", err)
	}

	c.waClient = waClient

	flow, err := LoadFlow(c.cfg.FlowFilePath)
	if err != nil {
		return fmt.Errorf("flow load failed: %w", err)
	}

	fsm := NewFSM(flow, c.logger)
	actions := NewActionHandler(c.logger, waClient, c.cfg.VoucherPath)
	renderer := template.NewRenderer(c.logger)
	bot := NewBot(c.cfg, repo, fsm, actions, renderer, waClient, c.logger, callbacks.OnMessage)

	err = c.connect(waClient, callbacks)
	if err != nil {
		return err
	}

	waClient.AddEventHandler(bot.HandleEvent)

	c.waitForShutdown(ctx)

	return nil
}

// Shutdown attempts a graceful stop:
// - signals Start loop to exit
// - disconnects WhatsApp client (with timeout)
// - closes DB and log file
// - logs shutdown progress.
func (c *BotController) Shutdown(ctx context.Context) {
	c.logger.Info("Shutting down...")

	close(c.shutdownCh)

	shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()

	done := make(chan struct{})

	go func() {
		if c.waClient != nil {
			c.waClient.Disconnect()
		}

		close(done)
	}()

	select {
	case <-done:
		c.logger.Info("WhatsApp client disconnected")
	case <-shutdownCtx.Done():
		c.logger.Warn("Shutdown timeout while disconnecting WhatsApp client")
	}

	if c.db != nil {
		err := c.db.Close()
		if err != nil {
			c.logger.Error("Failed to close database", "error", err)
		}
	}

	if c.logFile != nil {
		err := c.logFile.Close()
		if err != nil {
			c.logger.Error("Failed to close log file", "error", err)
		}
	}

	c.logger.Info("Shutdown completed")
}

// connect logs in with a QR code when the device is not paired yet, and otherwise reconnects.
// It calls OnConnected once connected.
func (c *BotController) connect(waClient *whatsapp.Client, callbacks ControllerCallbacks) error {
	if waClient.Store.ID == nil {
		err := whatsapp.LoginWithQR(waClient.Client, c.logger, callbacks.OnQRCode)
		if err != nil {
			return fmt.Errorf("QR login failed: %w", err)
		}
	} else {
		err := waClient.Connect()
		if err != nil {
			return fmt.Errorf("connection failed: %w", err)
		}

		c.logger.Info("Connection successful")
	}

	if callbacks.OnConnected != nil {
		callbacks.OnConnected()
	}

	return nil
}

// waitForShutdown blocks until an interrupt signal, a Shutdown call or ctx is cancelled.
func (c *BotController) waitForShutdown(ctx context.Context) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigChan:
		c.logger.Info("Received interrupt signal")
	case <-c.shutdownCh:
		c.logger.Info("Received shutdown request")
	case <-ctx.Done():
		c.logger.Info("Context cancelled")
	}
}
