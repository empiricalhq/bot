package logger

import (
	"fmt"
	"log"
	"os"
	"time"
)

// Config holds configuration for the logger factory.
type Config struct {
	Level Level
}

// Factory creates named Logger instances.
type Factory struct {
	config     Config
	fileLogger *log.Logger
}

const (
	logDirPerm  = 0o755 // rwxr-xr-x
	logFilePerm = 0o664 // rw-rw-r--
)

// NewFactory creates a new logger factory that writes to a timestamped file and optionally to the console.
func NewFactory(config Config) (*Factory, *os.File, error) {
	logDir := "log"
	if err := os.MkdirAll(logDir, logDirPerm); err != nil {
		return nil, nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	fileName := fmt.Sprintf("%s/bot_%s.log.json", logDir, time.Now().Format("2006-01-02T15-04-05"))

	// #nosec G304 -- File path is constructed internally, not from user input.
	logFile, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, logFilePerm)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open log file: %w", err)
	}

	factory := &Factory{
		config:     config,
		fileLogger: log.New(logFile, "", 0),
	}

	return factory, logFile, nil
}

// GetLogger returns a new logger with the specified name.
func (f *Factory) GetLogger(name string) *Logger {
	var consoleLogger *log.Logger
	// Suppress noisy internal library logs from the console.
	if name != "whatsmeow" && name != "sqlstore" {
		consoleLogger = log.New(os.Stdout, "", 0)
	}

	return NewLogger(name, f.config.Level, f.fileLogger, consoleLogger)
}
