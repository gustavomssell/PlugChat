package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"chatfree/api/internal/accounts"
	"chatfree/api/internal/auth"
	"chatfree/api/internal/chat"
	"chatfree/api/internal/files"
	"chatfree/api/internal/flow"
	"chatfree/api/internal/realtime"
	"chatfree/api/internal/store"
)

var accentPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type API struct {
	store    store.Store
	chat     *chat.Service
	accounts *accounts.Service
	tokens   *auth.Manager
	hub      *realtime.Hub
	widget   *widgetGuard
	files    *files.Store
}

// Option customizes the HTTP API. It exists mainly so tests and deployments
// can tune the public widget abuse policy.
type Option func(*API)

// WithWidgetPolicy overrides the default rate limits and proxy trust used by
// the public widget routes.
func WithWidgetPolicy(policy WidgetPolicy) Option {
	return func(a *API) {
		a.widget = newWidgetGuard(policy)
	}
}

// WithAttachments enables the upload endpoints. Without it they answer 503,
// so a deployment that does not want to accept files simply omits the store.
func WithAttachments(store *files.Store) Option {
	return func(a *API) {
		a.files = store
	}
}

type contextKey struct{ name string }

var claimsKey = contextKey{"claims"}

func New(st store.Store, chatService *chat.Service, accountsService *accounts.Service, tokens *auth.Manager, hub *realtime.Hub, options ...Option) http.Handler {
	api := &API{
		store:    st,
		chat:     chatService,
		accounts: accountsService,
		tokens:   tokens,
		hub:      hub,
		widget:   newWidgetGuard(DefaultWidgetPolicy()),
	}
	for _, option := range options {
		option(api)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", api.health)

	mux.HandleFunc("POST /api/auth/register", api.register)
	mux.HandleFunc("POST /api/auth/login", api.login)
	mux.HandleFunc("GET /api/auth/me", api.requireAuth(api.me))
	mux.HandleFunc("PATCH /api/auth/me/availability", api.requireAuth(api.setAvailability))

	mux.HandleFunc("GET /api/stats", api.requireAuth(api.stats))
	mux.HandleFunc("GET /api/metrics", api.requireAuth(api.metrics))

	mux.HandleFunc("GET /api/replies", api.requireAuth(api.listReplies))
	mux.HandleFunc("POST /api/replies", api.requireAuth(api.createReply))
	mux.HandleFunc("PUT /api/replies/{id}", api.requireAuth(api.updateReply))
	mux.HandleFunc("DELETE /api/replies/{id}", api.requireAuth(api.deleteReply))

	mux.HandleFunc("GET /api/agents", api.requireAuth(api.listAgents))
	mux.HandleFunc("POST /api/agents", api.requireAuth(api.createAgent))

	mux.HandleFunc("GET /api/channels", api.requireAuth(api.listChannels))
	mux.HandleFunc("POST /api/channels", api.requireAuth(api.createChannel))
	mux.HandleFunc("GET /api/channels/{id}", api.requireAuth(api.getChannel))
	mux.HandleFunc("PUT /api/channels/{id}", api.requireAuth(api.updateChannel))
	mux.HandleFunc("DELETE /api/channels/{id}", api.requireAuth(api.deleteChannel))

	mux.HandleFunc("GET /api/flows", api.requireAuth(api.listFlows))
	mux.HandleFunc("POST /api/flows", api.requireAuth(api.createFlow))
	mux.HandleFunc("GET /api/flows/{id}", api.requireAuth(api.getFlow))
	mux.HandleFunc("PUT /api/flows/{id}", api.requireAuth(api.saveFlow))
	mux.HandleFunc("DELETE /api/flows/{id}", api.requireAuth(api.deleteFlow))

	mux.HandleFunc("GET /api/conversations", api.requireAuth(api.listConversations))
	mux.HandleFunc("GET /api/conversations/export", api.requireAuth(api.exportConversations))
	mux.HandleFunc("POST /api/conversations/bulk", api.requireAuth(api.bulkConversations))
	mux.HandleFunc("GET /api/conversations/{id}", api.requireAuth(api.getConversation))
	mux.HandleFunc("PATCH /api/conversations/{id}", api.requireAuth(api.patchConversation))
	mux.HandleFunc("GET /api/conversations/{id}/messages", api.requireAuth(api.listMessages))
	mux.HandleFunc("POST /api/conversations/{id}/messages", api.requireAuth(api.agentReply))
	mux.HandleFunc("POST /api/conversations/{id}/notes", api.requireAuth(api.addNote))
	mux.HandleFunc("POST /api/conversations/{id}/attachments", api.requireAuth(api.uploadAttachment))
	mux.HandleFunc("GET /api/conversations/{id}/events", api.requireAuth(api.conversationEvents))
	mux.HandleFunc("GET /api/events", api.requireAuth(api.inboxEvents))

	mux.HandleFunc("GET /api/attachments/{id}", api.requireAuth(api.downloadAttachment))

	mux.HandleFunc("GET /api/widget/config/{key}", api.widgetConfig)
	mux.HandleFunc("POST /api/widget/start/{key}", api.startConversation)
	mux.HandleFunc("GET /api/widget/conversations/{id}", api.widgetConversation)
	mux.HandleFunc("GET /api/widget/conversations/{id}/messages", api.widgetMessages)
	mux.HandleFunc("POST /api/widget/conversations/{id}/messages", api.visitorMessage)
	mux.HandleFunc("POST /api/widget/conversations/{id}/attachments", api.widgetUploadAttachment)
	mux.HandleFunc("GET /api/widget/conversations/{id}/events", api.widgetConversationEvents)
	mux.HandleFunc("GET /api/widget/attachments/{id}", api.widgetAttachment)

	return securityHeaders(cors(mux))
}

// securityHeaders applies conservative defaults to every response. The widget
// API belongs to third-party pages, so its payloads must never be cached and
// must not be treated as HTML/script by browsers.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Add("Vary", "Origin")

		if strings.HasPrefix(r.URL.Path, "/api/widget/") {
			w.Header().Set("Cache-Control", "no-store")
		}

		next.ServeHTTP(w, r)
	})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := ""

		if header := r.Header.Get("Authorization"); strings.HasPrefix(header, "Bearer ") {
			token = strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		}
		if token == "" {
			token = r.URL.Query().Get("token")
		}

		if token == "" {
			writeError(w, http.StatusUnauthorized, "autenticação necessária")
			return
		}

		claims, err := a.tokens.Parse(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "sessão inválida ou expirada")
			return
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next(w, r.WithContext(ctx))
	}
}

func claimsOf(r *http.Request) auth.Claims {
	claims, _ := r.Context().Value(claimsKey).(auth.Claims)
	return claims
}

func canManage(claims auth.Claims) bool {
	return claims.Role == "owner" || claims.Role == "admin"
}

func (a *API) stream(w http.ResponseWriter, r *http.Request, topic string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "stream indisponível")
		return
	}

	events, cancel := a.hub.Subscribe(topic)
	defer cancel()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	keepAlive := time.NewTicker(25 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepAlive.C:
			_, _ = fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case event := <-events:
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Name, event.Data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func readJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// fieldOf extracts the field label from a ErrMissingField wrapper, whose text
// looks like "flow: missing required field: E-mail".
func fieldOf(err error) string {
	message := err.Error()
	index := strings.LastIndex(message, ": ")
	if index < 0 || index+2 >= len(message) {
		return "obrigatório"
	}
	return strings.TrimSpace(message[index+2:])
}

func (a *API) writeChatError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, chat.ErrChannelNotFound):
		writeError(w, http.StatusNotFound, "canal não encontrado")
	case errors.Is(err, chat.ErrConversationNotFound):
		writeError(w, http.StatusNotFound, "conversa não encontrada")
	case errors.Is(err, chat.ErrEmptyMessage):
		writeError(w, http.StatusBadRequest, "mensagem vazia")
	case errors.Is(err, flow.ErrMissingField):
		writeError(w, http.StatusBadRequest, "preencha o campo: "+fieldOf(err))
	case errors.Is(err, flow.ErrTooManySteps):
		writeError(w, http.StatusBadRequest, "a conversa excedeu o número de passos do fluxo e foi encerrada")
	case errors.Is(err, chat.ErrMessageTooLong):
		writeError(w, http.StatusBadRequest, "mensagem muito longa")
	case errors.Is(err, chat.ErrInvalidVisitorID):
		writeError(w, http.StatusBadRequest, "identificador de visitante inválido")
	case errors.Is(err, chat.ErrInvalidStatus):
		writeError(w, http.StatusBadRequest, "status inválido")
	case errors.Is(err, chat.ErrInvalidAssignee):
		writeError(w, http.StatusBadRequest, "responsável inválido")
	case errors.Is(err, chat.ErrInvalidNote):
		writeError(w, http.StatusBadRequest, "escreva uma nota de até 4000 caracteres")
	case errors.Is(err, chat.ErrBulkTooManyIDs):
		writeError(w, http.StatusBadRequest, "selecione até 100 conversas")
	case errors.Is(err, chat.ErrInvalidBulkAction):
		writeError(w, http.StatusBadRequest, "ação inválida")
	case errors.Is(err, chat.ErrInvalidReply):
		writeError(w, http.StatusBadRequest, "informe título e corpo da resposta")
	case errors.Is(err, chat.ErrInvalidShortcut):
		writeError(w, http.StatusBadRequest, "atalho inválido ou já em uso")
	case errors.Is(err, chat.ErrTooManyReplies):
		writeError(w, http.StatusBadRequest, "limite de respostas prontas atingido")
	case errors.Is(err, chat.ErrReplyNotFound):
		writeError(w, http.StatusNotFound, "resposta pronta não encontrada")
	case errors.Is(err, chat.ErrAttachmentNotFound):
		writeError(w, http.StatusBadRequest, "anexo não encontrado")
	case errors.Is(err, chat.ErrAttachmentTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "anexo maior que o limite de 8 MB")
	case errors.Is(err, chat.ErrInvalidAttachmentKind):
		writeError(w, http.StatusUnsupportedMediaType, "tipo de arquivo não permitido")
	case errors.Is(err, chat.ErrTooManyAttachments):
		writeError(w, http.StatusBadRequest, "até 5 anexos por mensagem")
	case errors.Is(err, chat.ErrAttachmentNotInConversation):
		writeError(w, http.StatusBadRequest, "anexo não pertence a esta conversa")
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "registro não encontrado")
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
