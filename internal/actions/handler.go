package actions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"whatsbot/internal/logger"
	"whatsbot/internal/message"
	"whatsbot/internal/state"
)

var (
	ErrUnknownAction = errors.New("unknown action")
	ErrUserNameEmpty = errors.New("user name cannot be empty")
)

type Handler interface {
	Execute(ctx context.Context, actionName, userID string, inputMessage *message.Message) error
}

type DefaultHandler struct {
	stateManager state.Manager
	logger       *logger.Logger
}

func NewHandler(sm state.Manager, log *logger.Logger) *DefaultHandler {
	return &DefaultHandler{
		stateManager: sm,
		logger:       log,
	}
}

func (h *DefaultHandler) Execute(ctx context.Context, actionName, userID string, inputMessage *message.Message) error {
	if actionName == "" {
		return nil // no action to execute
	}

	h.logger.Debug("Executing action", map[string]interface{}{
		"action": actionName,
		"userID": userID,
	})

	switch actionName {
	case ActionSaveUserName:
		return h.saveUserName(ctx, userID, inputMessage.GetText())
	case ActionCreateNewLead:
		return h.createNewLead(ctx, userID)
	case ActionUpdateLeadBeginner:
		return h.updateLeadInterest(ctx, userID, "beginner")
	case ActionUpdateLeadAdvanced:
		return h.updateLeadInterest(ctx, userID, "advanced")
	case ActionUpdateLeadPrice:
		return h.updateLeadConsultedPrice(ctx, userID)
	case ActionEscalateToHuman:
		return h.escalateToHumanAgent(ctx, userID)
	default:
		h.logger.Warn("Unknown action", map[string]interface{}{
			"action": actionName,
			"userID": userID,
		})

		return fmt.Errorf("%w: %s", ErrUnknownAction, actionName)
	}
}

func (h *DefaultHandler) saveUserName(ctx context.Context, userID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrUserNameEmpty
	}

	userState, err := h.stateManager.GetUserState(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user state: %w", err)
	}

	userState.UserName = name

	err = h.stateManager.SaveUserState(ctx, userState)
	if err != nil {
		return fmt.Errorf("failed to save user name: %w", err)
	}

	h.logger.Info("User name saved", map[string]interface{}{
		"userID": userID,
		"name":   name,
	})

	return nil
}

func (h *DefaultHandler) createNewLead(ctx context.Context, userID string) error {
	userState, err := h.stateManager.GetUserState(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user state: %w", err)
	}

	h.logger.Info("New lead created", map[string]interface{}{
		"userID":      userID,
		"currentNode": userState.CurrentNode,
		"userName":    userState.UserName,
	})

	return nil
}

func (h *DefaultHandler) updateLeadInterest(ctx context.Context, userID, interest string) error {
	userState, err := h.stateManager.GetUserState(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user state: %w", err)
	}

	userState.CourseInterest = interest

	err = h.stateManager.SaveUserState(ctx, userState)
	if err != nil {
		return fmt.Errorf("failed to update lead interest: %w", err)
	}

	h.logger.Info("Lead interest updated", map[string]interface{}{
		"userID":   userID,
		"interest": interest,
	})

	return nil
}

func (h *DefaultHandler) updateLeadConsultedPrice(ctx context.Context, userID string) error {
	userState, err := h.stateManager.GetUserState(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user state: %w", err)
	}

	userState.ConsultedPrice = true

	err = h.stateManager.SaveUserState(ctx, userState)
	if err != nil {
		return fmt.Errorf("failed to update price consultation: %w", err)
	}

	h.logger.Info("Lead consulted price", map[string]interface{}{
		"userID": userID,
	})

	return nil
}

func (h *DefaultHandler) escalateToHumanAgent(ctx context.Context, userID string) error {
	userState, err := h.stateManager.GetUserState(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user state: %w", err)
	}

	userState.RequiresHumanAgent = true

	err = h.stateManager.SaveUserState(ctx, userState)
	if err != nil {
		return fmt.Errorf("failed to mark for human agent: %w", err)
	}

	h.logger.Warn("Conversation escalated to human agent", map[string]interface{}{
		"userID":   userID,
		"userName": userState.UserName,
		"interest": userState.CourseInterest,
	})

	return nil
}
