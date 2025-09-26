package service

import (
	"context"
	"time"

	"go.mau.fi/whatsmeow/types/events"

	"whatsbot/internal/config"
	"whatsbot/internal/domain"
	"whatsbot/internal/logger"
	"whatsbot/internal/message"
	"whatsbot/internal/repository"
	"whatsbot/internal/template"
)

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
}

func NewBot(
	repo repository.Repository,
	fsm FSM,
	actions ActionHandler,
	renderer template.Renderer,
	whatsapp WhatsAppClient,
	logger logger.Logger,
) *Bot {
	return &Bot{
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

	// Ignore bot's own messages and group chats
	if msgEvent.Info.IsFromMe || msgEvent.Info.Chat.Server != "s.whatsapp.net" {
		return
	}

	msg := message.FromEvent(msgEvent)
	if msg == nil {
		return
	}

	if b.shouldIgnoreUser(msg.SenderID) {
		b.logger.Debug("Ignoring user in dev mode", "user", msg.SenderID)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	outMsg := &domain.ConversationMessage{
		UserID:         msg.SenderID,
		Timestamp:      time.Now(),
		Direction:      "outbound",
		MessageContent: responseText,
		NodeID:         nextNode,
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
	}

	b.logger.Debug("Message processed",
		"user", msg.SenderID,
		"from", originalNode,
		"to", nextNode,
		"action", action)

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
		b.logger.Info("New user initialized", "user", msg.SenderID, "node", state.CurrentNode)
	}

	return state, nil
}

func (b *Bot) generateResponse(nodeID string, state *domain.UserState) string {
	node := b.fsm.GetNode(nodeID)
	if node == nil {
		b.logger.Error("Node not found", "node", nodeID)
		return "Sorry, something went wrong."
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
