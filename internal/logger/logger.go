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

// Dispatcher fans out log records to multiple slog.Handlers (console, file, GUI, etc.).
type Dispatcher struct {
	handlers []slog.Handler
}

// New sets up a slog.Logger with console + file logging, plus any extra handlers.
// - Console logs are time-trimmed (HH:MM:SS).
// - File logs are written to "log/bot_<timestamp>.log".
// - Additional handlers can be injected (e.g. GUI log handler).
func New(level string, extraHandlers ...slog.Handler) (*slog.Logger, io.Closer, error) {
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

	// base handlers are always console and file.
	handlers := []slog.Handler{
		// a custom handler to filter out whatsmeow from console
		newConsoleFilter(consoleHandler),
		fileHandler,
	}

	// add any extra handlers provided.
	handlers = append(handlers, extraHandlers...)

	dispatcher := &Dispatcher{
		handlers: handlers,
	}

	logger := slog.New(dispatcher)

	return logger, logFile, nil
}

// Enabled returns true if any underlying handler is enabled for the given level.
func (d *Dispatcher) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range d.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}

	return false
}

// Handle forwards a log record to all handlers, returning the first error (if any).
func (d *Dispatcher) Handle(ctx context.Context, r slog.Record) error {
	var firstErr error

	for _, h := range d.handlers {
		err := h.Handle(ctx, r)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// WithAttrs applies attrs to all handlers and returns a new Dispatcher.
func (d *Dispatcher) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(d.handlers))
	for i, h := range d.handlers {
		newHandlers[i] = h.WithAttrs(attrs)
	}

	return &Dispatcher{handlers: newHandlers}
}

// WithGroup applies a group to all handlers and returns a new Dispatcher.
func (d *Dispatcher) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(d.handlers))
	for i, h := range d.handlers {
		newHandlers[i] = h.WithGroup(name)
	}

	return &Dispatcher{handlers: newHandlers}
}

// consoleFilter wraps a handler but suppresses logs with component=whatsmeow.
// Used to keep WhatsApp logs out of console while still writing to file.
type consoleFilter struct {
	slog.Handler
}

func newConsoleFilter(handler slog.Handler) *consoleFilter {
	return &consoleFilter{Handler: handler}
}

// Handle filters out "component=whatsmeow" before delegating to the inner handler.
func (h *consoleFilter) Handle(ctx context.Context, r slog.Record) error {
	isWhatsmeow := false

	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "component" && a.Value.String() == "whatsmeow" {
			isWhatsmeow = true

			return false // stop iterating
		}

		return true
	})

	if isWhatsmeow {
		return nil // skip this record
	}

	return h.Handler.Handle(ctx, r)
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
