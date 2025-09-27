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

// Dispatcher writes logs to a file and optionally to the console.
// Console output can be disabled, and logs with "component=whatsmeow"
// are excluded from console output.
type Dispatcher struct {
	consoleHandler slog.Handler
	fileHandler    slog.Handler
	disableConsole bool
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
		disableConsole: false,
	}

	logger := slog.New(dispatcher)

	return logger, logFile, nil
}

// Enabled returns true if the file handler allows the given log level.
// Console handler is ignored here since it can be disabled.
func (d *Dispatcher) Enabled(ctx context.Context, level slog.Level) bool {
	return d.fileHandler.Enabled(ctx, level)
}

// Handle always writes the record to the file.
// Console output is skipped if disableConsole is true.
func (d *Dispatcher) Handle(ctx context.Context, r slog.Record) error {
	err := d.fileHandler.Handle(ctx, r)
	if err != nil {
		return err
	}

	if !d.disableConsole {
		return d.consoleHandler.Handle(ctx, r)
	}

	return nil
}

// WithAttrs returns a new Dispatcher with added attributes.
// If the "component" is "whatsmeow", console output is disabled
// for this handler and all derived ones.
func (d *Dispatcher) WithAttrs(attrs []slog.Attr) slog.Handler {
	isWhatsmeow := false

	for _, a := range attrs {
		if a.Key == "component" && a.Value.String() == "whatsmeow" {
			isWhatsmeow = true

			break
		}
	}

	newDispatcher := &Dispatcher{
		consoleHandler: d.consoleHandler.WithAttrs(attrs),
		fileHandler:    d.fileHandler.WithAttrs(attrs),
		disableConsole: d.disableConsole,
	}

	if isWhatsmeow {
		newDispatcher.disableConsole = true
	}

	return newDispatcher
}

// WithGroup returns a new Dispatcher with the group applied.
// The disableConsole flag is carried over.
func (d *Dispatcher) WithGroup(name string) slog.Handler {
	return &Dispatcher{
		consoleHandler: d.consoleHandler.WithGroup(name),
		fileHandler:    d.fileHandler.WithGroup(name),
		disableConsole: d.disableConsole,
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
