package httpapi

import (
	"errors"
	"io"
	"net/http"

	"chatfree/api/internal/chat"
	"chatfree/api/internal/flow"
	"chatfree/api/internal/model"
	"chatfree/api/internal/store"
)

func (a *API) widgetConfig(w http.ResponseWriter, r *http.Request) {
	channelKey := r.PathValue("key")
	if !a.widget.allowConfig(w, r, channelKey) {
		return
	}

	channel, err := a.store.ChannelByKey(r.Context(), channelKey)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "canal não encontrado")
			return
		}
		writeInternalError(w, err)
		return
	}

	if !originAllowed(channel, r) {
		writeError(w, http.StatusForbidden, "origem não permitida para este canal")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":        channel.Key,
		"name":       channel.Name,
		"appearance": channel.Appearance,
	})
}

func (a *API) startConversation(w http.ResponseWriter, r *http.Request) {
	channelKey := r.PathValue("key")
	if !a.widget.allowStartIP(w, r, channelKey) {
		return
	}

	channel, err := a.store.ChannelByKey(r.Context(), channelKey)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "canal não encontrado")
			return
		}
		writeInternalError(w, err)
		return
	}

	if !originAllowed(channel, r) {
		writeError(w, http.StatusForbidden, "origem não permitida para este canal")
		return
	}

	if !a.widget.allowStartChannel(w, channel.Key) {
		return
	}

	var body struct {
		VisitorID string `json:"visitorId"`
	}

	if err := readWidgetJSON(w, r, &body); err != nil && !errors.Is(err, io.EOF) {
		writeWidgetBodyError(w, err)
		return
	}

	conversation, messages, err := a.chat.StartConversation(r.Context(), channel.Key, body.VisitorID)
	if err != nil {
		a.writeChatError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"conversation": conversation,
		"messages":     messages,
	})
}

func (a *API) widgetConversation(w http.ResponseWriter, r *http.Request) {
	if !a.widget.allowRead(w, r, r.PathValue("id")) {
		return
	}

	conversation, _, ok := a.widgetContext(w, r)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, conversation)
}

func (a *API) widgetMessages(w http.ResponseWriter, r *http.Request) {
	if !a.widget.allowRead(w, r, r.PathValue("id")) {
		return
	}

	_, _, ok := a.widgetContext(w, r)
	if !ok {
		return
	}

	messages, err := a.chat.WidgetMessages(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeChatError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, messages)
}

func (a *API) visitorMessage(w http.ResponseWriter, r *http.Request) {
	if !a.widget.allowMessage(w, r, r.PathValue("id")) {
		return
	}

	if _, _, ok := a.widgetContext(w, r); !ok {
		return
	}

	var body flow.VisitorInput
	if err := readWidgetJSON(w, r, &body); err != nil {
		writeWidgetBodyError(w, err)
		return
	}

	conversation, messages, err := a.chat.VisitorMessage(r.Context(), r.PathValue("id"), body)
	if err != nil {
		a.writeChatError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"conversation": conversation,
		"messages":     messages,
	})
}

func (a *API) widgetConversationEvents(w http.ResponseWriter, r *http.Request) {
	if !a.widget.allowRead(w, r, r.PathValue("id")) {
		return
	}

	if _, _, ok := a.widgetContext(w, r); !ok {
		return
	}

	a.stream(w, r, chat.ConversationTopic(r.PathValue("id")))
}

func (a *API) widgetContext(w http.ResponseWriter, r *http.Request) (model.Conversation, model.Channel, bool) {
	conversation, err := a.chat.WidgetConversation(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeChatError(w, err)
		return model.Conversation{}, model.Channel{}, false
	}

	channel, err := a.store.Channel(r.Context(), conversation.WorkspaceID, conversation.ChannelID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "canal não encontrado")
			return model.Conversation{}, model.Channel{}, false
		}
		writeInternalError(w, err)
		return model.Conversation{}, model.Channel{}, false
	}

	if !originAllowed(channel, r) {
		writeError(w, http.StatusForbidden, "origem não permitida para este canal")
		return model.Conversation{}, model.Channel{}, false
	}

	return conversation, channel, true
}
