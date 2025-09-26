package service

import (
	"errors"
	"strings"

	"whatsbot/internal/domain"
	"whatsbot/internal/logger"
	"whatsbot/internal/message"
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
		return a.saveUserName(state, msg.Text)
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

func (a *actionHandler) saveUserName(state *domain.UserState, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("user name cannot be empty")
	}
	state.UserName = name
	return nil
}
