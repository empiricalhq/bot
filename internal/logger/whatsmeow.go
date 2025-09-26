package logger

import (
	"fmt"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// WhatsmeowLogger adapts our custom logger to the whatsmeow library's interface.
type WhatsmeowLogger struct {
	logger *Logger
	tag    string
}

// NewWhatsmeowLogger creates a new adapter.
func NewWhatsmeowLogger(logger *Logger, tag string) *WhatsmeowLogger {
	return &WhatsmeowLogger{
		logger: logger,
		tag:    tag,
	}
}

func (w *WhatsmeowLogger) Errorf(msg string, args ...interface{}) {
	w.logger.Error(fmt.Sprintf(msg, args...), nil)
}

func (w *WhatsmeowLogger) Warnf(msg string, args ...interface{}) {
	w.logger.Warn(fmt.Sprintf(msg, args...), nil)
}

func (w *WhatsmeowLogger) Infof(msg string, args ...interface{}) {
	w.logger.Info(fmt.Sprintf(msg, args...), nil)
}

func (w *WhatsmeowLogger) Debugf(msg string, args ...interface{}) {
	w.logger.Debug(fmt.Sprintf(msg, args...), nil)
}

func (w *WhatsmeowLogger) Sub(module string) waLog.Logger {
	return NewWhatsmeowLogger(w.logger, fmt.Sprintf("%s/%s", w.tag, module))
}
