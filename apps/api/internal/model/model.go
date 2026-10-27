package model

import (
	"time"

	"chatfree/api/internal/flow"
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type Workspace struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type Membership struct {
	Role         string `json:"role"`
	Availability string `json:"availability"`
}

type AuthResult struct {
	User       User       `json:"user"`
	Workspace  Workspace  `json:"workspace"`
	Membership Membership `json:"membership"`
}

type AuthResponse struct {
	Token string `json:"token"`
	AuthResult
}

type Agent struct {
	UserID       string `json:"userId"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	Availability string `json:"availability"`
}

type Appearance struct {
	Mode          string `json:"mode"`
	Position      string `json:"position"`
	Theme         string `json:"theme"`
	Accent        string `json:"accent"`
	Greeting      string `json:"greeting"`
	LauncherLabel string `json:"launcherLabel"`
	ShowBranding  bool   `json:"showBranding"`
}

var DefaultAppearance = Appearance{
	Mode:          "bubble",
	Position:      "bottom-right",
	Theme:         "light",
	Accent:        "#4f46e5",
	Greeting:      "Olá! Como podemos ajudar você hoje?",
	LauncherLabel: "Fale com a gente",
	ShowBranding:  true,
}

type Channel struct {
	ID             string     `json:"id"`
	WorkspaceID    string     `json:"workspaceId"`
	Key            string     `json:"key"`
	Name           string     `json:"name"`
	FlowID         string     `json:"flowId"`
	Appearance     Appearance `json:"appearance"`
	AllowedOrigins []string   `json:"allowedOrigins"`
	CreatedAt      time.Time  `json:"createdAt"`
}

type Conversation struct {
	ID             string      `json:"id"`
	WorkspaceID    string      `json:"workspaceId"`
	ChannelID      string      `json:"channelId"`
	VisitorID      string      `json:"visitorId"`
	Status         flow.Status `json:"status"`
	FlowID         string      `json:"flowId"`
	FlowVersion    int         `json:"flowVersion"`
	CurrentNodeID  string      `json:"currentNodeId"`
	AssignedUserID string      `json:"assignedUserId,omitempty"`
	Tags           []string    `json:"tags"`
	Waiting        *flow.Wait  `json:"waiting,omitempty"`
	CreatedAt      time.Time   `json:"createdAt"`
	UpdatedAt      time.Time   `json:"updatedAt"`
}

// Message roles. Notes exist only for the team: they never reach the visitor
// through the widget API.
const (
	RoleVisitor = "visitor"
	RoleBot     = "bot"
	RoleAgent   = "agent"
	RoleSystem  = "system"
	RoleNote    = "note"
)

// Attachment kinds accepted by the upload endpoints.
const (
	KindImage = "image"
	KindAudio = "audio"
	KindFile  = "file"
)

type Message struct {
	ID             string       `json:"id"`
	ConversationID string       `json:"conversationId"`
	Role           string       `json:"role"`
	Text           string       `json:"text"`
	AuthorUserID   string       `json:"authorUserId,omitempty"`
	AuthorName     string       `json:"authorName,omitempty"`
	Attachments    []Attachment `json:"attachments,omitempty"`
	CreatedAt      time.Time    `json:"createdAt"`
}

type Attachment struct {
	ID             string `json:"id"`
	WorkspaceID    string `json:"-"`
	ConversationID string `json:"conversationId"`
	Kind           string `json:"kind"`
	Filename       string `json:"filename"`
	Mime           string `json:"mime"`
	SizeBytes      int64  `json:"sizeBytes"`
	// StoredName is the on-disk file name (id plus extension). It never leaves
	// the API: clients download through the attachment URL.
	StoredName string    `json:"-"`
	CreatedAt  time.Time `json:"createdAt"`
}

type SavedReply struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspaceId"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	Shortcut    string    `json:"shortcut"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ChannelMetric aggregates conversations and first response time per channel.
type ChannelMetric struct {
	ChannelID       string `json:"channelId"`
	Name            string `json:"name"`
	Conversations   int    `json:"conversations"`
	Closed          int    `json:"closed"`
	FirstResponseMS int64  `json:"firstResponseMs"`
}

// AgentMetric aggregates the workload of a single workspace member.
type AgentMetric struct {
	UserID          string `json:"userId"`
	Name            string `json:"name"`
	Conversations   int    `json:"conversations"`
	Closed          int    `json:"closed"`
	FirstResponseMS int64  `json:"firstResponseMs"`
}

// Metrics is the report shown by the dashboard reports page.
type Metrics struct {
	Days               int             `json:"days"`
	Conversations      int             `json:"conversations"`
	Closed             int             `json:"closed"`
	Messages           int             `json:"messages"`
	FirstResponseMS    int64           `json:"firstResponseMs"`
	ConversationsByDay []DayCount      `json:"conversationsByDay"`
	MessagesByDay      []DayCount      `json:"messagesByDay"`
	ByChannel          []ChannelMetric `json:"byChannel"`
	ByAgent            []AgentMetric   `json:"byAgent"`
}

type FlowSummary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Version   int       `json:"version"`
	Nodes     int       `json:"nodes"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type DayCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type Stats struct {
	Total              int        `json:"total"`
	Active             int        `json:"active"`
	Waiting            int        `json:"waiting"`
	Closed             int        `json:"closed"`
	MessagesToday      int        `json:"messagesToday"`
	ConversationsByDay []DayCount `json:"conversationsByDay"`
}
