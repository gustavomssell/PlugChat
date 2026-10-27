package httpapi

import (
	"net/http"
	"strings"

	"chatfree/api/internal/chat"
	"chatfree/api/internal/store"
)

func (a *API) listConversations(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)

	filter := store.ConversationFilter{
		Status:    strings.TrimSpace(r.URL.Query().Get("status")),
		ChannelID: strings.TrimSpace(r.URL.Query().Get("channelId")),
		Search:    strings.TrimSpace(r.URL.Query().Get("search")),
	}

	conversations, err := a.chat.Conversations(r.Context(), claims.WorkspaceID, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, conversations)
}

func (a *API) getConversation(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)

	conversation, err := a.chat.Conversation(r.Context(), claims.WorkspaceID, r.PathValue("id"))
	if err != nil {
		a.writeChatError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, conversation)
}

func (a *API) patchConversation(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)

	var body struct {
		Status         *string   `json:"status"`
		AssignedUserID *string   `json:"assignedUserId"`
		Tags           *[]string `json:"tags"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}

	conversation, err := a.chat.Patch(r.Context(), claims.WorkspaceID, r.PathValue("id"), chat.Patch{
		Status:         body.Status,
		AssignedUserID: body.AssignedUserID,
		Tags:           body.Tags,
	})
	if err != nil {
		a.writeChatError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, conversation)
}

func (a *API) listMessages(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)

	messages, err := a.chat.Messages(r.Context(), claims.WorkspaceID, r.PathValue("id"))
	if err != nil {
		a.writeChatError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, messages)
}

func (a *API) agentReply(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)

	var body struct {
		Text          string   `json:"text"`
		AttachmentIDs []string `json:"attachmentIds"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}

	conversation, messages, err := a.chat.AgentReply(r.Context(), claims.WorkspaceID, claims.Subject, r.PathValue("id"), body.Text, body.AttachmentIDs)
	if err != nil {
		a.writeChatError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"conversation": conversation,
		"messages":     messages,
	})
}

func (a *API) conversationEvents(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)

	conversationID := r.PathValue("id")
	if _, err := a.chat.Conversation(r.Context(), claims.WorkspaceID, conversationID); err != nil {
		a.writeChatError(w, err)
		return
	}

	a.stream(w, r, chat.ConversationTopic(conversationID))
}

func (a *API) inboxEvents(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)
	a.stream(w, r, chat.InboxTopic(claims.WorkspaceID))
}
