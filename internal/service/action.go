package service

import (
	"errors"
	"fmt"
	"strings"

	"whatsbot/internal/domain"
	"whatsbot/internal/logger"
	"whatsbot/internal/message"
)

var (
	ErrUnknownAction = errors.New("unknown action")
	ErrUserNameEmpty = errors.New("user name cannot be empty")
)

const (
	ActionSaveUserName       = "save_user_name"
	ActionCreateNewLead      = "create_new_lead"
	ActionUpdateLeadBeginner = "update_lead_interest_beginner"
	ActionUpdateLeadAdvanced = "update_lead_interest_advanced"
	ActionUpdateLeadPrice    = "update_lead_consulted_price"
	ActionEscalateToHuman    = "escalate_to_human_agent"
)

type ActionHandler interface {
	Execute(actionName string, userState *domain.UserState, inputMsg *message.Message) error
}

type actionHandler struct {
	logger *logger.Logger
}

func NewActionHandler(log *logger.Logger) ActionHandler {
	return &actionHandler{logger: log}
}

// Execute runs the specified action, modifying the provided UserState in-memory.
func (h *actionHandler) Execute(actionName string, userState *domain.UserState, inputMsg *message.Message) error {
	if actionName == "" {
		return nil
	}

	h.logger.Debug("Executing action", map[string]interface{}{
		"action": actionName, "userID": userState.UserID,
	})

	var err error

	switch actionName {
	case ActionSaveUserName:
		err = h.saveUserName(userState, inputMsg.GetText())
	case ActionCreateNewLead:
		h.logger.Info("New lead created", map[string]interface{}{
			"userID": userState.UserID, "userName": userState.UserName,
		})
	case ActionUpdateLeadBeginner:
		userState.CourseInterest = "beginner"
	case ActionUpdateLeadAdvanced:
		userState.CourseInterest = "advanced"
	case ActionUpdateLeadPrice:
		userState.ConsultedPrice = true
	case ActionEscalateToHuman:
		userState.RequiresHumanAgent = true
		h.logger.Warn("Conversation escalated to human agent", map[string]interface{}{
			"userID": userState.UserID, "userName": userState.UserName,
		})
	default:
		err = fmt.Errorf("%w: %s", ErrUnknownAction, actionName)
		h.logger.Warn("Unknown action requested", map[string]interface{}{
			"action": actionName, "userID": userState.UserID,
		})
	}

	return err
}

func (h *actionHandler) saveUserName(userState *domain.UserState, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrUserNameEmpty
	}

	userState.UserName = name

	return nil
}
