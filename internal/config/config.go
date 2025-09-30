package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"

	"whatsbot/pkg/utils"
)

type Config struct {
	LogLevel        string
	FlowFilePath    string
	SQLiteDBPath    string
	Environment     string
	VoucherPath     string
	DevAllowedUsers map[string]bool
}

// findEnvFile searches for .env in the current directory and parent directories
// up to the filesystem root, returning the first found path.
func findEnvFile() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		envPath := filepath.Join(dir, ".env")
		if _, err := os.Stat(envPath); err == nil {
			return envPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			break
		}
		dir = parent
	}

	return "", os.ErrNotExist
}

func Load() (*Config, error) {
	// .env is optional; only serves to override defaults
	// Search for .env in current directory and parent directories
	envPath, err := findEnvFile()
	if err == nil {
		err = godotenv.Load(envPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: failed to load .env file from %s: %v\n", envPath, err)
		}
	}

	cfg := &Config{
		LogLevel:     utils.GetEnv("LOG_LEVEL", "INFO"),
		FlowFilePath: utils.GetEnv("FLOW_FILE_PATH", "conversation.json"),
		SQLiteDBPath: utils.GetEnv("SQLITE_DB_PATH", "store.db"),
		Environment:  strings.ToLower(utils.GetEnv("ENV", "prod")),
		VoucherPath:  utils.GetEnv("VOUCHER_SAVE_PATH", "vouchers"),
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
