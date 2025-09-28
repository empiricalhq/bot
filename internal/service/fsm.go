package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"

	"whatsbot/internal/domain"
	"whatsbot/internal/message"
	"whatsbot/pkg/utils"
)

type FSM interface {
	DetermineNext(state *domain.UserState, msg *message.Message) (nodeID, action string)
	GetNode(nodeID string) *domain.Node
	GetStartNode() string
}

type fsm struct {
	flow       *domain.Flow
	regexCache map[string]*regexp.Regexp
	mutex      sync.RWMutex
	logger     *slog.Logger
}

func NewFSM(flow *domain.Flow, logger *slog.Logger) FSM {
	return &fsm{
		flow:       flow,
		regexCache: make(map[string]*regexp.Regexp),
		logger:     logger,
	}
}

func (f *fsm) DetermineNext(state *domain.UserState, msg *message.Message) (nodeID, action string) {
	input := strings.ToLower(strings.TrimSpace(msg.Text))

	currentNode, nodeExists := f.flow.Nodes[state.CurrentNode]
	if !nodeExists {
		f.logger.Error("Current node in user state does not exist in flow", "node", state.CurrentNode)

		return f.flow.StartNode, ""
	}

	// Check global transitions first, unless the current node ignores them.
	if !currentNode.IgnoreGlobalTransitions {
		for _, transition := range f.flow.GlobalTransitions {
			if f.matchesCondition(input, msg, transition.Condition) {
				f.logger.Debug("Matched global transition",
					"user", state.UserID, "from_node", state.CurrentNode, "to_node", transition.Target, "action", transition.Action, "condition_type", transition.Condition.Type)

				return transition.Target, transition.Action
			}
		}
	}

	// Check node-specific transitions
	for _, transition := range currentNode.Transitions {
		if f.matchesCondition(input, msg, transition.Condition) {
			action = transition.Action
			if action == "" {
				action = currentNode.Action // fallback
			}

			f.logger.Debug("Matched node transition",
				"user", state.UserID, "from_node", state.CurrentNode, "to_node", transition.Target, "action", action, "condition_type", transition.Condition.Type)

			return transition.Target, action
		}
	}

	// Use fallback
	fallback := f.flow.FallbackNode
	if fallback == "" || f.flow.Nodes[fallback].Message.Content == "" {
		fallback = f.flow.StartNode
	}

	f.logger.Debug("No transition matched, using fallback",
		"user", state.UserID, "from_node", state.CurrentNode, "fallback_node", fallback)

	return fallback, ""
}

func (f *fsm) GetStartNode() string {
	return f.flow.StartNode
}

func (f *fsm) GetNode(nodeID string) *domain.Node {
	node, exists := f.flow.Nodes[nodeID]
	if !exists {
		return nil
	}

	return &node
}

func (f *fsm) matchesCondition(input string, msg *message.Message, condition domain.Condition) bool {
	switch condition.Type {
	case "exact":
		for _, value := range condition.Value {
			if strings.EqualFold(input, value) {
				return true
			}
		}
	case "keyword":
		words := strings.Fields(input)
		for _, keyword := range condition.Value {
			keywordLower := strings.ToLower(keyword)

			if strings.Contains(input, keywordLower) {
				return true
			}
			// Then check with Levenshtein distance for fuzzy matching
			// But be more strict with very short inputs to avoid false positives
			for _, word := range words {
				distance := utils.LevenshteinDistance(keywordLower, word)
				// More restrictive threshold for short words to avoid false positives
				// Este cambio se hizo porque al dar opciones como 1 o 2, el bot redirije a quickresponses
				// parece ser que confunde 1 con un globaltransition que tiene ok como keyword
				threshold := 2
				if len(word) <= 2 || len(keywordLower) <= 2 {
					threshold = 0 // Only exact matches for very short words
				} else if len(word) <= 4 || len(keywordLower) <= 4 {
					threshold = 1 // More strict for short words
				}
				if distance <= threshold {
					return true
				}
			}
		}

	case "regex":
		return f.matchesRegex(input, condition.Regex)
	case "any_text":
		return input != "" && !msg.HasMedia
	case "media":
		return msg.HasMedia
	case "media_type":
		if slices.Contains(condition.Value, msg.MediaType) {
			return true
		}
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

func LoadFlow(path string) (*domain.Flow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read flow file: %w", err)
	}

	var flow domain.Flow

	err = json.Unmarshal(data, &flow)
	if err != nil {
		return nil, fmt.Errorf("failed to parse flow file: %w", err)
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
