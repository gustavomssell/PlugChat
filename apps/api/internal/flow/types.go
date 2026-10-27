package flow

import "time"

type NodeType string

const (
	NodeTrigger      NodeType = "trigger"
	NodeMessage      NodeType = "message"
	NodeCondition    NodeType = "condition"
	NodeCollectInput NodeType = "collect_input"
	NodeLLM          NodeType = "llm"
	NodeHandoff      NodeType = "handoff"
	NodeWebhook      NodeType = "webhook"
	NodeTag          NodeType = "tag"
	NodeAssign       NodeType = "assign"
	NodeDelay        NodeType = "delay"
	NodeEnd          NodeType = "end"
)

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Node struct {
	ID       string         `json:"id"`
	Type     NodeType       `json:"type"`
	Label    string         `json:"label,omitempty"`
	Position Point          `json:"position"`
	Config   map[string]any `json:"config,omitempty"`
}

type Edge struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	Target       string `json:"target"`
	SourceHandle string `json:"sourceHandle,omitempty"`
	Label        string `json:"label,omitempty"`
}

type Graph struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Version     int       `json:"version"`
	EntryNodeID string    `json:"entryNodeId,omitempty"`
	Nodes       []Node    `json:"nodes"`
	Edges       []Edge    `json:"edges"`
	UpdatedAt   time.Time `json:"updatedAt,omitempty"`
}

type Status string

const (
	StatusBot     Status = "bot"
	StatusWaiting Status = "waiting"
	StatusQueued  Status = "queued"
	StatusHuman   Status = "human"
	StatusClosed  Status = "closed"
)

type Option struct {
	Label string `json:"label"`
	Value string `json:"value"`
	// Next is an optional shortcut: when the visitor picks this option the
	// flow jumps straight to the node, skipping the linear edge.
	Next string `json:"nextNodeId,omitempty"`
}

// Field describes one input of a multi-field form shown in a single step, so
// the visitor answers everything with a single interaction.
type Field struct {
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Type     string   `json:"type,omitempty"`
	Required bool     `json:"required,omitempty"`
	Options  []Option `json:"options,omitempty"`
}

// VisitorInput is the untrusted payload of a visitor message. Text carries a
// free-form message or the chosen option; Fields carries a form submission and
// AttachmentIDs reference files uploaded beforehand.
type VisitorInput struct {
	Text          string            `json:"text"`
	Fields        map[string]string `json:"fields,omitempty"`
	AttachmentIDs []string          `json:"attachmentIds,omitempty"`
}

type Wait struct {
	NodeID    string   `json:"nodeId"`
	Variable  string   `json:"variable,omitempty"`
	InputType string   `json:"inputType,omitempty"`
	Options   []Option `json:"options,omitempty"`
	Fields    []Field  `json:"fields,omitempty"`
	// FallbackNodeID receives answers that do not match any option.
	FallbackNodeID string `json:"fallbackNodeId,omitempty"`
}

type State struct {
	ConversationID string         `json:"conversationId"`
	FlowID         string         `json:"flowId"`
	FlowVersion    int            `json:"flowVersion"`
	CurrentNodeID  string         `json:"currentNodeId"`
	Status         Status         `json:"status"`
	Variables      map[string]any `json:"variables"`
	Waiting        *Wait          `json:"waiting,omitempty"`
	PendingNodeID  string         `json:"pendingNodeId,omitempty"`
	WakeAt         *time.Time     `json:"wakeAt,omitempty"`
	// Steps counts executed nodes across the whole conversation and guards
	// against infinite loops created by careless graphs.
	Steps int `json:"steps,omitempty"`
}

type Effect struct {
	Type     string   `json:"type"`
	Text     string   `json:"text,omitempty"`
	Prompt   string   `json:"prompt,omitempty"`
	Variable string   `json:"variable,omitempty"`
	Reason   string   `json:"reason,omitempty"`
	Team     string   `json:"team,omitempty"`
	AgentID  string   `json:"agentId,omitempty"`
	URL      string   `json:"url,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	MS       int      `json:"ms,omitempty"`
}
