package service

import (
	"strings"

	"whatsbot/internal/domain"
	"whatsbot/internal/logger"
	"whatsbot/internal/util"
)

type FSM interface {
	DetermineNextNode(userState *domain.UserState, inputText string, hasMedia bool) (nextNodeID, actionName string)
	GetFlow() *domain.Flow
}

type FsmEngine struct {
	flow       *domain.Flow
	logger     *logger.Logger
	regexCache *util.RegexCache
}

func NewFsmEngine(
	flow *domain.Flow,
	regexCache *util.RegexCache,
	log *logger.Logger,
) FSM {
	return &FsmEngine{
		flow:       flow,
		logger:     log,
		regexCache: regexCache,
	}
}

func (e *FsmEngine) GetFlow() *domain.Flow {
	return e.flow
}

func (e *FsmEngine) DetermineNextNode(
	userState *domain.UserState,
	inputText string,
	hasMedia bool,
) (nextNodeID, actionName string) {
	if _, exists := e.flow.Nodes[userState.CurrentNode]; !exists {
		e.logger.Warn("User in invalid node, resetting to start node", map[string]interface{}{
			"userID":      userState.UserID,
			"invalidNode": userState.CurrentNode,
		})
		userState.CurrentNode = e.flow.StartNode
	}

	lowerInput := strings.ToLower(inputText)

	// check global transitions first (higher priority).
	for _, transition := range e.flow.GlobalTransitions {
		if e.matchCondition(lowerInput, hasMedia, transition.Condition) {
			return transition.Target, transition.Action
		}
	}

	// check node-specific transitions.
	currentNode := e.flow.Nodes[userState.CurrentNode]
	for _, transition := range currentNode.Transitions {
		if e.matchCondition(lowerInput, hasMedia, transition.Condition) {
			return transition.Target, transition.Action
		}
	}

	// if no match, use the fallback node.
	return e.getFallbackNode(), ""
}

func (e *FsmEngine) getFallbackNode() string {
	fallbackNode := e.flow.FallbackNode
	if fallbackNode == "" {
		fallbackNode = "FALLBACK_MENU"
	}

	if _, exists := e.flow.Nodes[fallbackNode]; !exists {
		return e.flow.StartNode // if all else fails, go to start node.
	}

	return fallbackNode
}

func (e *FsmEngine) matchCondition(lowerInput string, hasMedia bool, condition domain.Condition) bool {
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
		return e.matchRegex(lowerInput, condition.Regex)
	case "any_text":
		return lowerInput != "" && !hasMedia
	case "media":
		return hasMedia
	default:
		e.logger.Warn("Unknown condition type", map[string]interface{}{"type": condition.Type})

		return false
	}
}

func (e *FsmEngine) matchRegex(input, pattern string) bool {
	if pattern == "" {
		return false
	}

	regex, err := e.regexCache.Get(pattern)
	if err != nil {
		e.logger.Error("Invalid regex pattern in flow", map[string]interface{}{
			"pattern": pattern,
			"error":   err.Error(),
		})

		return false
	}

	return regex.MatchString(input)
}
