package httpapi

import (
	"net/http"

	"chatfree/api/internal/chat"
)

// addNote stores an internal note. Notes travel only through the workspace
// inbox topic: the visitor never receives them, not even by SSE.
func (a *API) addNote(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)

	var body struct {
		Text string `json:"text"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}

	note, err := a.chat.AddNote(r.Context(), claims.WorkspaceID, claims.Subject, r.PathValue("id"), body.Text)
	if err != nil {
		a.writeChatError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, note)
}

func (a *API) bulkConversations(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)

	var body struct {
		IDs            []string `json:"ids"`
		Action         string   `json:"action"`
		AssignedUserID string   `json:"assignedUserId"`
		Tags           []string `json:"tags"`
		Status         string   `json:"status"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}

	updated, err := a.chat.BulkPatch(r.Context(), claims.WorkspaceID, chat.BulkAction{
		IDs:            body.IDs,
		Action:         body.Action,
		AssignedUserID: body.AssignedUserID,
		Tags:           body.Tags,
		Status:         body.Status,
	})
	if err != nil {
		a.writeChatError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"updated": updated})
}
