package logger

import (
	"fmt"
	"log"
	"os"
	"time"
)

type Config struct {
	Level Level
}

type Factory struct {
	config     Config
	fileLogger *log.Logger
}

func NewFactory(config Config) (*Factory, *os.File, error) {
	logDir := "log"
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	fileName := fmt.Sprintf("%s/bot_%s.log.json", logDir, time.Now().Format("2006-01-02T15-04-05"))

	logFile, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o664)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &Factory{
		config:     config,
		fileLogger: log.New(logFile, "", 0),
	}, logFile, nil
}

func (f *Factory) GetLogger(name string) *Logger {
	var consoleLogger *log.Logger
	if name != "whatsmeow" && name != "sqlstore" {
		consoleLogger = log.New(os.Stdout, "", 0)
	}

	return NewLogger(name, f.config.Level, f.fileLogger, consoleLogger)
}
