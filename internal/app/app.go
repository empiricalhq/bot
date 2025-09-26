package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.mau.fi/whatsmeow"

	"whatsbot/internal/handler"
	"whatsbot/internal/logger"
	"whatsbot/internal/platform/client"
)

const shutdownTimeout = 30 * time.Second

var ErrQRLoginTimeout = errors.New("QR login timed out")

type App struct {
	logger  *logger.Logger
	client  *whatsmeow.Client
	handler *handler.WhatsApp
}

func New(
	log *logger.Logger,
	client *whatsmeow.Client,
	whatsAppHandler *handler.WhatsApp,
) *App {
	return &App{
		logger:  log,
		client:  client,
		handler: whatsAppHandler,
	}
}

func (a *App) Run() error {
	a.client.AddEventHandler(a.handler.EventHandler)

	if a.client.Store.ID == nil {
		a.logger.Info("No device stored, initiating QR login", nil)

		err := client.LoginWithQR(a.client, a.logger, ErrQRLoginTimeout)
		if err != nil {
			return fmt.Errorf("QR login failed: %w", err)
		}
	} else {
		a.logger.Info("Restoring existing session", nil)

		err := a.client.Connect()
		if err != nil {
			return fmt.Errorf("failed to restore existing session: %w", err)
		}
	}

	return a.waitForShutdown()
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
