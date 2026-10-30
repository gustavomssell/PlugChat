package chat

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"chatfree/api/internal/files"
	"chatfree/api/internal/id"
	"chatfree/api/internal/model"
	"chatfree/api/internal/store"
)

const (
	// MaxNoteRunes bounds an internal note, matching the message ceiling.
	MaxNoteRunes = 4000
	// MaxBulkIDs caps how many conversations one bulk request may touch.
	MaxBulkIDs = 100
	// MaxAttachmentsPerMessage caps files carried by a single message.
	MaxAttachmentsPerMessage = 5
	// MaxAttachmentBytes caps one uploaded file.
	MaxAttachmentBytes = 8 << 20
	// MaxSavedReplies caps the library of a workspace.
	MaxSavedReplies = 200
)

// Bulk actions accepted by the inbox batch endpoint.
const (
	BulkAssign = "assign"
	BulkTag    = "tag"
	BulkStatus = "status"
)

var (
	ErrInvalidNote       = errors.New("note is empty or too long")
	ErrBulkTooManyIDs    = errors.New("too many conversations in one request")
	ErrInvalidBulkAction = errors.New("invalid bulk action")
	ErrInvalidReply      = errors.New("saved reply needs a title and a body")
	ErrInvalidShortcut   = errors.New("shortcut must use lowercase letters, numbers and dashes")
	ErrTooManyReplies    = errors.New("saved reply limit reached")
	ErrReplyNotFound     = errors.New("saved reply not found")

	ErrAttachmentNotFound          = errors.New("attachment not found")
	ErrAttachmentTooLarge          = errors.New("attachment is too large")
	ErrInvalidAttachmentKind       = errors.New("unsupported attachment kind")
	ErrTooManyAttachments          = errors.New("too many attachments for one message")
	ErrAttachmentNotInConversation = errors.New("attachment belongs to another conversation")
)

var shortcutPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

// AuthorName resolves how an internal note is signed in the inbox.
func (s *Service) authorName(ctx context.Context, workspaceID, userID string) string {
	members, err := s.store.Members(ctx, workspaceID)
	if err != nil {
		return "Atendente"
	}
	for _, member := range members {
		if member.UserID == userID {
			return member.Name
		}
	}
	return "Atendente"
}

// AddNote stores an internal note: visible to the team, never to the visitor.
// Notes are published only on the workspace inbox topic, so the widget stream
// of the same conversation stays untouched.
func (s *Service) AddNote(ctx context.Context, workspaceID, userID, conversationID, text string) (model.Message, error) {
	text = strings.TrimSpace(text)
	if text == "" || utf8.RuneCountInString(text) > MaxNoteRunes {
		return model.Message{}, ErrInvalidNote
	}

	if _, err := s.store.ConversationInWorkspace(ctx, workspaceID, conversationID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return model.Message{}, ErrConversationNotFound
		}
		return model.Message{}, err
	}

	note := model.Message{
		ID:             id.New("msg"),
		ConversationID: conversationID,
		Role:           model.RoleNote,
		Text:           text,
		AuthorUserID:   userID,
		AuthorName:     s.authorName(ctx, workspaceID, userID),
		CreatedAt:      time.Now().UTC(),
	}

	if err := s.store.AppendMessages(ctx, note); err != nil {
		return model.Message{}, err
	}

	s.hub.Publish(InboxTopic(workspaceID), "note", note)

	return note, nil
}

// BulkPatch applies one action to many conversations at once. Unknown or
// foreign conversations are skipped; the caller learns how many were updated.
// Each individual patch goes through Patch, so validation and SSE events stay
// identical to the single-conversation flow.
func (s *Service) BulkPatch(ctx context.Context, workspaceID string, action BulkAction) (int, error) {
	seen := make(map[string]struct{}, len(action.IDs))
	ids := make([]string, 0, len(action.IDs))
	for _, raw := range action.IDs {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		ids = append(ids, item)
	}

	if len(ids) == 0 || len(ids) > MaxBulkIDs {
		return 0, ErrBulkTooManyIDs
	}

	if !oneOf(action.Action, BulkAssign, BulkTag, BulkStatus) {
		return 0, ErrInvalidBulkAction
	}

	var patch Patch
	switch action.Action {
	case BulkAssign:
		assignee := strings.TrimSpace(action.AssignedUserID)
		if assignee == "" {
			return 0, ErrInvalidAssignee
		}
		patch.AssignedUserID = &assignee
		// Atribuir costuma significar "assumir": um status opcional acompanha.
		if status := strings.TrimSpace(action.Status); status != "" {
			if !oneOf(status, "bot", "waiting", "queued", "human", "closed") {
				return 0, ErrInvalidStatus
			}
			patch.Status = &status
		}
	case BulkTag:
		tags := sanitizeTags(action.Tags)
		patch.Tags = &tags
	case BulkStatus:
		status := strings.TrimSpace(action.Status)
		if !oneOf(status, "bot", "waiting", "queued", "human", "closed") {
			return 0, ErrInvalidStatus
		}
		patch.Status = &status
	}

	updated := 0
	for _, conversationID := range ids {
		if _, err := s.Patch(ctx, workspaceID, conversationID, patch); err != nil {
			if errors.Is(err, ErrConversationNotFound) {
				continue
			}
			return updated, err
		}
		updated++
	}

	return updated, nil
}

// BulkAction is the payload of the batch endpoint.
type BulkAction struct {
	IDs            []string
	Action         string
	AssignedUserID string
	Tags           []string
	Status         string
}

type SavedReplyInput struct {
	Title    string
	Body     string
	Shortcut string
	Tags     []string
}

func (s *Service) SavedReplies(ctx context.Context, workspaceID string) ([]model.SavedReply, error) {
	return s.store.SavedReplies(ctx, workspaceID)
}

func (s *Service) CreateSavedReply(ctx context.Context, workspaceID string, input SavedReplyInput) (model.SavedReply, error) {
	reply, err := buildReply(workspaceID, input)
	if err != nil {
		return model.SavedReply{}, err
	}

	existing, err := s.store.SavedReplies(ctx, workspaceID)
	if err != nil {
		return model.SavedReply{}, err
	}
	if len(existing) >= MaxSavedReplies {
		return model.SavedReply{}, ErrTooManyReplies
	}
	for _, item := range existing {
		if item.Shortcut != "" && item.Shortcut == reply.Shortcut {
			return model.SavedReply{}, ErrInvalidShortcut
		}
	}

	if err := s.store.CreateSavedReply(ctx, reply); err != nil {
		return model.SavedReply{}, err
	}
	return reply, nil
}

func (s *Service) UpdateSavedReply(ctx context.Context, workspaceID, replyID string, input SavedReplyInput) (model.SavedReply, error) {
	current, err := s.store.SavedReply(ctx, workspaceID, replyID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return model.SavedReply{}, ErrReplyNotFound
		}
		return model.SavedReply{}, err
	}

	reply, err := buildReply(workspaceID, input)
	if err != nil {
		return model.SavedReply{}, err
	}

	existing, err := s.store.SavedReplies(ctx, workspaceID)
	if err != nil {
		return model.SavedReply{}, err
	}
	for _, item := range existing {
		if item.ID != replyID && item.Shortcut != "" && item.Shortcut == reply.Shortcut {
			return model.SavedReply{}, ErrInvalidShortcut
		}
	}

	reply.ID = current.ID
	reply.CreatedAt = current.CreatedAt

	if err := s.store.UpdateSavedReply(ctx, reply); err != nil {
		return model.SavedReply{}, err
	}
	return reply, nil
}

func (s *Service) DeleteSavedReply(ctx context.Context, workspaceID, replyID string) error {
	if err := s.store.DeleteSavedReply(ctx, workspaceID, replyID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrReplyNotFound
		}
		return err
	}
	return nil
}

func buildReply(workspaceID string, input SavedReplyInput) (model.SavedReply, error) {
	title := strings.TrimSpace(input.Title)
	body := strings.TrimSpace(input.Body)
	shortcut := strings.ToLower(strings.TrimSpace(input.Shortcut))

	if len(title) < 2 || len(title) > 80 || body == "" {
		return model.SavedReply{}, ErrInvalidReply
	}
	if utf8.RuneCountInString(body) > MaxNoteRunes {
		return model.SavedReply{}, ErrInvalidReply
	}
	if shortcut != "" && !shortcutPattern.MatchString(shortcut) {
		return model.SavedReply{}, ErrInvalidShortcut
	}

	now := time.Now().UTC()
	return model.SavedReply{
		ID:          id.New("reply"),
		WorkspaceID: workspaceID,
		Title:       title,
		Body:        body,
		Shortcut:    shortcut,
		Tags:        sanitizeTags(input.Tags),
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (s *Service) Metrics(ctx context.Context, workspaceID string, days int) (model.Metrics, error) {
	return s.store.Metrics(ctx, workspaceID, days)
}

// sanitizeFilename strips path separators and control characters from a
// client-provided file name.
func sanitizeFilename(filename string) string {
	filename = strings.TrimSpace(filename)
	filename = truncateRunes(filename, 200)
	filename = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == '/' || r == '\\' || r == ':' {
			return -1
		}
		return r
	}, filename)
	// Pontos iniciais só esconderiam a extensão real de um upload.
	return strings.TrimLeft(strings.TrimSpace(filename), ".")
}

// AttachmentInput carries the metadata of an upload that passed validation in
// the HTTP layer.
type AttachmentInput struct {
	Kind     string
	Filename string
	Mime     string
	Size     int64
}

var allowedAttachmentKinds = []string{model.KindImage, model.KindAudio, model.KindFile}

// AddAttachment registers an uploaded file as a pending attachment of the
// conversation. A message claims it later through attachmentIds.
func (s *Service) AddAttachment(ctx context.Context, workspaceID, conversationID string, input AttachmentInput) (model.Attachment, error) {
	conversation, err := s.store.ConversationInWorkspace(ctx, workspaceID, conversationID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return model.Attachment{}, ErrConversationNotFound
		}
		return model.Attachment{}, err
	}

	if !oneOf(strings.TrimSpace(input.Kind), allowedAttachmentKinds...) {
		return model.Attachment{}, ErrInvalidAttachmentKind
	}
	filename := sanitizeFilename(input.Filename)
	if filename == "" {
		return model.Attachment{}, ErrInvalidAttachmentKind
	}
	if files.ExtensionOf(filename) == "" {
		return model.Attachment{}, ErrInvalidAttachmentKind
	}
	if input.Size <= 0 || input.Size > MaxAttachmentBytes {
		return model.Attachment{}, ErrAttachmentTooLarge
	}

	// O arquivo em disco se chama id + extensão derivada do nome original.
	attachmentID := id.New("att")
	extension := files.ExtensionOf(filename)

	attachment := model.Attachment{
		ID:             attachmentID,
		WorkspaceID:    conversation.WorkspaceID,
		ConversationID: conversation.ID,
		Kind:           strings.TrimSpace(input.Kind),
		Filename:       filename,
		Mime:           strings.TrimSpace(input.Mime),
		SizeBytes:      input.Size,
		StoredName:     attachmentID + extension,
		CreatedAt:      time.Now().UTC(),
	}

	if err := s.store.CreateAttachment(ctx, attachment); err != nil {
		return model.Attachment{}, err
	}
	return attachment, nil
}

// claimAttachments resolves the uploads referenced by a message. Every id must
// exist, belong to the workspace and already point at this conversation.
func (s *Service) claimAttachments(ctx context.Context, workspaceID, conversationID string, ids []string) ([]model.Attachment, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) > MaxAttachmentsPerMessage {
		return nil, ErrTooManyAttachments
	}

	attachments, err := s.store.AttachmentsByIDs(ctx, workspaceID, ids)
	if err != nil {
		return nil, err
	}
	if len(attachments) != len(ids) {
		return nil, ErrAttachmentNotFound
	}
	for _, attachment := range attachments {
		if attachment.ConversationID != conversationID {
			return nil, ErrAttachmentNotInConversation
		}
	}

	return attachments, nil
}
