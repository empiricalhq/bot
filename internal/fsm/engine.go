package fsm

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"whatsbot/internal/actions"
	"whatsbot/internal/logger"
	"whatsbot/internal/message"
	"whatsbot/internal/state"
	"whatsbot/internal/templates"
)

var ErrCurrentNodeNotFound = errors.New("current node not found")

type Engine struct {
	flow          *Flow
	stateManager  state.Manager
	actionHandler actions.Handler
	renderer      templates.Renderer
	logger        *logger.Logger
}

func NewEngine(
	flow *Flow,
	stateManager state.Manager,
	actionHandler actions.Handler,
	renderer templates.Renderer,
	log *logger.Logger,
) *Engine {
	return &Engine{
		flow:          flow,
		stateManager:  stateManager,
		actionHandler: actionHandler,
		renderer:      renderer,
		logger:        log,
	}
}

func (e *Engine) ProcessMessage(ctx context.Context, msg *message.Message) (string, error) {
	userID := msg.GetSenderID()
	e.logger.Debug("Processing message", map[string]interface{}{
		"userID":  userID,
		"message": msg.GetText(),
	})

	userState, err := e.getOrCreateUserState(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user state: %w", err)
	}

	// save incoming message
	err = e.saveInboundMessage(ctx, userState, msg)
	if err != nil {
		e.logger.Error("Failed to save inbound message", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// determine next node
	nextNode, actionToExecute, err := e.determineNextNode(msg.GetText(), userState)
	if err != nil {
		return "", fmt.Errorf("failed to determine next node: %w", err)
	}

	// update user state
	userState.CurrentNode = nextNode
	userState.LastUpdated = time.Now()

	// execute action
	if actionToExecute != "" {
		err := e.actionHandler.Execute(ctx, actionToExecute, userID, msg)
		if err != nil {
			e.logger.Error("Action execution failed", map[string]interface{}{
				"action": actionToExecute,
				"error":  err.Error(),
			})
		}
	}

	// save updated state
	err = e.stateManager.SaveUserState(ctx, userState)
	if err != nil {
		e.logger.Error("Failed to save user state", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// render response
	node := e.flow.Nodes[nextNode]
	response := e.renderer.RenderText(node.Message.Content, userState.UserName)

	// save outbound message
	err = e.saveOutboundMessage(ctx, userState, response)
	if err != nil {
		e.logger.Error("Failed to save outbound message", map[string]interface{}{
			"error": err.Error(),
		})
	}

	e.logger.Info("Message processed", map[string]interface{}{
		"userID":   userID,
		"nextNode": nextNode,
		"action":   actionToExecute,
	})

	return response, nil
}

func (e *Engine) getOrCreateUserState(ctx context.Context, userID string) (*state.UserState, error) {
	userState, err := e.stateManager.GetUserState(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("could not get user state: %w", err)
	}

	// A new user is identified by an empty CurrentNode from GetUserState
	if userState.CurrentNode == "" {
		userState.CurrentNode = e.flow.StartNode
		userState.LastUpdated = time.Now()

		err = e.stateManager.SaveUserState(ctx, userState)
		if err != nil {
			e.logger.Error("Failed to save new user state", map[string]interface{}{
				"error": err.Error(),
			})

			return nil, fmt.Errorf("could not save new user state: %w", err)
		}
	}

	// Fallback validation for existing users with invalid nodes
	if _, exists := e.flow.Nodes[userState.CurrentNode]; !exists {
		userState.CurrentNode = e.flow.StartNode
	}

	return userState, nil
}

func (e *Engine) determineNextNode(inputText string, userState *state.UserState) (nextNode, action string, err error) {
	currentNode, exists := e.flow.Nodes[userState.CurrentNode]
	if !exists {
		return "", "", fmt.Errorf("%w: %s", ErrCurrentNodeNotFound, userState.CurrentNode)
	}

	// check global transitions first
	for _, transition := range e.flow.GlobalTransitions {
		if e.matchCondition(inputText, transition.Condition) {
			return transition.Target, transition.Action, nil
		}
	}

	// check node-specific transitions
	for _, transition := range currentNode.Transitions {
		if e.matchCondition(inputText, transition.Condition) {
			return transition.Target, transition.Action, nil
		}
	}

	// fallback
	fallbackNode := e.flow.FallbackNode
	if fallbackNode == "" {
		fallbackNode = e.flow.StartNode
	}

	e.logger.Debug("No matching transition, using fallback", map[string]interface{}{
		"currentNode":  userState.CurrentNode,
		"fallbackNode": fallbackNode,
		"input":        inputText,
	})

	return fallbackNode, "", nil
}

func (e *Engine) matchCondition(inputText string, condition Condition) bool {
	lowerInput := strings.ToLower(strings.TrimSpace(inputText))

	switch condition.Type {
	case "exact":
		for _, val := range condition.Value {
			if strings.EqualFold(lowerInput, val) {
				return true
			}
		}

		return false
	case "keyword":
		for _, val := range condition.Value {
			if strings.Contains(lowerInput, strings.ToLower(val)) {
				return true
			}
		}

		return false
	case "regex":
		if condition.Regex == "" {
			return false
		}
		compiledRegex, err := regexp.Compile(condition.Regex)
		if err != nil {
			e.logger.Error("Invalid regex", map[string]interface{}{"regex": condition.Regex, "error": err.Error()})

			return false
		}

		return compiledRegex.MatchString(lowerInput)
	case "any_text":
		return lowerInput != ""
	}

	return false
}

func (e *Engine) saveInboundMessage(ctx context.Context, userState *state.UserState, msg *message.Message) error {
	err := e.stateManager.SaveMessage(ctx, &state.ConversationMessage{
		UserID:         userState.UserID,
		Timestamp:      time.Now(),
		Direction:      "inbound",
		MessageContent: msg.GetText(),
		NodeID:         userState.CurrentNode,
	})
	if err != nil {
		return fmt.Errorf("failed to save inbound message: %w", err)
	}

	return nil
}

func (e *Engine) saveOutboundMessage(ctx context.Context, userState *state.UserState, response string) error {
	err := e.stateManager.SaveMessage(ctx, &state.ConversationMessage{
		UserID:         userState.UserID,
		Timestamp:      time.Now(),
		Direction:      "outbound",
		MessageContent: response,
		NodeID:         userState.CurrentNode,
	})
	if err != nil {
		return fmt.Errorf("failed to save outbound message: %w", err)
	}

	return nil
}
