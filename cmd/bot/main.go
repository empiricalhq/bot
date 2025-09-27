package main

import (
	"context"
	"log"
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
	"whatsbot/internal/service"
	"whatsbot/internal/template"
)

const shutdownTimeout = 30 * time.Second

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logger, logFile, err := logger.New(cfg.LogLevel)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logFile.Close()

	slog.SetDefault(logger)

	logger.Info("Starting bot", "env", cfg.Environment)

	database, err := database.NewSQLite(ctx, cfg.SQLiteDBPath, logger)
	if err != nil {
		log.Fatalf("Database init failed: %v", err)
	}

	defer func() {
		closeErr := database.Close()
		if closeErr != nil {
			logger.Error("Failed to close database", "error", closeErr)
		}
	}()

	repo := repository.New(database, logger)

	err = repo.InitSchema(ctx)
	if err != nil {
		log.Fatalf("Schema init failed: %v", err)
	}

	waLogger := logger.With("component", "whatsmeow")

	waClient, err := whatsapp.NewClient(ctx, database, waLogger)
	if err != nil {
		log.Fatalf("WhatsApp client init failed: %v", err)
	}

	flow, err := service.LoadFlow(cfg.FlowFilePath)
	if err != nil {
		log.Fatalf("Flow load failed: %v", err)
	}

	fsm := service.NewFSM(flow, logger)
	actions := service.NewActionHandler(logger)
	renderer := template.NewRenderer(logger)

	bot := service.NewBot(cfg, repo, fsm, actions, renderer, waClient, logger)

	if waClient.Store.ID == nil {
		err = whatsapp.LoginWithQR(waClient.Client, logger)
		if err != nil {
			log.Fatalf("QR login failed: %v", err)
		}
	} else {
		err = waClient.Connect()
		if err != nil {
			log.Fatalf("Connection failed: %v", err)
		}
	}

	waClient.AddEventHandler(bot.HandleEvent)

	// wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()

	done := make(chan struct{})

	go func() {
		waClient.Disconnect()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("Shutdown completed")
	case <-shutdownCtx.Done():
		logger.Warn("Shutdown timeout")
	}
}
