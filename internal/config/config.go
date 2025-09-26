package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"

	"whatsbot/pkg/utils"
)

type Config struct {
	LogLevel        string
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
		LogLevel:     utils.GetEnv("LOG_LEVEL", "INFO"),
		FlowFilePath: utils.GetEnv("FLOW_FILE_PATH", "conversation.json"),
		SQLiteDBPath: utils.GetEnv("SQLITE_DB_PATH", "store.db"),
		Environment:  strings.ToLower(utils.GetEnv("ENV", "prod")),
	}

	allowedUsers := utils.GetEnv("DEV_ALLOWED_USERS", "")
	cfg.DevAllowedUsers = parseAllowedUsers(allowedUsers)

	return cfg, cfg.validate()
}

func parseAllowedUsers(usersStr string) map[string]bool {
	allowed := make(map[string]bool)
	if usersStr == "" {
		return allowed
	}

	for _, user := range strings.Split(usersStr, ",") {
		user = strings.TrimSpace(user)
		if user != "" {
			allowed[user] = true
		}
	}

	return allowed
}

func (c *Config) validate() error {
	if c.FlowFilePath == "" {
		return errors.New("FLOW_FILE_PATH is required")
	}

	if c.SQLiteDBPath == "" {
		return errors.New("SQLITE_DB_PATH is required")
	}

	_, err := os.Stat(c.FlowFilePath)
	if os.IsNotExist(err) {
		return errors.New("flow file not found: " + c.FlowFilePath)
	}

	return nil
}
