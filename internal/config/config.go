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
	ErrMissingFlowFile  = errors.New("BOT_FLOW_FILE_PATH is required")
	ErrMissingDBPath    = errors.New("BOT_SQLITE_DB_PATH is required")
	ErrFlowFileNotFound = errors.New("flow file not found")
)

type Config struct {
	LogLevel     logger.Level
	FlowFilePath string
	SQLiteDBPath string
}

func Load() (*Config, error) {
	// .env is optional; only serves to override defaults
	err := godotenv.Load()
	if err != nil && !os.IsNotExist(err) {
		// warn if .env file is not found
	}

	cfg := &Config{
		LogLevel:     logger.ParseLevel(utils.GetEnv("BOT_LOG_LEVEL", "INFO")),
		FlowFilePath: utils.GetEnv("BOT_FLOW_FILE_PATH", "conversation.json"),
		SQLiteDBPath: utils.GetEnv("BOT_SQLITE_DB_PATH", "store.db"),
	}

	err = cfg.validate()
	if err != nil {
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

	_, err := os.Stat(c.FlowFilePath)
	if os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", ErrFlowFileNotFound, c.FlowFilePath)
	}

	return nil
}
