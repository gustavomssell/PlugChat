package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"chatfree/api/internal/flow"
	"chatfree/api/internal/id"
	"chatfree/api/internal/llm"
	"chatfree/api/internal/model"
	"chatfree/api/internal/realtime"
	"chatfree/api/internal/store"
)

var (
	ErrChannelNotFound      = errors.New("channel not found")
	ErrConversationNotFound = errors.New("conversation not found")
	ErrEmptyMessage         = errors.New("message is empty")
	ErrMessageTooLong       = errors.New("message is too long")
	ErrInvalidVisitorID     = errors.New("invalid visitor id")
	ErrInvalidStatus        = errors.New("invalid status")
	ErrInvalidAssignee      = errors.New("invalid assignee")
)

// Limits bounds the untrusted input accepted by the chat service. The widget
// is public, so every visitor-provided field needs a hard ceiling.
type Limits struct {
	MaxMessageRunes   int
	MaxVisitorIDRunes int
}

func DefaultLimits() Limits {
	return Limits{
		MaxMessageRunes:   4000,
		MaxVisitorIDRunes: 128,
	}
}

func (l Limits) normalized() Limits {
	defaults := DefaultLimits()
	if l.MaxMessageRunes <= 0 {
		l.MaxMessageRunes = defaults.MaxMessageRunes
	}
	if l.MaxVisitorIDRunes <= 0 {
		l.MaxVisitorIDRunes = defaults.MaxVisitorIDRunes
	}
	return l
}

type Patch struct {
	Status         *string
	AssignedUserID *string
	Tags           *[]string
}

type Option func(*Service)

type Service struct {
	store  store.Store
	hub    *realtime.Hub
	llm    llm.Client
	client *http.Client
	limits Limits
}

func WithLLM(client llm.Client) Option {
	return func(s *Service) {
		s.llm = client
	}
}

// WithLimits overrides the default message and visitor ID ceilings.
func WithLimits(limits Limits) Option {
	return func(s *Service) {
		s.limits = limits.normalized()
	}
}

func New(st store.Store, hub *realtime.Hub, options ...Option) *Service {
	service := &Service{
		store:  st,
		hub:    hub,
		client: &http.Client{Timeout: 8 * time.Second},
		limits: DefaultLimits(),
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func ConversationTopic(conversationID string) string {
	return "conversation:" + conversationID
}

func InboxTopic(workspaceID string) string {
	return "inbox:" + workspaceID
}

func (s *Service) StartConversation(ctx context.Context, channelKey, visitorID string) (model.Conversation, []model.Message, error) {
	channel, err := s.store.ChannelByKey(ctx, channelKey)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return model.Conversation{}, nil, ErrChannelNotFound
		}
		return model.Conversation{}, nil, err
	}

	graph, err := s.store.Flow(ctx, channel.WorkspaceID, channel.FlowID)
	if err != nil {
		return model.Conversation{}, nil, err
	}

	if strings.TrimSpace(visitorID) == "" {
		visitorID = id.New("visitor")
	} else {
		visitorID = strings.TrimSpace(visitorID)
		if !s.validVisitorID(visitorID) {
			return model.Conversation{}, nil, ErrInvalidVisitorID
		}
	}

	now := time.Now().UTC()
	conversation := model.Conversation{
		ID:          id.New("conv"),
		WorkspaceID: channel.WorkspaceID,
		ChannelID:   channel.ID,
		VisitorID:   visitorID,
		Status:      flow.StatusBot,
		FlowID:      graph.ID,
		FlowVersion: graph.Version,
		Tags:        []string{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	state, effects, err := flow.Start(graph, conversation.ID)
	if err != nil {
		return model.Conversation{}, nil, err
	}

	conversation.Status = state.Status
	conversation.CurrentNodeID = state.CurrentNodeID
	conversation.Waiting = state.Waiting

	messages, err := s.applyEffects(ctx, &conversation, &state, effects)
	if err != nil {
		return model.Conversation{}, nil, err
	}

	if err := s.store.CreateConversation(ctx, conversation, state, graph); err != nil {
		return model.Conversation{}, nil, err
	}
	if err := s.store.AppendMessages(ctx, messages...); err != nil {
		return model.Conversation{}, nil, err
	}

	s.publish(conversation, messages)

	return conversation, messages, nil
}

func (s *Service) VisitorMessage(ctx context.Context, conversationID string, input flow.VisitorInput) (model.Conversation, []model.Message, error) {
	text := strings.TrimSpace(input.Text)
	if text == "" && len(input.Fields) == 0 && len(input.AttachmentIDs) == 0 {
		return model.Conversation{}, nil, ErrEmptyMessage
	}
	if text != "" && utf8.RuneCountInString(text) > s.limits.MaxMessageRunes {
		return model.Conversation{}, nil, ErrMessageTooLong
	}

	conversation, err := s.store.Conversation(ctx, conversationID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return model.Conversation{}, nil, ErrConversationNotFound
		}
		return model.Conversation{}, nil, err
	}

	// Anexos são validados contra a conversa antes de qualquer gravação.
	attachments, err := s.claimAttachments(ctx, conversation.WorkspaceID, conversationID, input.AttachmentIDs)
	if err != nil {
		return model.Conversation{}, nil, err
	}

	state, err := s.store.State(ctx, conversationID)
	if err != nil {
		return model.Conversation{}, nil, err
	}

	now := time.Now().UTC()
	visitorMessage := model.Message{
		ID:             id.New("msg"),
		ConversationID: conversationID,
		Role:           "visitor",
		Text:           visitorText(input),
		Attachments:    attachments,
		CreatedAt:      now,
	}

	conversation.UpdatedAt = now

	if state.Status != flow.StatusWaiting || conversation.Status == flow.StatusHuman || conversation.Status == flow.StatusClosed {
		if err := s.store.AppendMessages(ctx, visitorMessage); err != nil {
			return model.Conversation{}, nil, err
		}
		if err := s.store.LinkAttachments(ctx, visitorMessage.ID, input.AttachmentIDs); err != nil {
			return model.Conversation{}, nil, err
		}
		if err := s.store.UpdateConversationMeta(ctx, conversation); err != nil {
			return model.Conversation{}, nil, err
		}
		s.publish(conversation, []model.Message{visitorMessage})
		return conversation, nil, nil
	}

	graph, err := s.store.FlowSnapshot(ctx, conversationID)
	if err != nil {
		return model.Conversation{}, nil, err
	}

	nextState, effects, err := flow.ResumeInput(graph, state, input)
	if err != nil {
		return model.Conversation{}, nil, err
	}

	conversation.Status = nextState.Status
	conversation.CurrentNodeID = nextState.CurrentNodeID
	conversation.FlowVersion = nextState.FlowVersion
	conversation.Waiting = nextState.Waiting

	messages, err := s.applyEffects(ctx, &conversation, &nextState, effects)
	if err != nil {
		return model.Conversation{}, nil, err
	}

	if err := s.store.AppendMessages(ctx, append([]model.Message{visitorMessage}, messages...)...); err != nil {
		return model.Conversation{}, nil, err
	}
	if err := s.store.LinkAttachments(ctx, visitorMessage.ID, input.AttachmentIDs); err != nil {
		return model.Conversation{}, nil, err
	}
	if err := s.store.UpdateConversation(ctx, conversation, nextState); err != nil {
		return model.Conversation{}, nil, err
	}

	s.publish(conversation, append([]model.Message{visitorMessage}, messages...))

	return conversation, messages, nil
}

func (s *Service) AgentReply(ctx context.Context, workspaceID, userID, conversationID, text string, attachmentIDs []string) (model.Conversation, []model.Message, error) {
	text = strings.TrimSpace(text)
	if text == "" && len(attachmentIDs) == 0 {
		return model.Conversation{}, nil, ErrEmptyMessage
	}
	if utf8.RuneCountInString(text) > s.limits.MaxMessageRunes {
		return model.Conversation{}, nil, ErrMessageTooLong
	}

	conversation, err := s.store.ConversationInWorkspace(ctx, workspaceID, conversationID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return model.Conversation{}, nil, ErrConversationNotFound
		}
		return model.Conversation{}, nil, err
	}

	attachments, err := s.claimAttachments(ctx, workspaceID, conversationID, attachmentIDs)
	if err != nil {
		return model.Conversation{}, nil, err
	}

	state, err := s.store.State(ctx, conversationID)
	if err != nil {
		return model.Conversation{}, nil, err
	}

	state.Status = flow.StatusHuman
	state.Waiting = nil
	state.PendingNodeID = ""
	state.WakeAt = nil

	conversation.Status = flow.StatusHuman
	conversation.Waiting = nil
	if conversation.AssignedUserID == "" {
		conversation.AssignedUserID = userID
	}
	conversation.UpdatedAt = time.Now().UTC()

	message := model.Message{
		ID:             id.New("msg"),
		ConversationID: conversationID,
		Role:           "agent",
		Text:           text,
		Attachments:    attachments,
		CreatedAt:      conversation.UpdatedAt,
	}

	if err := s.store.UpdateConversation(ctx, conversation, state); err != nil {
		return model.Conversation{}, nil, err
	}
	if err := s.store.AppendMessages(ctx, message); err != nil {
		return model.Conversation{}, nil, err
	}
	if err := s.store.LinkAttachments(ctx, message.ID, attachmentIDs); err != nil {
		return model.Conversation{}, nil, err
	}

	s.publish(conversation, []model.Message{message})

	return conversation, []model.Message{message}, nil
}

func (s *Service) Patch(ctx context.Context, workspaceID, conversationID string, patch Patch) (model.Conversation, error) {
	conversation, err := s.store.ConversationInWorkspace(ctx, workspaceID, conversationID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return model.Conversation{}, ErrConversationNotFound
		}
		return model.Conversation{}, err
	}

	if patch.Status != nil {
		status := flow.Status(strings.TrimSpace(*patch.Status))
		switch status {
		case flow.StatusBot, flow.StatusWaiting, flow.StatusQueued, flow.StatusHuman, flow.StatusClosed:
			conversation.Status = status
		default:
			return model.Conversation{}, ErrInvalidStatus
		}
	}

	if patch.AssignedUserID != nil {
		assignee := strings.TrimSpace(*patch.AssignedUserID)
		if assignee != "" {
			members, err := s.store.Members(ctx, workspaceID)
			if err != nil {
				return model.Conversation{}, err
			}
			found := false
			for _, member := range members {
				if member.UserID == assignee {
					found = true
					break
				}
			}
			if !found {
				return model.Conversation{}, ErrInvalidAssignee
			}
		}
		conversation.AssignedUserID = assignee
	}

	if patch.Tags != nil {
		conversation.Tags = sanitizeTags(*patch.Tags)
	}

	state, err := s.store.State(ctx, conversationID)
	if err != nil {
		return model.Conversation{}, err
	}

	if conversation.Status == flow.StatusHuman || conversation.Status == flow.StatusClosed {
		state.Status = conversation.Status
		state.Waiting = nil
		state.PendingNodeID = ""
		state.WakeAt = nil
		conversation.Waiting = nil
	}

	conversation.UpdatedAt = time.Now().UTC()

	if err := s.store.UpdateConversation(ctx, conversation, state); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return model.Conversation{}, ErrConversationNotFound
		}
		return model.Conversation{}, err
	}

	s.hub.Publish(InboxTopic(workspaceID), "conversation", conversation)
	s.hub.Publish(ConversationTopic(conversationID), "conversation", conversation)

	return conversation, nil
}

func (s *Service) Conversations(ctx context.Context, workspaceID string, filter store.ConversationFilter) ([]model.Conversation, error) {
	return s.store.Conversations(ctx, workspaceID, filter)
}

func (s *Service) Conversation(ctx context.Context, workspaceID, conversationID string) (model.Conversation, error) {
	conversation, err := s.store.ConversationInWorkspace(ctx, workspaceID, conversationID)
	if errors.Is(err, store.ErrNotFound) {
		return model.Conversation{}, ErrConversationNotFound
	}
	return conversation, err
}

func (s *Service) WidgetConversation(ctx context.Context, conversationID string) (model.Conversation, error) {
	conversation, err := s.store.Conversation(ctx, conversationID)
	if errors.Is(err, store.ErrNotFound) {
		return model.Conversation{}, ErrConversationNotFound
	}
	return conversation, err
}

func (s *Service) Messages(ctx context.Context, workspaceID, conversationID string) ([]model.Message, error) {
	if _, err := s.store.ConversationInWorkspace(ctx, workspaceID, conversationID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrConversationNotFound
		}
		return nil, err
	}
	return s.store.Messages(ctx, conversationID)
}

func (s *Service) WidgetMessages(ctx context.Context, conversationID string) ([]model.Message, error) {
	if _, err := s.store.Conversation(ctx, conversationID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrConversationNotFound
		}
		return nil, err
	}
	// Notas internas ficam com a equipe: o visitante nunca as recebe.
	return s.store.MessagesForVisitor(ctx, conversationID)
}

func (s *Service) Stats(ctx context.Context, workspaceID string) (model.Stats, error) {
	return s.store.Stats(ctx, workspaceID)
}

func (s *Service) publish(conversation model.Conversation, messages []model.Message) {
	for _, message := range messages {
		s.hub.Publish(ConversationTopic(conversation.ID), "chat", message)
	}
	s.hub.Publish(InboxTopic(conversation.WorkspaceID), "conversation", conversation)
}

// truncateRunes limita um texto a N caracteres sem jamais cortar no meio de
// um rune multi-byte.
func truncateRunes(value string, limit int) string {
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}

func sanitizeTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))

	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		// Corte por rune, não por byte: tag[:32] cortaria no meio de um
		// acento e gravaria UTF-8 inválido no banco.
		tag = truncateRunes(tag, 32)
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
		if len(out) >= 12 {
			break
		}
	}

	return out
}

// visitorText is what the inbox shows for a submission. A form answer becomes
// a compact line so the agent reads it without opening the state.
func visitorText(input flow.VisitorInput) string {
	text := strings.TrimSpace(input.Text)
	if len(input.Fields) == 0 {
		return text
	}

	parts := make([]string, 0, len(input.Fields))
	for _, name := range sortedFieldNames(input.Fields) {
		parts = append(parts, fmt.Sprintf("%s: %s", name, strings.TrimSpace(input.Fields[name])))
	}

	answer := strings.Join(parts, " · ")
	if text == "" {
		return answer
	}
	return text + " · " + answer
}

func sortedFieldNames(fields map[string]string) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (s *Service) validVisitorID(value string) bool {
	if utf8.RuneCountInString(value) > s.limits.MaxVisitorIDRunes {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	return true
}

func (s *Service) applyEffects(ctx context.Context, conversation *model.Conversation, state *flow.State, effects []flow.Effect) ([]model.Message, error) {
	messages := make([]model.Message, 0, len(effects))
	now := time.Now().UTC()

	for _, effect := range effects {
		switch effect.Type {
		case "send_message":
			if strings.TrimSpace(effect.Text) == "" {
				continue
			}
			messages = append(messages, s.botMessage(conversation.ID, effect.Text, now))
		case "handoff":
			messages = append(messages, model.Message{
				ID:             id.New("msg"),
				ConversationID: conversation.ID,
				Role:           "system",
				Text:           "Encaminhando você para um atendente.",
				CreatedAt:      now,
			})
		case "tag":
			conversation.Tags = sanitizeTags(append(conversation.Tags, effect.Tags...))
		case "assign":
			s.assignAgent(ctx, conversation, effect.AgentID)
		case "call_webhook":
			s.callWebhook(ctx, *conversation, *state, effect.URL)
		case "typing":
			// Estado de interface, não mensagem: some no SSE quando o bot
			// (ou o atendente) responde.
			s.hub.Publish(ConversationTopic(conversation.ID), "typing", map[string]any{"ms": effect.MS})
		case "request_llm":
			text := s.completeLLM(ctx, effect, *state)
			if effect.Variable != "" {
				if state.Variables == nil {
					state.Variables = map[string]any{}
				}
				state.Variables[effect.Variable] = text
			}
			messages = append(messages, s.botMessage(conversation.ID, text, now))
		}
		now = now.Add(time.Microsecond)
	}

	return messages, nil
}

func (s *Service) botMessage(conversationID, text string, createdAt time.Time) model.Message {
	return model.Message{
		ID:             id.New("msg"),
		ConversationID: conversationID,
		Role:           "bot",
		Text:           text,
		CreatedAt:      createdAt,
	}
}

func (s *Service) assignAgent(ctx context.Context, conversation *model.Conversation, agentID string) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return
	}

	members, err := s.store.Members(ctx, conversation.WorkspaceID)
	if err != nil {
		log.Printf("chat: não foi possível validar o atendente %s: %v", agentID, err)
		return
	}

	for _, member := range members {
		if member.UserID == agentID {
			conversation.AssignedUserID = agentID
			return
		}
	}

	log.Printf("chat: atendente %s não pertence ao workspace %s", agentID, conversation.WorkspaceID)
}

func (s *Service) completeLLM(ctx context.Context, effect flow.Effect, state flow.State) string {
	if s.llm != nil {
		text, err := s.llm.Complete(ctx, effect.Prompt, state.Variables)
		if err == nil && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
		if err != nil {
			log.Printf("chat: falha ao consultar IA: %v", err)
		}
	}

	if fallback := strings.TrimSpace(effect.Text); fallback != "" {
		return fallback
	}
	return "Não consegui gerar uma resposta automática agora. Um atendente vai continuar daqui."
}

func (s *Service) callWebhook(ctx context.Context, conversation model.Conversation, state flow.State, rawURL string) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		log.Printf("chat: webhook inválido para a conversa %s", conversation.ID)
		return
	}

	payload, err := json.Marshal(map[string]any{
		"event":          "flow.webhook",
		"conversationId": conversation.ID,
		"workspaceId":    conversation.WorkspaceID,
		"channelId":      conversation.ChannelID,
		"visitorId":      conversation.VisitorID,
		"status":         conversation.Status,
		"currentNodeId":  state.CurrentNodeID,
		"variables":      state.Variables,
	})
	if err != nil {
		log.Printf("chat: falha ao montar payload do webhook: %v", err)
		return
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, parsed.String(), bytes.NewReader(payload))
	if err != nil {
		log.Printf("chat: falha ao criar requisição do webhook: %v", err)
		return
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "ChatFree-Webhook/1.0")

	response, err := s.client.Do(request)
	if err != nil {
		log.Printf("chat: webhook %s falhou: %v", parsed.Host, err)
		return
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		log.Printf("chat: webhook %s respondeu %d", parsed.Host, response.StatusCode)
	}
}

func (s *Service) StartDelayWorker(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			conversations, err := s.store.DueConversations(ctx, 50)
			if err != nil {
				log.Printf("chat: falha ao buscar atrasos pendentes: %v", err)
				continue
			}

			for _, conversation := range conversations {
				if err := s.resumeDelay(ctx, conversation.ID); err != nil {
					log.Printf("chat: falha ao retomar atraso da conversa %s: %v", conversation.ID, err)
				}
			}
		}
	}
}

func (s *Service) resumeDelay(ctx context.Context, conversationID string) error {
	conversation, err := s.store.Conversation(ctx, conversationID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return err
	}

	state, err := s.store.State(ctx, conversationID)
	if err != nil {
		return err
	}
	if state.PendingNodeID == "" {
		return nil
	}

	graph, err := s.store.FlowSnapshot(ctx, conversationID)
	if err != nil {
		return err
	}

	nextState, effects, err := flow.Continue(graph, state)
	if err != nil {
		return err
	}

	conversation.Status = nextState.Status
	conversation.CurrentNodeID = nextState.CurrentNodeID
	conversation.FlowVersion = nextState.FlowVersion
	conversation.Waiting = nextState.Waiting
	conversation.UpdatedAt = time.Now().UTC()

	messages, err := s.applyEffects(ctx, &conversation, &nextState, effects)
	if err != nil {
		return err
	}

	if err := s.store.AppendMessages(ctx, messages...); err != nil {
		return err
	}
	if err := s.store.UpdateConversation(ctx, conversation, nextState); err != nil {
		return err
	}

	s.publish(conversation, messages)

	return nil
}
