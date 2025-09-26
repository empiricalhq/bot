package service

import (
	"context"
	"fmt"
	"time"

	"whatsbot/internal/domain"
	"whatsbot/internal/logger"
	"whatsbot/internal/message"
	"whatsbot/internal/repository"
	"whatsbot/internal/templates"
)

// Bot defines the interface for the core bot service.
type Bot interface {
	ProcessMessage(ctx context.Context, msg *message.Message) error
}

// BotService orchestrates the bot's logic, coordinating between the FSM, actions, and repository.
type BotService struct {
	repo          repository.BotRepository
	fsm           FSM
	actionHandler ActionHandler
	renderer      templates.Renderer
	sender        *message.Sender
	logger        *logger.Logger
}

// NewBotService creates a new instance of the bot's core service.
func NewBotService(
	repo repository.BotRepository,
	fsm FSM,
	ah ActionHandler,
	r templates.Renderer,
	sender *message.Sender,
	log *logger.Logger,
) Bot {
	return &BotService{
		repo:          repo,
		fsm:           fsm,
		actionHandler: ah,
		renderer:      r,
		sender:        sender,
		logger:        log,
	}
}

// ProcessMessage handles an incoming message, determines the response, and persists the state.
func (s *BotService) ProcessMessage(ctx context.Context, msg *message.Message) error {
	userID := msg.GetSenderID()
	inputText := msg.GetText()

	if inputText == "" && !msg.HasMedia() {
		s.logger.Debug("Ignoring empty message", map[string]interface{}{"userID": userID})

		return nil
	}

	userState, err := s.getOrCreateUserState(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to get/create user state: %w", err)
	}

	originalNodeID := userState.CurrentNode
	nextNodeID, actionName := s.fsm.DetermineNextNode(userState, inputText, msg.HasMedia())

	if err := s.actionHandler.Execute(actionName, userState, msg); err != nil {
		s.logger.Error("Action execution failed", map[string]interface{}{
			"action": actionName, "userID": userID, "error": err.Error(),
		})
	}

	userState.CurrentNode = nextNodeID

	node, exists := s.fsm.GetFlow().Nodes[nextNodeID]
	if !exists {
		return fmt.Errorf("target node '%s' not found in flow definition", nextNodeID)
	}

	responseContent := s.renderer.RenderText(node.Message.Content, userState)

	inboundMsg := &domain.ConversationMessage{
		UserID: userID, Timestamp: time.Now(), Direction: "inbound",
		MessageContent: inputText, NodeID: originalNodeID,
	}
	outboundMsg := &domain.ConversationMessage{
		UserID: userID, Timestamp: time.Now(), Direction: "outbound",
		MessageContent: responseContent, NodeID: nextNodeID,
	}

	if err := s.repo.UpdateUserStateAndLog(ctx, userState, inboundMsg, outboundMsg); err != nil {
		return fmt.Errorf("failed to save state and messages: %w", err)
	}

	if s.sender != nil && responseContent != "" {
		err := s.sender.SendText(ctx, msg.Recipient, responseContent)
		if err != nil {
			s.logger.Error("Failed to send response message", map[string]interface{}{
				"error": err.Error(), "recipient": userID,
			})
		}
	}

	s.logger.Debug("Message processed successfully", map[string]interface{}{
		"userID": userID, "fromNode": originalNodeID, "toNode": nextNodeID, "action": actionName,
	})

	return nil
}

func (s *BotService) getOrCreateUserState(ctx context.Context, msg *message.Message) (*domain.UserState, error) {
	userState, err := s.repo.GetUserState(ctx, msg.GetSenderID())
	if err != nil {
		return nil, err
	}

	// If CurrentNode is empty, this is a new or uninitialized user.
	if userState.CurrentNode == "" {
		userState.CurrentNode = s.fsm.GetFlow().StartNode
		userState.UserName = msg.GetPushName()
		s.logger.Info("New user initialized", map[string]interface{}{
			"userID": userState.UserID, "startNode": userState.CurrentNode,
		})
	}

	return userState, nil
}
