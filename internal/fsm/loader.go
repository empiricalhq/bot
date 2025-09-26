package fsm

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var (
	ErrStartNodeEmpty           = errors.New("start_node cannot be empty")
	ErrStartNodeNotFound        = errors.New("start_node not found in nodes")
	ErrTransitionTargetNotFound = errors.New("transition to non-existent target")
)

func Load(path string) (*Flow, error) {
	flow, err := loadFile(path)
	if err != nil {
		return nil, err
	}

	err = validate(flow)
	if err != nil {
		return nil, fmt.Errorf("invalid conversation flow: %w", err)
	}

	return flow, nil
}

func loadFile(path string) (*Flow, error) {
	cleanPath := filepath.Clean(path)

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read flow file %s: %w", path, err)
	}

	var flow Flow

	err = json.Unmarshal(data, &flow)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal flow: %w", err)
	}

	return &flow, nil
}

func validate(flow *Flow) error {
	if flow.StartNode == "" {
		return ErrStartNodeEmpty
	}

	if _, exists := flow.Nodes[flow.StartNode]; !exists {
		return fmt.Errorf("%w: '%s'", ErrStartNodeNotFound, flow.StartNode)
	}

	// validate all transition targets exist
	for nodeID, node := range flow.Nodes {
		for _, transition := range node.Transitions {
			if _, exists := flow.Nodes[transition.Target]; !exists {
				return fmt.Errorf("%w: from '%s' to '%s'", ErrTransitionTargetNotFound, nodeID, transition.Target)
			}
		}
	}

	return nil
}
