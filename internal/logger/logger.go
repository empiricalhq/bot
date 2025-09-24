package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

type Level int8

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
	DISABLED
)

func ParseLevel(levelStr string) Level {
	switch strings.ToUpper(strings.TrimSpace(levelStr)) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN":
		return WARN
	case "ERROR":
		return ERROR
	case "DISABLED":
		return DISABLED
	default:
		return INFO
	}
}

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case DISABLED:
		return "DISABLED"
	default:
		return "UNKNOWN"
	}
}

func (l Level) IsEnabled(level Level) bool {
	return l != DISABLED && level >= l
}

type Entry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Name      string                 `json:"name,omitempty"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

type Logger struct {
	name  string
	level Level
}

func NewLogger(name string, level Level) *Logger {
	return &Logger{
		name:  name,
		level: level,
	}
}

func (l *Logger) Debug(msg string, data map[string]interface{}) {
	if l.level.IsEnabled(DEBUG) {
		l.log(DEBUG, msg, data)
	}
}

func (l *Logger) Info(msg string, data map[string]interface{}) {
	if l.level.IsEnabled(INFO) {
		l.log(INFO, msg, data)
	}
}

func (l *Logger) Warn(msg string, data map[string]interface{}) {
	if l.level.IsEnabled(WARN) {
		l.log(WARN, msg, data)
	}
}

func (l *Logger) Error(msg string, data map[string]interface{}) {
	if l.level.IsEnabled(ERROR) {
		l.log(ERROR, msg, data)
	}
}

func (l *Logger) log(level Level, msg string, data map[string]interface{}) {
	entry := Entry{
		Timestamp: time.Now(),
		Level:     level.String(),
		Name:      l.name,
		Message:   msg,
		Data:      data,
	}

	jsonData, err := json.Marshal(entry)
	if err != nil {
		fmt.Printf("Logger marshal error: %v\n", err)
		return
	}

	log.Println(string(jsonData))
}
