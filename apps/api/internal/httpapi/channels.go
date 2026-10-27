package httpapi

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"chatfree/api/internal/id"
	"chatfree/api/internal/model"
	"chatfree/api/internal/store"
)

const maxChannelOrigins = 20

type channelBody struct {
	Name           string           `json:"name"`
	FlowID         string           `json:"flowId"`
	Appearance     model.Appearance `json:"appearance"`
	AllowedOrigins []string         `json:"allowedOrigins"`
}

func (a *API) listChannels(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)

	channels, err := a.store.Channels(r.Context(), claims.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, channels)
}

func (a *API) getChannel(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)

	channel, err := a.store.Channel(r.Context(), claims.WorkspaceID, r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "canal não encontrado")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, channel)
}

func (a *API) createChannel(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)
	if !canManage(claims) {
		writeError(w, http.StatusForbidden, "somente administradores podem criar canais")
		return
	}

	var body channelBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = "Novo canal"
	}

	flowID := strings.TrimSpace(body.FlowID)
	if flowID == "" {
		graphs, err := a.store.Flows(r.Context(), claims.WorkspaceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if len(graphs) == 0 {
			writeError(w, http.StatusBadRequest, "crie um fluxo antes de criar um canal")
			return
		}
		flowID = graphs[0].ID
	} else if _, err := a.store.Flow(r.Context(), claims.WorkspaceID, flowID); err != nil {
		writeError(w, http.StatusBadRequest, "fluxo inválido")
		return
	}

	appearance, message := normalizeAppearance(body.Appearance)
	if message != "" {
		writeError(w, http.StatusBadRequest, message)
		return
	}

	allowedOrigins, message := sanitizeOrigins(body.AllowedOrigins)
	if message != "" {
		writeError(w, http.StatusBadRequest, message)
		return
	}

	channel := model.Channel{
		ID:             id.New("ch"),
		WorkspaceID:    claims.WorkspaceID,
		Key:            id.New("pk"),
		Name:           name,
		FlowID:         flowID,
		Appearance:     appearance,
		AllowedOrigins: allowedOrigins,
		CreatedAt:      time.Now().UTC(),
	}

	if err := a.store.CreateChannel(r.Context(), channel); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, channel)
}

func (a *API) updateChannel(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)
	if !canManage(claims) {
		writeError(w, http.StatusForbidden, "somente administradores podem editar canais")
		return
	}

	channel, err := a.store.Channel(r.Context(), claims.WorkspaceID, r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "canal não encontrado")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var body channelBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}

	if name := strings.TrimSpace(body.Name); name != "" {
		channel.Name = name
	}

	if flowID := strings.TrimSpace(body.FlowID); flowID != "" && flowID != channel.FlowID {
		if _, err := a.store.Flow(r.Context(), claims.WorkspaceID, flowID); err != nil {
			writeError(w, http.StatusBadRequest, "fluxo inválido")
			return
		}
		channel.FlowID = flowID
	}

	appearance, message := normalizeAppearance(body.Appearance)
	if message != "" {
		writeError(w, http.StatusBadRequest, message)
		return
	}
	allowedOrigins, message := sanitizeOrigins(body.AllowedOrigins)
	if message != "" {
		writeError(w, http.StatusBadRequest, message)
		return
	}
	channel.Appearance = appearance
	channel.AllowedOrigins = allowedOrigins

	if err := a.store.UpdateChannel(r.Context(), channel); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "canal não encontrado")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, channel)
}

func (a *API) deleteChannel(w http.ResponseWriter, r *http.Request) {
	claims := claimsOf(r)
	if !canManage(claims) {
		writeError(w, http.StatusForbidden, "somente administradores podem remover canais")
		return
	}

	if err := a.store.DeleteChannel(r.Context(), claims.WorkspaceID, r.PathValue("id")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "canal não encontrado")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func normalizeAppearance(in model.Appearance) (model.Appearance, string) {
	out := in

	if out.Mode == "" {
		out.Mode = model.DefaultAppearance.Mode
	}
	if out.Position == "" {
		out.Position = model.DefaultAppearance.Position
	}
	if out.Theme == "" {
		out.Theme = model.DefaultAppearance.Theme
	}
	if out.Accent == "" {
		out.Accent = model.DefaultAppearance.Accent
	}
	if strings.TrimSpace(out.Greeting) == "" {
		out.Greeting = model.DefaultAppearance.Greeting
	}
	if strings.TrimSpace(out.LauncherLabel) == "" {
		out.LauncherLabel = model.DefaultAppearance.LauncherLabel
	}

	if !oneOf(out.Mode, "bubble", "inline", "popup", "fullscreen") {
		return out, "modo de exibição inválido"
	}
	if !oneOf(out.Position, "bottom-right", "bottom-left", "top-right", "top-left") {
		return out, "posição inválida"
	}
	if !oneOf(out.Theme, "light", "dark", "auto") {
		return out, "tema inválido"
	}
	if !accentPattern.MatchString(out.Accent) {
		return out, "cor de destaque inválida"
	}

	return out, ""
}

func sanitizeOrigins(origins []string) ([]string, string) {
	out := make([]string, 0, len(origins))
	seen := make(map[string]struct{}, len(origins))

	for _, origin := range origins {
		if strings.TrimSpace(origin) == "" {
			continue
		}
		normalized, ok := normalizeOrigin(origin)
		if !ok {
			return nil, "origem inválida: " + truncate(strings.TrimSpace(origin), 80)
		}
		if normalized == "*" {
			return []string{"*"}, ""
		}

		key := strings.ToLower(normalized)
		if _, exists := seen[key]; exists {
			continue
		}
		if len(out) >= maxChannelOrigins {
			return nil, "um canal pode ter no máximo 20 origens permitidas"
		}
		seen[key] = struct{}{}
		out = append(out, normalized)
	}

	return out, ""
}

// normalizeOrigin accepts "https://site.com", "site.com" (assumed https) and
// "*". It rejects paths, queries, fragments, user info and wildcard hosts so
// the stored value always matches a browser Origin header exactly.
func normalizeOrigin(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "null") {
		return "", false
	}
	if raw == "*" {
		return "*", true
	}

	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return "", false
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", false
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false
	}
	if path := parsed.EscapedPath(); path != "" && path != "/" {
		return "", false
	}

	host := strings.ToLower(parsed.Host)
	if strings.Contains(host, "*") {
		return "", false
	}
	host = stripDefaultPort(scheme, host)

	return scheme + "://" + host, true
}

func stripDefaultPort(scheme, host string) string {
	name, port, err := net.SplitHostPort(host)
	if err != nil {
		return host
	}
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		return name
	}
	return host
}

func originAllowed(channel model.Channel, r *http.Request) bool {
	if len(channel.AllowedOrigins) == 0 {
		return true
	}

	for _, allowed := range channel.AllowedOrigins {
		if strings.TrimSpace(allowed) == "*" {
			return true
		}
	}

	origin, ok := normalizeOrigin(r.Header.Get("Origin"))
	if !ok || origin == "*" {
		return false
	}

	for _, allowed := range channel.AllowedOrigins {
		normalized, ok := normalizeOrigin(allowed)
		if ok && normalized == origin {
			return true
		}
	}

	return false
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}
