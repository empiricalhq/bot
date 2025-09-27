package service

import (
	"errors"
	"strings"

	"whatsbot/internal/domain"
	"whatsbot/internal/logger"
	"whatsbot/internal/message"
	"whatsbot/internal/nameparser"
)

type ActionHandler interface {
	Execute(action string, state *domain.UserState, msg *message.Message) error
}

type actionHandler struct {
	logger logger.Logger
}

func NewActionHandler(logger logger.Logger) ActionHandler {
	return &actionHandler{logger: logger}
}

func (a *actionHandler) Execute(action string, state *domain.UserState, msg *message.Message) error {
	if action == "" {
		return nil
	}

	a.logger.Debug("Executing action", "action", action, "user", state.UserID)

	switch action {
	case "save_user_name":
		return a.saveUserName(state, msg)
	case "clear_user_name":
		return a.clearUserName(state)
	case "create_new_lead":
		a.logger.Info("New lead created", "user", state.UserID, "name", state.UserName)
	case "update_lead_interest_beginner":
		state.CourseInterest = "beginner"
	case "update_lead_interest_advanced":
		state.CourseInterest = "advanced"
	case "update_lead_consulted_price":
		state.ConsultedPrice = true
	case "escalate_to_human_agent":
		state.RequiresHumanAgent = true
		a.logger.Warn("Escalated to human", "user", state.UserID, "name", state.UserName)
	default:
		a.logger.Warn("Unknown action", "action", action)

		return errors.New("unknown action: " + action)
	}

	return nil
}

// clearUserName deactivates name personalization by clearing the stored name
// with no stored name, the renderer falls back to the default.
func (a *actionHandler) clearUserName(state *domain.UserState) error {
	state.UserName = ""
	a.logger.Info("User name cleared by user request", "user", state.UserID)

	return nil
}

func (a *actionHandler) saveUserName(state *domain.UserState, msg *message.Message) error {
	nameInput := strings.TrimSpace(msg.Text)
	if nameInput == "" {
		return nil
	}

	if nameparser.Parse(nameInput) == "" {
		a.logger.Warn("Invalid name input ignored during update attempt", "user", state.UserID, "input", nameInput)

		return nil
	}

	state.UserName = nameInput
	a.logger.Info("User name updated by user request", "user", state.UserID, "new_name", nameInput)

	return nil
}
