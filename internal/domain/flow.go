package domain

type MessageContent struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type Condition struct {
	Type    string   `json:"type"`
	Value   []string `json:"value,omitempty"`
	Regex   string   `json:"regex,omitempty"`
	IsMedia bool     `json:"is_media,omitempty"`
}

type Transition struct {
	Condition Condition `json:"condition"`
	Target    string    `json:"target"`
	Action    string    `json:"action,omitempty"`
}

type Node struct {
	Message     MessageContent `json:"message"`
	Transitions []Transition   `json:"transitions"`
	Action      string         `json:"action,omitempty"`
}

type Flow struct {
	StartNode         string          `json:"start_node"`
	FallbackNode      string          `json:"fallback_node,omitempty"`
	Nodes             map[string]Node `json:"nodes"`
	GlobalTransitions []Transition    `json:"global_transitions,omitempty"`
}
