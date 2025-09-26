package handler

import (
	"context"
	"time"

	"go.mau.fi/whatsmeow/types/events"

	"whatsbot/internal/config"
	"whatsbot/internal/logger"
	"whatsbot/internal/message"
	"whatsbot/internal/service"
)

const msgProcTimeout = 30 * time.Second

type WhatsApp struct {
	config  *config.Config
	service service.Bot
	logger  *logger.Logger
}

func NewWhatsApp(cfg *config.Config, s service.Bot, l *logger.Logger) *WhatsApp {
	return &WhatsApp{
		config:  cfg,
		service: s,
		logger:  l,
	}
}

func (h *WhatsApp) EventHandler(evt interface{}) {
	switch event := evt.(type) {
	case *events.Message:
		// ignore all non-direct messages and messages sent by the bot itself
		if event.Info.IsFromMe || event.Info.Chat.Server != "s.whatsapp.net" {
			return
		}

		h.handleMessage(event)
	case *events.Connected:
		h.logger.Info("WhatsApp client connected", nil)
	case *events.Disconnected:
		h.logger.Warn("WhatsApp client disconnected", nil)
	}
}

func (h *WhatsApp) handleMessage(evt *events.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), msgProcTimeout)
	defer cancel()

	msg := message.New(evt)
	if msg == nil {
		h.logger.Debug("Ignoring non-text or group message",
			map[string]interface{}{"from": evt.Info.Sender.String()})

		return
	}

	senderID := msg.GetSenderID()
	if h.shouldIgnoreMessage(senderID) {
		h.logger.Info("Ignoring message in dev mode from unauthorized user", map[string]interface{}{
			"from": senderID, "env": h.config.Environment,
		})

		return
	}

	h.logger.Debug("Processing inbound message", map[string]interface{}{
		"from": senderID, "text": msg.GetText(),
	})

	err := h.service.ProcessMessage(ctx, msg)
	if err != nil {
		h.logger.Error("Message processing failed", map[string]interface{}{
			"error":  err.Error(),
			"sender": senderID,
		})
	}
}

// shouldIgnoreMessage checks if the message should be processed based on dev mode settings.
func (h *WhatsApp) shouldIgnoreMessage(senderID string) bool {
	if h.config.Environment != "dev" {
		return false
	}

	if len(h.config.DevAllowedUsers) == 0 {
		h.logger.Warn("No DEV_ALLOWED_USERS specified; ignoring all messages in dev mode", nil)
		return true // in dev mode, if no users are specified, ignore all.
	}

	_, allowed := h.config.DevAllowedUsers[senderID]

	return !allowed
}
