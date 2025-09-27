package service

import (
	"context"
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"whatsbot/internal/config"
	"whatsbot/internal/domain"
	"whatsbot/internal/logger"
	"whatsbot/internal/message"
	"whatsbot/internal/repository"
	"whatsbot/internal/template"
)

const messageTimeout = 30 * time.Second

type Bot struct {
	config   *config.Config
	repo     repository.Repository
	fsm      FSM
	actions  ActionHandler
	renderer template.Renderer
	whatsapp WhatsAppClient
	logger   logger.Logger
}

type WhatsAppClient interface {
	SendText(ctx context.Context, to, text string) error
	GetJID() types.JID
}

func NewBot(
	config *config.Config,
	repo repository.Repository,
	fsm FSM,
	actions ActionHandler,
	renderer template.Renderer,
	whatsapp WhatsAppClient,
	logger logger.Logger,
) *Bot {
	return &Bot{
		config:   config,
		repo:     repo,
		fsm:      fsm,
		actions:  actions,
		renderer: renderer,
		whatsapp: whatsapp,
		logger:   logger,
	}
}

func (b *Bot) HandleEvent(evt interface{}) {
	msgEvent, ok := evt.(*events.Message)
	if !ok {
		return
	}

	// Ignore messages sent by this bot.
	// In PROD: ignore all self-messages.
	// In DEV: only allow self-messages from another linked device.
	if msgEvent.Info.IsFromMe {
		isDevMode := b.config.Environment == "dev"
		isFromAnotherDevice := msgEvent.Info.DeviceSentMeta != nil

		if !isDevMode || !isFromAnotherDevice {
			return
		}
	}

	msg := message.FromEvent(msgEvent)
	if msg == nil {
		return
	}

	if b.shouldIgnoreUser(msg.SenderID) {
		b.logger.Debug("Ignoring user in dev mode", "user", msg.SenderID)

		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), messageTimeout)
	defer cancel()

	err := b.processMessage(ctx, msg)
	if err != nil {
		b.logger.Error("Message processing failed", "error", err, "user", msg.SenderID)
	}
}

func (b *Bot) processMessage(ctx context.Context, msg *message.Message) error {
	userState, err := b.getOrCreateUserState(ctx, msg)
	if err != nil {
		return err
	}

	originalNode := userState.CurrentNode
	nextNode, action := b.fsm.DetermineNext(userState, msg.Text, msg.HasMedia)

	if action != "" {
		err = b.actions.Execute(action, userState, msg)
		if err != nil {
			b.logger.Error("Action failed", "action", action, "error", err)
		}
	}

	userState.CurrentNode = nextNode
	userState.LastUpdated = time.Now()

	responseText := b.generateResponse(nextNode, userState)

	inMsg := &domain.ConversationMessage{
		UserID:         msg.SenderID,
		Timestamp:      time.Now(),
		Direction:      "inbound",
		MessageContent: msg.Text,
		NodeID:         originalNode,
	}

	var outMsg *domain.ConversationMessage
	if responseText != "" {
		outMsg = &domain.ConversationMessage{
			UserID:         msg.SenderID,
			Timestamp:      time.Now(),
			Direction:      "outbound",
			MessageContent: responseText,
			NodeID:         nextNode,
		}
	}

	err = b.repo.SaveStateAndMessages(ctx, userState, inMsg, outMsg)
	if err != nil {
		return err
	}

	if responseText != "" {
		err = b.whatsapp.SendText(ctx, msg.SenderID, responseText)
		if err != nil {
			b.logger.Error("Failed to send message", "error", err, "to", msg.SenderID)
		}
	} else {
		b.logger.Debug("No response text to send",
			"user", msg.SenderID,
			"from_node", originalNode,
			"to_node", nextNode)
	}

	return nil
}

func (b *Bot) getOrCreateUserState(ctx context.Context, msg *message.Message) (*domain.UserState, error) {
	state, err := b.repo.GetUserState(ctx, msg.SenderID)
	if err != nil {
		return nil, err
	}

	if state.CurrentNode == "" {
		state.CurrentNode = b.fsm.GetStartNode()
		state.UserName = msg.PushName
		b.logger.Info("New user initialized", "user", msg.SenderID, "node", state.CurrentNode, "push_name", msg.PushName)
	}

	return state, nil
}

func (b *Bot) generateResponse(nodeID string, state *domain.UserState) string {
	node := b.fsm.GetNode(nodeID)
	if node == nil {
		b.logger.Error("Node not found", "node", nodeID)

		return "Sorry, something went wrong."
	}

	if node.Message.Content == "" {
		b.logger.Debug("Node has no message content", "node", nodeID)

		return ""
	}

	return b.renderer.Render(node.Message.Content, state)
}

func (b *Bot) shouldIgnoreUser(userID string) bool {
	if b.config.Environment != "dev" {
		return false
	}

	if len(b.config.DevAllowedUsers) == 0 {
		b.logger.Debug("There are no allowed users set. Add DEV_ALLOWED_USERS for testing.")

		return true // In dev mode with no allowed users, ignore all
	}

	_, allowed := b.config.DevAllowedUsers[userID]

	return !allowed
}
