package config

import (
	"errors"
	"fmt"

	"github.com/joho/godotenv"
	"whatsbot/internal/logger"
	"whatsbot/pkg/utils"
)

var (
	ErrMissingS3Bucket     = errors.New("BOT_FSM_S3_BUCKET is required")
	ErrMissingUserTable    = errors.New("BOT_DYNAMODB_USER_TABLE is required")
	ErrMissingHistoryTable = errors.New("BOT_DYNAMODB_HISTORY_TABLE is required")
	ErrMissingSessionTable = errors.New("BOT_DYNAMODB_SESSION_TABLE is required")
	ErrMissingSessionID    = errors.New("BOT_SESSION_ID is required")
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
	err := godotenv.Load()
	if err != nil {
		fmt.Printf("No .env file found or failed to load: %v\n", err)
	}

	cfg := &Config{
		LogLevel:         logger.ParseLevel(utils.GetEnv("BOT_LOG_LEVEL", "INFO")),
		S3FlowBucket:     utils.GetEnv("BOT_FSM_S3_BUCKET", ""),
		S3FlowKey:        utils.GetEnv("BOT_FSM_S3_KEY", "conversation.json"),
		UserTableName:    utils.GetEnv("BOT_DYNAMODB_USER_TABLE", "WhatsbotUserState"),
		HistoryTableName: utils.GetEnv("BOT_DYNAMODB_HISTORY_TABLE", "WhatsbotConversationHistory"),
		SessionTableName: utils.GetEnv("BOT_DYNAMODB_SESSION_TABLE", "WhatsbotSession"),
		SessionID:        utils.GetEnv("BOT_SESSION_ID", "primary-bot-session"),
	}

	err = cfg.validate()
	if err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.S3FlowBucket == "" {
		return ErrMissingS3Bucket
	}

	if c.UserTableName == "" {
		return ErrMissingUserTable
	}

	if c.HistoryTableName == "" {
		return ErrMissingHistoryTable
	}

	if c.SessionTableName == "" {
		return ErrMissingSessionTable
	}

	if c.SessionID == "" {
		return ErrMissingSessionID
	}

	return nil
}
