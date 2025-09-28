package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"whatsbot/internal/config"
	"whatsbot/internal/domain"
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
	logger   *slog.Logger
}

type WhatsAppClient interface {
	SendText(ctx context.Context, to, text string) error
	GetJID() types.JID
	Download(msg whatsmeow.DownloadableMessage) ([]byte, error)
}

func NewBot(
	config *config.Config,
	repo repository.Repository,
	fsm FSM,
	actions ActionHandler,
	renderer template.Renderer,
	whatsapp WhatsAppClient,
	logger *slog.Logger,
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
		b.logger.Debug("Ignoring event: not a message", "event_type", fmt.Sprintf("%T", evt))

		return
	}

	b.logger.Debug("Received message event",
		"sender", msgEvent.Info.Sender.ToNonAD().String(),
		"is_from_me", msgEvent.Info.IsFromMe)

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
    b.logger.Debug("Ignoring event: message not processable", "sender", msgEvent.Info.Sender.ToNonAD().String())
		return
	}

	if b.shouldIgnoreUser(msg.SenderID) {
		b.logger.Debug("Ignoring user in dev mode", "user", msg.SenderID)

		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), messageTimeout)
	defer cancel()

	// Get user state. This step also handles onboarding new users.
	userState, isNewUser, err := b.getOrCreateUserState(ctx, msg)
	if err != nil {
		b.logger.Error("Failed to get or create user state", "error", err, "user", msg.SenderID)

		return
	}

	// If the user is new, their onboarding is complete. Stop here.
	if isNewUser {
		b.logger.Info("New user onboarded and greeted", "user", msg.SenderID)

		return
	}

	// If the user already exists, process their message through the FSM.
	err = b.processExistingUserMessage(ctx, userState, msg, msgEvent)
	if err != nil {
		b.logger.Error("Message processing failed for existing user", "error", err, "user", msg.SenderID)
	}
}

func (b *Bot) processExistingUserMessage(ctx context.Context, userState *domain.UserState, msg *message.Message, rawEvt interface{}) error {
	logger := b.logger.With("user", msg.SenderID, "from_node", userState.CurrentNode)
	logger.Debug("Processing message", "text", msg.Text, "has_media", msg.HasMedia)

	originalNode := userState.CurrentNode
	nextNode, action := b.fsm.DetermineNext(userState, msg)

	logger.Debug("FSM determined next state", "to_node", nextNode, "action", action)

	if action != "" {
		err := b.actions.Execute(action, userState, msg, rawEvt)
		if err != nil {
			logger.Error("Action failed", "action", action, "error", err)
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

	err := b.repo.SaveStateAndMessages(ctx, userState, inMsg, outMsg)
	if err != nil {
		return fmt.Errorf("failed to save state and messages: %w", err)
	}

	if responseText != "" {
		err := b.whatsapp.SendText(ctx, msg.SenderID, responseText)
		if err != nil {
			logger.Error("Failed to send message", "error", err)
		}
	}

	logger.Info("Message processed",
		"inbound_text", msg.Text,
		"outbound_text", responseText,
		"to_node", nextNode,
		"action", action,
	)

	return nil
}

// getOrCreateUserState retrieves a user's state. If the user does not exist,
// it creates a new state, sends the initial greeting, and returns isNew=true.
func (b *Bot) getOrCreateUserState(ctx context.Context, msg *message.Message) (state *domain.UserState, isNew bool, err error) {
	userState, err := b.repo.GetUserState(ctx, msg.SenderID)
	if err != nil {
		return nil, false, fmt.Errorf("could not get user state from repository: %w", err)
	}

	// If CurrentNode is empty, this is a new user.
	if userState.CurrentNode == "" {
		b.logger.Info("New user initialized", "user", msg.SenderID, "node", b.fsm.GetStartNode(), "push_name", msg.PushName)

		userState.CurrentNode = b.fsm.GetStartNode()
		userState.UserName = msg.PushName
		userState.LastUpdated = time.Now()

		responseText := b.generateResponse(userState.CurrentNode, userState)
		if responseText == "" {
			b.logger.Warn("Start node has no message content, new user will not be greeted", "node", userState.CurrentNode)
		}

		outMsg := &domain.ConversationMessage{
			UserID:         msg.SenderID,
			Timestamp:      time.Now(),
			Direction:      "outbound",
			MessageContent: responseText,
			NodeID:         userState.CurrentNode,
		}

		err := b.repo.SaveStateAndMessages(ctx, userState, nil, outMsg)
		if err != nil {
			return nil, false, fmt.Errorf("failed to save new user state: %w", err)
		}

		if responseText != "" {
			err := b.whatsapp.SendText(ctx, msg.SenderID, responseText)
			if err != nil {
				b.logger.Error("Failed to send welcome message to new user", "error", err)
			}
		}

		return userState, true, nil
	}

	return userState, false, nil
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
