package config

import (
	"errors"
	"fmt"

	"github.com/joho/godotenv"

	"whatsbot/internal/logger"
	"whatsbot/pkg/utils"
)

var (
	ErrMissingFlowFile = errors.New("BOT_FLOW_FILE_PATH is required")
	ErrMissingDBPath   = errors.New("BOT_SQLITE_DB_PATH is required")
)

type Config struct {
	LogLevel     logger.Level
	FlowFilePath string
	SQLiteDBPath string
}

func Load() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		// This is not a fatal error, as env vars could be set directly.
		fmt.Printf("Info: No .env file found or failed to load: %v\n", err)
	}

	cfg := &Config{
		LogLevel:     logger.ParseLevel(utils.GetEnv("BOT_LOG_LEVEL", "INFO")),
		FlowFilePath: utils.GetEnv("BOT_FLOW_FILE_PATH", "conversation.json"),
		SQLiteDBPath: utils.GetEnv("BOT_SQLITE_DB_PATH", "store.db"),
	}

	if err = cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.FlowFilePath == "" {
		return ErrMissingFlowFile
	}
	if c.SQLiteDBPath == "" {
		return ErrMissingDBPath
	}

	return nil
}
