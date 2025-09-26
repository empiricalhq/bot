package service

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"
	"sync"

	"whatsbot/internal/domain"
	"whatsbot/internal/logger"
)

type FSM interface {
	DetermineNext(state *domain.UserState, input string, hasMedia bool) (nodeID, action string)
	GetNode(nodeID string) *domain.Node
	GetStartNode() string
}

type fsm struct {
	flow       *domain.Flow
	regexCache map[string]*regexp.Regexp
	mutex      sync.RWMutex
	logger     logger.Logger
}

func NewFSM(flow *domain.Flow, logger logger.Logger) FSM {
	return &fsm{
		flow:       flow,
		regexCache: make(map[string]*regexp.Regexp),
		logger:     logger,
	}
}

func (f *fsm) DetermineNext(state *domain.UserState, input string, hasMedia bool) (string, string) {
	input = strings.ToLower(strings.TrimSpace(input))

	// Check global transitions first
	for _, transition := range f.flow.GlobalTransitions {
		if f.matchesCondition(input, hasMedia, transition.Condition) {
			return transition.Target, transition.Action
		}
	}

	// Check node-specific transitions
	currentNode := f.flow.Nodes[state.CurrentNode]
	for _, transition := range currentNode.Transitions {
		if f.matchesCondition(input, hasMedia, transition.Condition) {
			return transition.Target, transition.Action
		}
	}

	// Use fallback
	fallback := f.flow.FallbackNode
	if fallback == "" || f.flow.Nodes[fallback].Message.Content == "" {
		fallback = f.flow.StartNode
	}

	return fallback, ""
}

func (f *fsm) matchesCondition(input string, hasMedia bool, condition domain.Condition) bool {
	switch condition.Type {
	case "exact":
		for _, value := range condition.Value {
			if input == strings.ToLower(value) {
				return true
			}
		}
	case "keyword":
		for _, keyword := range condition.Value {
			if strings.Contains(input, strings.ToLower(keyword)) {
				return true
			}
		}
	case "regex":
		return f.matchesRegex(input, condition.Regex)
	case "any_text":
		return input != "" && !hasMedia
	case "media":
		return hasMedia
	}
	return false
}

func (f *fsm) matchesRegex(input, pattern string) bool {
	if pattern == "" {
		return false
	}

	f.mutex.RLock()
	regex, exists := f.regexCache[pattern]
	f.mutex.RUnlock()

	if !exists {
		f.mutex.Lock()
		// Double-check after acquiring write lock
		if regex, exists = f.regexCache[pattern]; !exists {
			var err error
			regex, err = regexp.Compile(pattern)
			if err != nil {
				f.logger.Error("Invalid regex pattern", "pattern", pattern, "error", err)
				f.mutex.Unlock()
				return false
			}
			f.regexCache[pattern] = regex
		}
		f.mutex.Unlock()
	}

	return regex.MatchString(input)
}

func (f *fsm) GetNode(nodeID string) *domain.Node {
	node, exists := f.flow.Nodes[nodeID]
	if !exists {
		return nil
	}
	return &node
}

func (f *fsm) GetStartNode() string {
	return f.flow.StartNode
}

func LoadFlow(path string) (*domain.Flow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var flow domain.Flow
	err = json.Unmarshal(data, &flow)
	if err != nil {
		return nil, err
	}

	return &flow, validateFlow(&flow)
}

func validateFlow(flow *domain.Flow) error {
	if flow.StartNode == "" {
		return errors.New("start_node cannot be empty")
	}

	if _, exists := flow.Nodes[flow.StartNode]; !exists {
		return errors.New("start_node not found in nodes")
	}

	// Validate all transitions point to existing nodes
	for nodeID, node := range flow.Nodes {
		for _, transition := range node.Transitions {
			if _, exists := flow.Nodes[transition.Target]; !exists {
				return errors.New("invalid transition from " + nodeID + " to " + transition.Target)
			}
		}
	}

	for _, transition := range flow.GlobalTransitions {
		if _, exists := flow.Nodes[transition.Target]; !exists {
			return errors.New("invalid global transition to " + transition.Target)
		}
	}

	return nil
}
