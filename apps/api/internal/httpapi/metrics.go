package httpapi

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
	"time"

	"chatfree/api/internal/store"
)

// metrics serves the report of the dashboard reports page. The period defaults
// to 7 days and is capped at 90 to keep every aggregate cheap.
func (a *API) metrics(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)

	days := 7
	if raw := strings.TrimSpace(r.URL.Query().Get("days")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "período inválido")
			return
		}
		days = parsed
	}

	metrics, err := a.chat.Metrics(r.Context(), claims.WorkspaceID, days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, metrics)
}

// exportConversations streams the filtered inbox as CSV. It mirrors the inbox
// filters so an agent can export exactly what is on screen.
func (a *API) exportConversations(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)

	filter := store.ConversationFilter{
		Status:    strings.TrimSpace(r.URL.Query().Get("status")),
		ChannelID: strings.TrimSpace(r.URL.Query().Get("channelId")),
		Search:    strings.TrimSpace(r.URL.Query().Get("search")),
		Limit:     5000,
	}

	conversations, err := a.chat.Conversations(r.Context(), claims.WorkspaceID, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	channels, err := a.store.Channels(r.Context(), claims.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	agents, err := a.store.Members(r.Context(), claims.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	channelNames := make(map[string]string, len(channels))
	for _, channel := range channels {
		channelNames[channel.ID] = channel.Name
	}
	agentNames := make(map[string]string, len(agents))
	for _, agent := range agents {
		agentNames[agent.UserID] = agent.Name
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="conversas-chatfree.csv"`)
	w.WriteHeader(http.StatusOK)

	// BOM helps Excel open UTF-8 CSVs with accents correctly.
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	_ = writer.Write([]string{
		"id", "canal", "visitante", "status", "responsavel", "tags",
		"fluxo", "criada_em", "atualizada_em",
	})

	for _, conversation := range conversations {
		status := string(conversation.Status)
		channel := channelNames[conversation.ChannelID]
		if channel == "" {
			channel = "Canal removido"
		}
		assignee := agentNames[conversation.AssignedUserID]
		if assignee == "" {
			assignee = "não atribuída"
		}

		_ = writer.Write([]string{
			conversation.ID,
			channel,
			conversation.VisitorID,
			status,
			assignee,
			strings.Join(conversation.Tags, " · "),
			conversation.FlowID,
			conversation.CreatedAt.Format(time.RFC3339),
			conversation.UpdatedAt.Format(time.RFC3339),
		})
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return
	}
}
