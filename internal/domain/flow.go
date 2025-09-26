package domain

// MessageContent defines the content of a message sent by the bot.
type MessageContent struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// Condition defines the criteria for a transition to occur.
type Condition struct {
	Type    string   `json:"type"`
	Value   []string `json:"value,omitempty"`
	Regex   string   `json:"regex,omitempty"`
	IsMedia bool     `json:"is_media,omitempty"`
}

// Transition defines a path from one node to another based on a condition.
type Transition struct {
	Condition Condition `json:"condition"`
	Target    string    `json:"target"`
	Action    string    `json:"action,omitempty"`
}

// Node represents a single state in the conversation flow.
type Node struct {
	Message     MessageContent `json:"message"`
	Transitions []Transition   `json:"transitions"`
}

// Flow represents the entire conversation graph.
type Flow struct {
	StartNode         string          `json:"start_node"`
	FallbackNode      string          `json:"fallback_node,omitempty"`
	Nodes             map[string]Node `json:"nodes"`
	GlobalTransitions []Transition    `json:"global_transitions,omitempty"`
}
