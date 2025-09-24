package logger

import (
	"encoding/json"
	"log"
	"time"
)

type Level int8

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

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
	default:
		return "UNKNOWN"
	}
}

type Entry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

type Logger struct {
	name  string
	level Level
}

func NewLogger(name string, level Level) *Logger {
	return &Logger{name: name, level: level}
}

func (l *Logger) Info(msg string) {
	if l.level <= INFO {
		entry := Entry{
			Timestamp: time.Now().Format(time.RFC3339),
			Level:     INFO.String(),
			Message:   msg,
		}
		jsonData, _ := json.Marshal(entry)
		log.Println(string(jsonData))
	}
}
