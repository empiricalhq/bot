package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

// Dispatcher forwards logs to both a file and the console.
// Logs with "component=whatsmeow" are skipped from console output.
type Dispatcher struct {
	consoleHandler slog.Handler
	fileHandler    slog.Handler
}

func New(level string) (*slog.Logger, io.Closer, error) {
	if err := os.MkdirAll("log", 0o755); err != nil {
		return nil, nil, fmt.Errorf("could not create log directory: %w", err)
	}

	fileName := fmt.Sprintf("log/bot_%s.log", time.Now().Format("2006-01-02T15-04-05"))
	logFile, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("could not open log file: %w", err)
	}

	logLevel := new(slog.LevelVar)
	logLevel.Set(parseLevel(level))

	consoleHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.Format("15:04:05"))
				}
			}
			return a
		},
	})

	fileHandler := slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: logLevel,
	})

	dispatcher := &Dispatcher{
		consoleHandler: consoleHandler,
		fileHandler:    fileHandler,
	}

	logger := slog.New(dispatcher)
	return logger, logFile, nil
}

// Enabled returns true if the log level is enabled on either handler.
func (d *Dispatcher) Enabled(ctx context.Context, level slog.Level) bool {
	return d.consoleHandler.Enabled(ctx, level) || d.fileHandler.Enabled(ctx, level)
}

// Handle writes every log to the file, and to the console
// unless the log has "component=whatsmeow".
func (d *Dispatcher) Handle(ctx context.Context, r slog.Record) error {
	if err := d.fileHandler.Handle(ctx, r); err != nil {
		return err
	}

	var isWhatsmeow bool
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "component" {
			if val, ok := a.Value.Any().(string); ok && val == "whatsmeow" {
				isWhatsmeow = true
				return false
			}
		}
		return true
	})

	if !isWhatsmeow {
		return d.consoleHandler.Handle(ctx, r)
	}

	return nil
}

// WithAttrs returns a new Dispatcher with extra attributes applied.
func (d *Dispatcher) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Dispatcher{
		consoleHandler: d.consoleHandler.WithAttrs(attrs),
		fileHandler:    d.fileHandler.WithAttrs(attrs),
	}
}

// WithGroup returns a new Dispatcher with the given group applied.
func (d *Dispatcher) WithGroup(name string) slog.Handler {
	return &Dispatcher{
		consoleHandler: d.consoleHandler.WithGroup(name),
		fileHandler:    d.fileHandler.WithGroup(name),
	}
}

func parseLevel(levelStr string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(levelStr)) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
