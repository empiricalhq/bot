//go:build gui

package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"time"
	"whatsbot/gui"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"whatsbot/internal/config"
	"whatsbot/internal/logger"
	"whatsbot/internal/platform/database"
	"whatsbot/internal/platform/whatsapp"
	"whatsbot/internal/repository"
	"whatsbot/internal/service"
	"whatsbot/internal/template"
)

//go:embed all:../../gui/frontend/dist
var assets embed.FS

// GUILogWriter is a custom writer to forward logs to the frontend
type GUILogWriter struct {
	ctx context.Context
}

func (w *GUILogWriter) Write(p []byte) (n int, err error) {
	runtime.EventsEmit(w.ctx, "log:new", string(p))
	return len(p), nil
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create a context that we can use for the GUI logger
	guiCtx := context.Background()
	guiLogWriter := &GUILogWriter{ctx: guiCtx}

	// We'll create a multi-writer to log to both file and our GUI
	logFile, err := createLogFile()
	if err != nil {
		log.Fatalf("Failed to create log file: %v", err)
	}
	defer logFile.Close()

	multiWriter := io.MultiWriter(logFile, guiLogWriter)

	logLevel := new(slog.LevelVar)
	logLevel.Set(logger.ParseLevel(cfg.LogLevel))
	handler := slog.NewTextHandler(multiWriter, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(handler))

	app := gui.NewApp(slog.Default())

	err = wails.Run(&options.App{
		Title:  "Bot control panel",
		Width:  1280,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			// Now that the Wails context is ready, update our log writer
			guiLogWriter.ctx = ctx
			// Start the bot in a goroutine
			go runBot(ctx, app, cfg)
		},
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}

// runBot contains the core logic from the original main function
func runBot(ctx context.Context, guiApp *gui.App, cfg *config.Config) {
	logger := slog.Default()
	logger.Info("Starting bot in GUI mode", "env", cfg.Environment)

	// Pass a function to LoginWithQR to emit the QR code to the frontend
	whatsapp.OnQR = func(qrString string) {
		runtime.EventsEmit(ctx, "qr:update", qrString)
	}

	// Create an event emitter for chat messages
	chatEmitter := func(sender, message string) {
		runtime.EventsEmit(ctx, "chat:new", map[string]string{
			"sender":  sender,
			"message": message,
		})
	}

	db, err := database.NewSQLite(ctx, cfg.SQLiteDBPath, logger)
	if err != nil {
		logger.Error("Database init failed", "error", err)
		return
	}
	defer db.Close()

	repo := repository.New(db, logger)
	repo.InitSchema(ctx)

	waClient, err := whatsapp.NewClient(ctx, db, logger)
	if err != nil {
		logger.Error("WhatsApp client init failed", "error", err)
		return
	}

	flow, _ := service.LoadFlow(cfg.FlowFilePath)
	fsm := service.NewFSM(flow, logger)
	actions := service.NewActionHandler(logger, waClient, cfg.VoucherPath)
	renderer := template.NewRenderer(logger)

	bot := service.NewBot(cfg, repo, fsm, actions, renderer, waClient, logger, chatEmitter)
	guiApp.SetBot(bot)

	if waClient.Store.ID == nil {
		runtime.EventsEmit(ctx, "status:update", "Waiting for QR Scan...")
		err = whatsapp.LoginWithQR(waClient.Client, logger)
		if err != nil {
			logger.Error("QR login failed", "error", err)
			runtime.EventsEmit(ctx, "status:update", fmt.Sprintf("Login Failed: %v", err))
			return
		}
	} else {
		runtime.EventsEmit(ctx, "status:update", "Connecting...")
		err = waClient.Connect()
		if err != nil {
			logger.Error("Connection failed", "error", err)
			runtime.EventsEmit(ctx, "status:update", fmt.Sprintf("Connection Failed: %v", err))
			return
		}
	}
	runtime.EventsEmit(ctx, "status:update", "Connected!")
	runtime.EventsEmit(ctx, "qr:hide", true)

	waClient.AddEventHandler(bot.HandleEvent)

	// Keep bot running until app quits
	<-ctx.Done()
	logger.Info("Shutdown signal received, disconnecting bot...")
	waClient.Disconnect()
}

func createLogFile() (*os.File, error) {
	if err := os.MkdirAll("log", 0o755); err != nil {
		return nil, fmt.Errorf("could not create log directory: %w", err)
	}
	fileName := fmt.Sprintf("log/bot-gui_%s.log", time.Now().Format("2006-01-02T15-04-05"))
	return os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
}
