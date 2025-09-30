package main

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"whatsbot/internal/service"
)

// GuiLogHandler forwards slog logs to the Wails frontend via events.
type GuiLogHandler struct {
	ctx   context.Context
	attrs []slog.Attr
	group string
}

// Enabled always returns true, letting the log level be controlled by the logger itself.
func (h *GuiLogHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

// Handle formats a log record and emits it as "bot:new_log" to the GUI.
func (h *GuiLogHandler) Handle(_ context.Context, r slog.Record) error {
	var builder strings.Builder
	builder.WriteString(r.Message)

	// Add the handler's own attributes first
	for _, attr := range h.attrs {
		builder.WriteString(fmt.Sprintf(" %s=%v", attr.Key, attr.Value.Any()))
	}

	// Add the record's attributes
	r.Attrs(func(a slog.Attr) bool {
		key := a.Key
		if h.group != "" {
			key = h.group + "." + key
		}

		builder.WriteString(fmt.Sprintf(" %s=%v", key, a.Value.Any()))

		return true
	})

	logData := map[string]string{
		"level":   r.Level.String(),
		"message": builder.String(),
	}

	runtime.EventsEmit(h.ctx, "bot:new_log", logData)

	return nil
}

func (h *GuiLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandler := *h
	newHandler.attrs = append(newHandler.attrs, attrs...)

	return &newHandler
}

func (h *GuiLogHandler) WithGroup(name string) slog.Handler {
	newHandler := *h
	if newHandler.group != "" {
		newHandler.group += "."
	}

	newHandler.group += name

	return &newHandler
}

// App wires the GUI (Wails) with the bot controller.
type App struct {
	ctx        context.Context
	controller *service.BotController
}

func NewApp() *App {
	return &App{}
}

// startup initializes the bot controller and GUI logger.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	guiLogger := &GuiLogHandler{ctx: ctx}

	controller, err := service.NewController(guiLogger)
	if err != nil {
		slog.Error("Failed to initialize controller", "error", err)
		panic(err)
	}

	a.controller = controller
}

func (a *App) shutdown(ctx context.Context) {
	if a.controller != nil {
		a.controller.Shutdown(ctx)
	}
}

// StartBot launches the bot controller in a goroutine
// and forwards QR codes + incoming/outgoing messages to the GUI.
func (a *App) StartBot() {
	go func() {
		callbacks := service.ControllerCallbacks{
			OnQRCode: func(qrCode string) {
				runtime.EventsEmit(a.ctx, "bot:qr_code", qrCode)
			},
			OnConnected: func() {
				runtime.EventsEmit(a.ctx, "bot:connected")
			},
			OnMessage: func(direction, userID, userName, text string) {
				messageData := map[string]string{
					"direction": direction,
					"userID":    userID,
					"userName":  userName,
					"text":      text,
				}
				runtime.EventsEmit(a.ctx, "bot:new_message", messageData)
			},
		}

		slog.Info("GUI is starting the bot controller...")

		err := a.controller.Start(a.ctx, callbacks)
		if err != nil {
			slog.Error("Bot controller failed to start", "error", err)
			runtime.EventsEmit(a.ctx, "bot:start_failed", err.Error())
		}
	}()
}

// GetAllowedUsers returns sorted dev-only allowed users from config.
func (a *App) GetAllowedUsers() []string {
	usersMap := a.controller.Config().DevAllowedUsers

	users := make([]string, 0, len(usersMap))
	for user := range usersMap {
		users = append(users, user)
	}

	sort.Strings(users)

	return users
}

// AddAllowedUser adds a dev-only allowed user for this session.
func (a *App) AddAllowedUser(user string) {
	user = strings.TrimSpace(user)
	if user == "" {
		return
	}

	slog.Info("Adding allowed user for this session", "user", user)
	a.controller.Config().DevAllowedUsers[user] = true
}
