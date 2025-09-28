package gui

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"whatsbot/internal/config"
	"whatsbot/internal/service"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx    context.Context
	logger *slog.Logger
	bot    *service.Bot
}

// NewApp creates a new App application struct
func NewApp(logger *slog.Logger) *App {
	return &App{
		logger: logger,
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// SetBot is used to inject the bot instance after it's been created
func (a *App) SetBot(bot *service.Bot) {
	a.bot = bot
}

// GetAllowedUsers returns the list of users from DEV_ALLOWED_USERS
func (a *App) GetAllowedUsers() []string {
	usersStr := os.Getenv("DEV_ALLOWED_USERS")
	if usersStr == "" {
		return []string{}
	}
	return strings.Split(usersStr, ",")
}

// AddAllowedUser adds a new user to the DEV_ALLOWED_USERS in the .env file
func (a *App) AddAllowedUser(number string) error {
	number = strings.TrimSpace(number)
	if number == "" {
		return fmt.Errorf("user number cannot be empty")
	}

	// For this demo, we'll just read/write the .env file directly.
	// A more robust solution might use a dedicated config management tool.
	env, err := os.ReadFile(".env")
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read .env file: %w", err)
	}

	lines := strings.Split(string(env), "\n")
	var newLines []string
	found := false
	key := "DEV_ALLOWED_USERS"

	for _, line := range lines {
		if strings.HasPrefix(line, key+"=") {
			parts := strings.SplitN(line, "=", 2)
			currentUsers := strings.TrimSpace(parts[1])
			if currentUsers == "" {
				line = fmt.Sprintf("%s=%s", key, number)
			} else {
				line = fmt.Sprintf("%s=%s,%s", key, currentUsers, number)
			}
			found = true
		}
		newLines = append(newLines, line)
	}

	if !found {
		newLines = append(newLines, fmt.Sprintf("%s=%s", key, number))
	}

	// This is a simple solution. Note that this doesn't update the running config.
	// We will emit an event so the UI can notify the user.
	if a.bot != nil {
		// In a real app, you might trigger a config reload on the bot.
		// For now, we update the file and the UI reflects the change.
		// The bot will pick it up on next restart.
	}

	runtime.EventsEmit(a.ctx, "show:toast", "User added. Bot restart required to apply changes.")

	return os.WriteFile(".env", []byte(strings.Join(newLines, "\n")), 0644)
}

// GetEnv returns the current environment (dev/prod)
func (a *App) GetEnv() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	return cfg.Environment, nil
}
