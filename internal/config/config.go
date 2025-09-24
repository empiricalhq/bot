package config

import (
	"errors"
	"fmt"
	"whatsbot/pkg/utils"

	"github.com/joho/godotenv"
)

type Config struct {
	LogLevel     string
	S3FlowBucket string
}

func Load() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, fmt.Errorf("error loading .env file: %v", err)
	}

	cfg := &Config{
		LogLevel:     utils.GetEnv("BOT_LOG_LEVEL", "INFO"),
		S3FlowBucket: utils.GetEnv("BOT_S3_BUCKET", ""),
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.S3FlowBucket == "" {
		return errors.New("BOT_S3_BUCKET is required")
	}
	return nil
}
