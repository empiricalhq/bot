package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"

	"whatsbot/internal/logger"
	"whatsbot/pkg/utils"
)

var (
	ErrMissingFlowFile  = errors.New("FLOW_FILE_PATH is required")
	ErrMissingDBPath    = errors.New("SQLITE_DB_PATH is required")
	ErrFlowFileNotFound = errors.New("flow file not found")
)

type Config struct {
	LogLevel        logger.Level
	FlowFilePath    string
	SQLiteDBPath    string
	Environment     string
	DevAllowedUsers map[string]bool
}

func Load() (*Config, error) {
	// .env is optional; only serves to override defaults
	err := godotenv.Load()
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "WARN: failed to load .env file: %v\n", err)
	}

	cfg := &Config{
		LogLevel:     logger.ParseLevel(utils.GetEnv("LOG_LEVEL", "INFO")),
		FlowFilePath: utils.GetEnv("FLOW_FILE_PATH", "conversation.json"),
		SQLiteDBPath: utils.GetEnv("SQLITE_DB_PATH", "store.db"),
		Environment:  strings.ToLower(utils.GetEnv("ENV", "prod")),
	}

	allowedUsersStr := utils.GetEnv("DEV_ALLOWED_USERS", "")
	cfg.DevAllowedUsers = parseAllowedUsers(allowedUsersStr)

	err = cfg.validate()
	if err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

func parseAllowedUsers(usersStr string) map[string]bool {
	allowed := make(map[string]bool)
	if usersStr == "" {
		return allowed
	}

	for _, user := range strings.Split(usersStr, ",") {
		trimmed := strings.TrimSpace(user)
		if trimmed != "" {
			allowed[trimmed] = true
		}
	}

	return allowed
}

func (c *Config) validate() error {
	if c.FlowFilePath == "" {
		return ErrMissingFlowFile
	}

	if c.SQLiteDBPath == "" {
		return ErrMissingDBPath
	}

	_, err := os.Stat(c.FlowFilePath)
	if os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", ErrFlowFileNotFound, c.FlowFilePath)
	}

	return nil
}
