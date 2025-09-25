package fsm

type MessageContent struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type Condition struct {
	Type  string   `json:"type"`
	Value []string `json:"value,omitempty"`
	Regex string   `json:"regex,omitempty"`
}

type Transition struct {
	Condition Condition `json:"condition"`
	Target    string    `json:"target"`
	Action    string    `json:"action,omitempty"`
}

type Node struct {
	Message     MessageContent `json:"message"`
	Action      string         `json:"action,omitempty"`
	Transitions []Transition   `json:"transitions"`
}

type Flow struct {
	StartNode         string          `json:"startNode"`
	FallbackNode      string          `json:"fallbackNode,omitempty"`
	Nodes             map[string]Node `json:"nodes"`
	GlobalTransitions []Transition    `json:"globalTransitions,omitempty"`
}
