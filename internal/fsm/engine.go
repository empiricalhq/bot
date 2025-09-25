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

var (
	ErrCurrentNodeNotFound = errors.New("current node not found")
	ErrInvalidCondition    = errors.New("invalid condition type")
)

type Engine struct {
	flow          *Flow
	stateManager  state.Manager
	actionHandler actions.Handler
	renderer      templates.Renderer
	logger        *logger.Logger
	regexCache    map[string]*regexp.Regexp
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
		regexCache:    make(map[string]*regexp.Regexp),
	}
}

func (e *Engine) ProcessMessage(ctx context.Context, msg *message.Message) (string, error) {
	userID := msg.GetSenderID()
	inputText := strings.TrimSpace(msg.GetText())

	// ignore empty messages (stickers, images without caption, etc.)
	if inputText == "" {
		return "", nil
	}

	e.logger.Debug("Processing message", map[string]interface{}{
		"userID": userID,
		"text":   inputText,
	})

	userState, err := e.getOrCreateUserState(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user state: %w", err)
	}

	if err := e.saveInboundMessage(ctx, userState, msg); err != nil {
		e.logger.Error("Failed to save inbound message", map[string]interface{}{"error": err.Error()})
	}

	nextNode, actionToExecute, err := e.determineNextNode(inputText, userState)
	if err != nil {
		return "", fmt.Errorf("failed to determine next node: %w", err)
	}

	// update user state
	userState.CurrentNode = nextNode
	userState.LastUpdated = time.Now()

	if actionToExecute != "" {
		err := e.actionHandler.Execute(ctx, actionToExecute, userID, msg)
		if err != nil {
			e.logger.Error("Action execution failed", map[string]interface{}{
				"action": actionToExecute,
				"error":  err.Error(),
			})
			// don't fail the entire flow for action errors
		}
	}

	if err := e.stateManager.SaveUserState(ctx, userState); err != nil {
		e.logger.Error("Failed to save user state", map[string]interface{}{"error": err.Error()})
	}

	node, exists := e.flow.Nodes[nextNode]
	if !exists {
		return "", fmt.Errorf("target node not found: %s", nextNode)
	}

	response := e.renderer.RenderText(node.Message.Content, userState.UserName)

	if err := e.saveOutboundMessage(ctx, userState, response); err != nil {
		e.logger.Error("Failed to save outbound message", map[string]interface{}{"error": err.Error()})
	}

	e.logger.Debug("Message processed successfully", map[string]interface{}{
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

	// new user: start with the flow's start node
	if userState.CurrentNode == "" {
		userState.CurrentNode = e.flow.StartNode
		userState.LastUpdated = time.Now()

		err := e.stateManager.SaveUserState(ctx, userState)
		if err != nil {
			e.logger.Error("Failed to save new user state", map[string]interface{}{"error": err.Error()})

			return nil, fmt.Errorf("could not save new user state: %w", err)
		}

		e.logger.Info("New user initialized", map[string]interface{}{
			"userID":    userID,
			"startNode": e.flow.StartNode,
		})
	}

	// validate current node exists
	if _, exists := e.flow.Nodes[userState.CurrentNode]; !exists {
		e.logger.Warn("Invalid current node, resetting to start", map[string]interface{}{
			"userID":      userID,
			"invalidNode": userState.CurrentNode,
		})
		userState.CurrentNode = e.flow.StartNode
	}

	return userState, nil
}

func (e *Engine) determineNextNode(inputText string, userState *state.UserState) (string, string, error) {
	currentNode, exists := e.flow.Nodes[userState.CurrentNode]
	if !exists {
		return "", "", fmt.Errorf("%w: %s", ErrCurrentNodeNotFound, userState.CurrentNode)
	}

	// check global transitions first (higher priority)
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

	// use fallback or default to FALLBACK_MENU
	fallbackNode := e.flow.FallbackNode
	if fallbackNode == "" {
		fallbackNode = "FALLBACK_MENU"
	}

	// ensure fallback node exists
	if _, exists := e.flow.Nodes[fallbackNode]; !exists {
		fallbackNode = e.flow.StartNode
	}

	e.logger.Debug("Using fallback node", map[string]interface{}{
		"currentNode":  userState.CurrentNode,
		"fallbackNode": fallbackNode,
		"input":        inputText,
	})

	return fallbackNode, "", nil
}

func (e *Engine) matchCondition(inputText string, condition Condition) bool {
	// TODO: is this condition check correct?
	if inputText == "" {
		return condition.Type == "any_text" && inputText != ""
	}

	lowerInput := strings.ToLower(inputText)

	switch condition.Type {
	case "exact":
		return e.matchExact(lowerInput, condition.Value)
	case "keyword":
		return e.matchKeyword(lowerInput, condition.Value)
	case "regex":
		return e.matchRegex(lowerInput, condition.Regex)
	case "any_text":
		return true
	default:
		e.logger.Warn("Unknown condition type", map[string]interface{}{
			"type": condition.Type,
		})

		return false
	}
}

func (e *Engine) matchExact(input string, values []string) bool {
	for _, val := range values {
		if strings.EqualFold(input, val) {
			return true
		}
	}

	return false
}

func (e *Engine) matchKeyword(input string, values []string) bool {
	for _, val := range values {
		if strings.Contains(input, strings.ToLower(val)) {
			return true
		}
	}

	return false
}

func (e *Engine) matchRegex(input, pattern string) bool {
	if pattern == "" {
		return false
	}

	regex, exists := e.regexCache[pattern]
	if !exists {
		var err error

		regex, err = regexp.Compile(pattern)
		if err != nil {
			e.logger.Error("Invalid regex pattern", map[string]interface{}{
				"pattern": pattern,
				"error":   err.Error(),
			})

			return false
		}

		e.regexCache[pattern] = regex
	}

	return regex.MatchString(input)
}

func (e *Engine) saveInboundMessage(ctx context.Context, userState *state.UserState, msg *message.Message) error {
	return e.stateManager.SaveMessage(ctx, &state.ConversationMessage{
		UserID:         userState.UserID,
		Timestamp:      time.Now(),
		Direction:      "inbound",
		MessageContent: msg.GetText(),
		NodeID:         userState.CurrentNode,
	})
}

func (e *Engine) saveOutboundMessage(ctx context.Context, userState *state.UserState, response string) error {
	return e.stateManager.SaveMessage(ctx, &state.ConversationMessage{
		UserID:         userState.UserID,
		Timestamp:      time.Now(),
		Direction:      "outbound",
		MessageContent: response,
		NodeID:         userState.CurrentNode,
	})
}
