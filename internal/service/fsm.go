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
		// User is in a node that no longer exists => reset to start.
		f.logger.Error("Current node in user state does not exist in flow", "node", state.CurrentNode)

		return f.flow.StartNode, ""
	}

	// 1. Try node-specific transitions first to prioritize context.
	for _, transition := range currentNode.Transitions {
		if f.matchesCondition(input, msg, transition.Condition) {
			action = transition.Action
			if action == "" {
				action = currentNode.Action // fallback to node's default action
			}

			f.logger.Debug("Matched node transition",
				"user", state.UserID, "from_node", state.CurrentNode, "to_node", transition.Target, "action", action, "condition_type", transition.Condition.Type)

			return transition.Target, action
		}
	}

	// 2. If no local transition matches, try global transitions (unless this node explicitly ignores them).
	if !currentNode.IgnoreGlobalTransitions {
		for _, transition := range f.flow.GlobalTransitions {
			if f.matchesCondition(input, msg, transition.Condition) {
				f.logger.Debug("Matched global transition",
					"user", state.UserID, "from_node", state.CurrentNode, "to_node", transition.Target, "action", transition.Action, "condition_type", transition.Condition.Type)

				return transition.Target, transition.Action
			}
		}
	}

	// 3. If no transition matched, handle special fallback cases.
	// Example: user sends a video where an image (voucher) was expected.
	if msg.HasMedia {
		for _, transition := range currentNode.Transitions {
			if transition.Condition.Type == "media_type" {
				f.logger.Debug("Matched specific fallback for wrong media type",
					"user", state.UserID, "from_node", state.CurrentNode, "sent_media", msg.MediaType, "expected_media", transition.Condition.Value)

				return state.CurrentNode, "trigger_fallback_wrong_media"
			}
		}
	}

	// 4. Nothing matched => remain in current node and trigger generic fallback response.
	f.logger.Debug("No transition matched, staying in current node and triggering fallback response",
		"user", state.UserID, "from_node", state.CurrentNode)

	// Special action tells caller to send fallback message but keep state unchanged.
	return state.CurrentNode, "trigger_fallback_response"
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
		// Match only if input equals one of the values (case-insensitive).
		for _, value := range condition.Value {
			if strings.EqualFold(input, value) {
				return true
			}
		}

	case "keyword":
		// Quick check: input must contain a keyword as a substring.
		// Example: "I need help" matches keyword "help".
		for _, keyword := range condition.Value {
			if strings.Contains(input, strings.ToLower(keyword)) {
				return true
			}
		}

		// Fallback: fuzzy match each word in input against keywords.
		// Uses Levenshtein distance with stricter thresholds for short words
		// to avoid false positives (e.g., confusing "1" with "ok").
		words := strings.Fields(input)
		for _, keyword := range condition.Value {
			keywordLower := strings.ToLower(keyword)

			for _, word := range words {
				distance := utils.LevenshteinDistance(keywordLower, word)

				// Threshold rules:
				// - Very short words (=< 2): exact match only
				// - Short words (=< 4): allow distance 1
				// - Otherwise: allow distance 2
				threshold := 2
				if len(word) <= 2 || len(keywordLower) <= 2 {
					threshold = 0
				} else if len(word) <= 4 || len(keywordLower) <= 4 {
					threshold = 1
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
		var err error
		if regex, exists = f.regexCache[pattern]; !exists {
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

	err = processTransitionIncludes(&flow)
	if err != nil {
		return nil, fmt.Errorf("failed to process transition includes: %w", err)
	}

	return &flow, validateFlow(&flow)
}

// processTransitionIncludes merges shared transitions from TransitionGroups into nodes.
func processTransitionIncludes(flow *domain.Flow) error {
	if len(flow.TransitionGroups) == 0 {
		return nil
	}

	processedNodes := make(map[string]domain.Node, len(flow.Nodes))
	for nodeID, node := range flow.Nodes {
		if node.IncludeTransitions != "" {
			groupName := node.IncludeTransitions

			group, ok := flow.TransitionGroups[groupName]
			if !ok {
				return fmt.Errorf("node %q includes non-existent transition group %q", nodeID, groupName)
			}

			// Prepend the group transitions so that node-specific transitions are checked last.
			node.Transitions = append(group, node.Transitions...)
		}

		processedNodes[nodeID] = node
	}

	flow.Nodes = processedNodes

	return nil
}

func validateFlow(flow *domain.Flow) error {
	if flow.StartNode == "" {
		return errors.New("start_node cannot be empty")
	}

	_, exists := flow.Nodes[flow.StartNode]
	if !exists {
		return errors.New("start_node not found in nodes")
	}

	// Helper to check that all transitions in a slice point to existing nodes.
	checkTransitions := func(transitions []domain.Transition, source string) error {
		for _, transition := range transitions {
			if _, exists := flow.Nodes[transition.Target]; !exists {
				return fmt.Errorf("invalid transition from %s to non-existent node %q", source, transition.Target)
			}
		}

		return nil
	}

	// Validate global transitions.
	err := checkTransitions(flow.GlobalTransitions, "global_transitions")
	if err != nil {
		return err
	}

	// Validate transitions for each node.
	for nodeID, node := range flow.Nodes {
		source := fmt.Sprintf("node %q", nodeID)

		err = checkTransitions(node.Transitions, source)
		if err != nil {
			return err
		}
	}

	return nil
}
