package main

import (
	"fmt"
	"os"

	"whatsbot/internal/bot"
	"whatsbot/internal/logger"
)

func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	logFactory, logFile, err := logger.NewFactory(logger.Config{
		Level: logger.ParseLevel(os.Getenv("LOG_LEVEL")),
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

	b, err := bot.New(logFactory)
	if err != nil {
		appLogger.Error("Initialization failed", map[string]interface{}{"error": err.Error()})

		return fmt.Errorf("failed to initialize bot: %w", err)
	}

	err = b.Run()
	if err != nil {
		appLogger.Error("Runtime error", map[string]interface{}{"error": err.Error()})

		return fmt.Errorf("bot execution failed: %w", err)
	}

	appLogger.Info("Bot shut down successfully", nil)

	return nil
}
