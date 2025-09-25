package config

import (
	"errors"
	"fmt"
	"os"

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
	if err := godotenv.Load(); err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("Warning: Failed to load .env file: %v\n", err)
		}
	}

	cfg := &Config{
		LogLevel:     logger.ParseLevel(utils.GetEnv("BOT_LOG_LEVEL", "INFO")),
		FlowFilePath: utils.GetEnv("BOT_FLOW_FILE_PATH", "conversation.json"),
		SQLiteDBPath: utils.GetEnv("BOT_SQLITE_DB_PATH", "store.db"),
	}

	if err := cfg.validate(); err != nil {
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

	if _, err := os.Stat(c.FlowFilePath); os.IsNotExist(err) {
		return fmt.Errorf("flow file not found: %s", c.FlowFilePath)
	}

	return nil
}
