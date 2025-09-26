package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

type logger struct {
	level      Level
	fileLogger *log.Logger
	console    *log.Logger
}

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

func parseLevel(levelStr string) Level {
	switch strings.ToUpper(strings.TrimSpace(levelStr)) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN":
		return WARN
	case "ERROR":
		return ERROR
	default:
		return INFO
	}
}

type entry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

func New(level string) Logger {
	logLevel := parseLevel(level)

	// Create log directory
	os.MkdirAll("log", 0o755)

	// Create log file
	fileName := fmt.Sprintf("log/bot_%s.log.json", time.Now().Format("2006-01-02T15-04-05"))

	logFile, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("Failed to open log file: %v", err)

		logFile = nil
	}

	var fileLogger *log.Logger
	if logFile != nil {
		fileLogger = log.New(logFile, "", 0)
	}

	return &logger{
		level:      logLevel,
		fileLogger: fileLogger,
		console:    log.New(os.Stdout, "", 0),
	}
}

func (l *logger) Debug(msg string, args ...interface{}) {
	if l.level <= DEBUG {
		l.log(DEBUG, msg, args...)
	}
}

func (l *logger) Info(msg string, args ...interface{}) {
	if l.level <= INFO {
		l.log(INFO, msg, args...)
	}
}

func (l *logger) Warn(msg string, args ...interface{}) {
	if l.level <= WARN {
		l.log(WARN, msg, args...)
	}
}

func (l *logger) Error(msg string, args ...interface{}) {
	if l.level <= ERROR {
		l.log(ERROR, msg, args...)
	}
}

func (l *logger) log(level Level, msg string, args ...interface{}) {
	data := make(map[string]interface{})
	for i := 0; i < len(args)-1; i += 2 {
		if key, ok := args[i].(string); ok && i+1 < len(args) {
			data[key] = args[i+1]
		}
	}

	entry := entry{
		Timestamp: time.Now().UTC(),
		Level:     level.String(),
		Message:   msg,
		Data:      data,
	}

	jsonData, err := json.Marshal(entry)
	if err != nil {
		return
	}

	line := string(jsonData)

	if l.fileLogger != nil {
		l.fileLogger.Println(line)
	}

	if l.console != nil {
		l.console.Println(line)
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
	default:
		return "UNKNOWN"
	}
}
