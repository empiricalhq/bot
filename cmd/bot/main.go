package main

import (
	"context"
	"fmt"
	"os"

	"whatsbot/internal/app"
	"whatsbot/internal/config"
	"whatsbot/internal/handler"
	"whatsbot/internal/logger"
	"whatsbot/internal/message"
	"whatsbot/internal/platform/client"
	"whatsbot/internal/platform/database"
	"whatsbot/internal/repository"
	"whatsbot/internal/service"
	"whatsbot/internal/templates"
	"whatsbot/internal/util"
)

func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. Config & Logger
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	logFactory, logFile, err := logger.NewFactory(logger.Config{Level: cfg.LogLevel})
	if err != nil {
		return fmt.Errorf("failed to create logger factory: %w", err)
	}

	defer func() {
		closeErr := logFile.Close()
		if closeErr != nil {
			fmt.Fprintf(os.Stderr, "ERROR: failed to close log file: %v\n", closeErr)
		}
	}()

	appLogger := logFactory.GetLogger("app")
	appLogger.Info("WhatsApp bot starting up", map[string]interface{}{"env": cfg.Environment})

	// 2. Platform (DB, WA Client)
	db, err := database.NewSQLite(context.Background(), cfg.SQLiteDBPath, appLogger)
	if err != nil {
		return fmt.Errorf("database initialization failed: %w", err)
	}
	defer db.Close()

	waClient, err := client.NewWhatsApp(context.Background(), db, logFactory)
	if err != nil {
		return fmt.Errorf("whatsapp client initialization failed: %w", err)
	}

	// 3. Core Dependencies (Repository, Services, etc.)
	botRepo, err := repository.NewBotRepository(context.Background(), db, logFactory.GetLogger("repo"))
	if err != nil {
		return fmt.Errorf("repository initialization failed: %w", err)
	}

	err = botRepo.InitSchema(context.Background())
	if err != nil {
		return fmt.Errorf("failed to initialize database schema: %w", err)
	}

	flow, err := service.LoadFlow(cfg.FlowFilePath)
	if err != nil {
		return fmt.Errorf("failed to load conversation flow: %w", err)
	}

	appLogger.Info("Conversation flow loaded and validated", nil)

	actionHandler := service.NewActionHandler(logFactory.GetLogger("action"))
	renderer := templates.NewTextRenderer()
	regexCache := util.NewRegexCache(128)
	fsmEngine := service.NewFsmEngine(flow, regexCache, logFactory.GetLogger("fsm"))
	sender := message.NewSender(waClient)
	botSvc := service.NewBotService(botRepo, fsmEngine, actionHandler, renderer, sender, logFactory.GetLogger("service"))

	// 4. Application
	whatsAppHandler := handler.NewWhatsApp(cfg, botSvc, logFactory.GetLogger("handler"))
	botApp := app.New(appLogger, waClient, whatsAppHandler)

	// 5. Run
	if err := botApp.Run(); err != nil {
		return fmt.Errorf("application runtime error: %w", err)
	}

	appLogger.Info("Bot shut down successfully", nil)

	return nil
}
