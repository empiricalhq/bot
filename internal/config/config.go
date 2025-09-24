package config

import (
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

	fmt.Printf("[DEBUG]: Configuración cargada: LogLevel=%s, S3Bucket=%s\n", cfg.LogLevel, cfg.S3FlowBucket)

	return cfg, nil
}
