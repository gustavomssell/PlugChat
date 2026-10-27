package httpapi

import (
	"errors"
	"net/http"

	"chatfree/api/internal/chat"
)

func (a *API) listReplies(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)

	replies, err := a.chat.SavedReplies(r.Context(), claims.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, replies)
}

func (a *API) createReply(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)
	if !canManage(claims) {
		writeError(w, http.StatusForbidden, "somente administradores podem gerenciar respostas prontas")
		return
	}

	body, ok := replyBody(w, r)
	if !ok {
		return
	}

	reply, err := a.chat.CreateSavedReply(r.Context(), claims.WorkspaceID, body)
	if err != nil {
		a.writeChatError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, reply)
}

func (a *API) updateReply(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)
	if !canManage(claims) {
		writeError(w, http.StatusForbidden, "somente administradores podem gerenciar respostas prontas")
		return
	}

	body, ok := replyBody(w, r)
	if !ok {
		return
	}

	reply, err := a.chat.UpdateSavedReply(r.Context(), claims.WorkspaceID, r.PathValue("id"), body)
	if err != nil {
		if errors.Is(err, chat.ErrReplyNotFound) {
			writeError(w, http.StatusNotFound, "resposta pronta não encontrada")
			return
		}
		a.writeChatError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, reply)
}

func (a *API) deleteReply(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)
	if !canManage(claims) {
		writeError(w, http.StatusForbidden, "somente administradores podem gerenciar respostas prontas")
		return
	}

	if err := a.chat.DeleteSavedReply(r.Context(), claims.WorkspaceID, r.PathValue("id")); err != nil {
		if errors.Is(err, chat.ErrReplyNotFound) {
			writeError(w, http.StatusNotFound, "resposta pronta não encontrada")
			return
		}
		a.writeChatError(w, err)
		return
	}

	writeJSON(w, http.StatusNoContent, nil)
}

func replyBody(w http.ResponseWriter, r *http.Request) (chat.SavedReplyInput, bool) {
	var body struct {
		Title    string   `json:"title"`
		Body     string   `json:"body"`
		Shortcut string   `json:"shortcut"`
		Tags     []string `json:"tags"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return chat.SavedReplyInput{}, false
	}

	return chat.SavedReplyInput{
		Title:    body.Title,
		Body:     body.Body,
		Shortcut: body.Shortcut,
		Tags:     body.Tags,
	}, true
}
