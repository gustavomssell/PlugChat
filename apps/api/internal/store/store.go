package store

import (
	"context"
	"errors"

	"chatfree/api/internal/flow"
	"chatfree/api/internal/model"
)

var ErrNotFound = errors.New("store: not found")

type ConversationFilter struct {
	Status    string
	ChannelID string
	Search    string
	// Limit caps the number of returned rows. Zero means the default of 200.
	Limit int
}

type Store interface {
	CreateUser(ctx context.Context, user model.User, passwordHash string) error
	UserByEmail(ctx context.Context, email string) (model.User, string, error)
	UserByID(ctx context.Context, id string) (model.User, error)
	CountUsers(ctx context.Context) (int, error)

	CreateWorkspace(ctx context.Context, workspace model.Workspace) error
	WorkspaceForUser(ctx context.Context, userID string) (model.Workspace, model.Membership, error)
	CreateMembership(ctx context.Context, workspaceID, userID, role, availability string) error
	Members(ctx context.Context, workspaceID string) ([]model.Agent, error)
	UpdateAvailability(ctx context.Context, workspaceID, userID, availability string) error

	ChannelByKey(ctx context.Context, key string) (model.Channel, error)
	Channels(ctx context.Context, workspaceID string) ([]model.Channel, error)
	Channel(ctx context.Context, workspaceID, id string) (model.Channel, error)
	CreateChannel(ctx context.Context, channel model.Channel) error
	UpdateChannel(ctx context.Context, channel model.Channel) error
	DeleteChannel(ctx context.Context, workspaceID, id string) error

	Flows(ctx context.Context, workspaceID string) ([]flow.Graph, error)
	Flow(ctx context.Context, workspaceID, id string) (flow.Graph, error)
	CreateFlow(ctx context.Context, workspaceID string, graph flow.Graph) error
	SaveFlow(ctx context.Context, workspaceID string, graph flow.Graph) error
	DeleteFlow(ctx context.Context, workspaceID, id string) error

	CreateConversation(ctx context.Context, conversation model.Conversation, state flow.State, snapshot flow.Graph) error
	Conversation(ctx context.Context, id string) (model.Conversation, error)
	ConversationInWorkspace(ctx context.Context, workspaceID, id string) (model.Conversation, error)
	Conversations(ctx context.Context, workspaceID string, filter ConversationFilter) ([]model.Conversation, error)
	DueConversations(ctx context.Context, limit int) ([]model.Conversation, error)
	UpdateConversation(ctx context.Context, conversation model.Conversation, state flow.State) error
	UpdateConversationMeta(ctx context.Context, conversation model.Conversation) error
	State(ctx context.Context, conversationID string) (flow.State, error)
	FlowSnapshot(ctx context.Context, conversationID string) (flow.Graph, error)
	AppendMessages(ctx context.Context, messages ...model.Message) error
	// Messages returns every message of a conversation, notes included. It is
	// the agent-facing view.
	Messages(ctx context.Context, conversationID string) ([]model.Message, error)
	// MessagesForVisitor hides internal notes from the public widget API.
	MessagesForVisitor(ctx context.Context, conversationID string) ([]model.Message, error)
	Stats(ctx context.Context, workspaceID string) (model.Stats, error)
	Metrics(ctx context.Context, workspaceID string, days int) (model.Metrics, error)

	SavedReplies(ctx context.Context, workspaceID string) ([]model.SavedReply, error)
	SavedReply(ctx context.Context, workspaceID, id string) (model.SavedReply, error)
	CreateSavedReply(ctx context.Context, reply model.SavedReply) error
	UpdateSavedReply(ctx context.Context, reply model.SavedReply) error
	DeleteSavedReply(ctx context.Context, workspaceID, id string) error

	CreateAttachment(ctx context.Context, attachment model.Attachment) error
	// Attachment reads one attachment of the workspace, stored name included.
	Attachment(ctx context.Context, workspaceID, id string) (model.Attachment, error)
	// AttachmentsByConversation lists the attachments of a conversation. It is
	// how the public widget proves that a requested file belongs to the
	// conversation it already knows.
	AttachmentsByConversation(ctx context.Context, conversationID string) ([]model.Attachment, error)
	// AttachmentsByIDs returns attachments of the workspace, in creation order.
	// It is how a message claims the files uploaded beforehand.
	AttachmentsByIDs(ctx context.Context, workspaceID string, ids []string) ([]model.Attachment, error)
	// LinkAttachments binds pending uploads to the message that carries them.
	LinkAttachments(ctx context.Context, messageID string, ids []string) error
}
