package logger

import (
	"fmt"

	waLog "go.mau.fi/whatsmeow/util/log"
)

type WhatsmeowLogger struct {
	logger *Logger
	tag    string
}

func NewWhatsmeowLogger(logger *Logger, tag string) *WhatsmeowLogger {
	return &WhatsmeowLogger{
		logger: logger,
		tag:    tag,
	}
}

func (w *WhatsmeowLogger) Errorf(msg string, args ...interface{}) {
	w.logger.Error(fmt.Sprintf("[%s] %s", w.tag, fmt.Sprintf(msg, args...)), nil)
}

func (w *WhatsmeowLogger) Warnf(msg string, args ...interface{}) {
	w.logger.Warn(fmt.Sprintf("[%s] %s", w.tag, fmt.Sprintf(msg, args...)), nil)
}

func (w *WhatsmeowLogger) Infof(msg string, args ...interface{}) {
	w.logger.Info(fmt.Sprintf("[%s] %s", w.tag, fmt.Sprintf(msg, args...)), nil)
}

func (w *WhatsmeowLogger) Debugf(msg string, args ...interface{}) {
	w.logger.Debug(fmt.Sprintf("[%s] %s", w.tag, fmt.Sprintf(msg, args...)), nil)
}

func (w *WhatsmeowLogger) Sub(module string) waLog.Logger {
	subTag := fmt.Sprintf("%s/%s", w.tag, module)
	return NewWhatsmeowLogger(w.logger, subTag)
}
