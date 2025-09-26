package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"whatsbot/internal/domain"
)

var (
	ErrStartNodeEmpty           = errors.New("start_node cannot be empty")
	ErrStartNodeNotFound        = errors.New("start_node not found in nodes")
	ErrTransitionTargetNotFound = errors.New("transition to non-existent target")
)

// LoadFlow reads, unmarshals, and validates the conversation flow from a JSON file.
func LoadFlow(path string) (*domain.Flow, error) {
	cleanPath := filepath.Clean(path)

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read flow file %s: %w", path, err)
	}

	var flow domain.Flow
	if err := json.Unmarshal(data, &flow); err != nil {
		return nil, fmt.Errorf("failed to unmarshal flow: %w", err)
	}

	if err := validate(&flow); err != nil {
		return nil, fmt.Errorf("invalid conversation flow: %w", err)
	}

	return &flow, nil
}

func validate(flow *domain.Flow) error {
	if flow.StartNode == "" {
		return ErrStartNodeEmpty
	}

	if _, exists := flow.Nodes[flow.StartNode]; !exists {
		return fmt.Errorf("%w: '%s'", ErrStartNodeNotFound, flow.StartNode)
	}

	allNodes := flow.Nodes
	for nodeID, node := range allNodes {
		for i, transition := range node.Transitions {
			if _, exists := allNodes[transition.Target]; !exists {
				return fmt.Errorf("%w: from node '%s' (transition %d) to '%s'",
					ErrTransitionTargetNotFound, nodeID, i, transition.Target)
			}
		}
	}

	for i, transition := range flow.GlobalTransitions {
		if _, exists := allNodes[transition.Target]; !exists {
			return fmt.Errorf("%w: from global transition %d to '%s'",
				ErrTransitionTargetNotFound, i, transition.Target)
		}
	}

	return nil
}
