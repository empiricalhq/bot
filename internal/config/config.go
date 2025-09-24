package config

import (
	"errors"
	"fmt"

	"github.com/joho/godotenv"

	"whatsbot/internal/logger"
	"whatsbot/pkg/utils"
)

type Config struct {
	LogLevel         logger.Level
	S3FlowBucket     string
	S3FlowKey        string
	UserTableName    string
	HistoryTableName string
	SessionTableName string
	SessionID        string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		LogLevel:         logger.ParseLevel(utils.GetEnv("BOT_LOG_LEVEL", "INFO")),
		S3FlowBucket:     utils.GetEnv("BOT_FSM_S3_BUCKET", ""),
		S3FlowKey:        utils.GetEnv("BOT_FSM_S3_KEY", "conversation_flow.json"),
		UserTableName:    utils.GetEnv("BOT_DYNAMODB_USER_TABLE", "WhatsbotUserState"),
		HistoryTableName: utils.GetEnv("BOT_DYNAMODB_HISTORY_TABLE", "WhatsbotConversationHistory"),
		SessionTableName: utils.GetEnv("BOT_DYNAMODB_SESSION_TABLE", "WhatsbotSession"),
		SessionID:        utils.GetEnv("BOT_SESSION_ID", "primary-bot-session"),
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.S3FlowBucket == "" {
		return errors.New("BOT_FSM_S3_BUCKET is required")
	}
	if c.UserTableName == "" {
		return errors.New("BOT_DYNAMODB_USER_TABLE is required")
	}
	if c.HistoryTableName == "" {
		return errors.New("BOT_DYNAMODB_HISTORY_TABLE is required")
	}
	if c.SessionTableName == "" {
		return errors.New("BOT_DYNAMODB_SESSION_TABLE is required")
	}
	if c.SessionID == "" {
		return errors.New("BOT_SESSION_ID is required")
	}
	return nil
}
