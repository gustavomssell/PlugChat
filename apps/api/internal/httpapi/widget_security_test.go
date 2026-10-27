package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"chatfree/api/internal/accounts"
	"chatfree/api/internal/auth"
	"chatfree/api/internal/chat"
	"chatfree/api/internal/db"
	"chatfree/api/internal/flow"
	"chatfree/api/internal/httpapi"
	"chatfree/api/internal/model"
	"chatfree/api/internal/realtime"
	"chatfree/api/internal/store"
)

// strictPolicy keeps limits small so the tests can trip them without waiting.
func strictPolicy() httpapi.WidgetPolicy {
	return httpapi.WidgetPolicy{
		// 3 tokens: o primeiro subteste já consome um pela rota de config.
		ConfigPerMinute: 3,
		// 2 tokens: a tentativa com origem bloqueada consome um (a proteção
		// roda antes da validação de origem, para poupar o banco), sobra um
		// para o início legítimo e o terceiro toma 429.
		StartsPerTenMinutesPerIP:         2,
		StartsPerHourPerChannel:          100,
		ReadsPerMinutePerConversation:    2,
		MessagesPerMinutePerIP:           3,
		MessagesPerMinutePerConversation: 5,
	}
}

type bundle struct {
	Conversation model.Conversation `json:"conversation"`
	Messages     []model.Message    `json:"messages"`
}

func TestWidgetAbuseProtection(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL não definido")
	}

	ctx := context.Background()
	pool, err := db.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	st := store.NewPostgres(pool)
	hub := realtime.NewHub()
	tokens := auth.NewManager("test-secret", time.Hour)
	chatService := chat.New(st, hub)
	accountsService := accounts.New(st, tokens)

	server := httptest.NewServer(httpapi.New(st, chatService, accountsService, tokens, hub, httpapi.WithWidgetPolicy(strictPolicy())))
	defer server.Close()

	suffix := time.Now().UnixNano()
	var registered model.AuthResponse
	requestJSON(t, server.URL, http.MethodPost, "/api/auth/register", "", map[string]any{
		"name":          "Proteção Widget",
		"email":         fmt.Sprintf("widget_guard_%d@test.dev", suffix),
		"password":      "senha12345",
		"workspaceName": "Workspace Guard",
	}, http.StatusCreated, &registered)

	workspaceID := registered.Workspace.ID
	userID := registered.User.ID
	defer func() {
		_, _ = pool.Exec(ctx, `delete from workspaces where id = $1`, workspaceID)
		_, _ = pool.Exec(ctx, `delete from users where id = $1`, userID)
	}()

	graph := flow.Sample()
	graph.ID = fmt.Sprintf("flow_guard_%d", suffix)
	graph.Name = "Fluxo guard"
	if err := st.CreateFlow(ctx, workspaceID, graph); err != nil {
		t.Fatalf("create flow: %v", err)
	}

	channelKey := fmt.Sprintf("pk_guard_%d", suffix)
	channel := model.Channel{
		ID:             fmt.Sprintf("ch_guard_%d", suffix),
		WorkspaceID:    workspaceID,
		Key:            channelKey,
		Name:           "Canal guard",
		FlowID:         graph.ID,
		Appearance:     model.DefaultAppearance,
		AllowedOrigins: []string{"https://allowed.test"},
		CreatedAt:      time.Now().UTC(),
	}
	if err := st.CreateChannel(ctx, channel); err != nil {
		t.Fatalf("create channel: %v", err)
	}

	const origin = "https://allowed.test"

	t.Run("origem ausente é rejeitada quando há allowlist", func(t *testing.T) {
		var denied map[string]string
		requestJSONWithOrigin(t, server.URL, http.MethodGet, "/api/widget/config/"+channelKey, "", http.StatusForbidden, &denied)
	})

	t.Run("respeita Retry-After ao estourar o limite", func(t *testing.T) {
		status, payload := widgetRequest(t, server.URL, http.MethodGet, "/api/widget/config/"+channelKey, origin, nil)
		if status != http.StatusOK {
			t.Fatalf("config 1 status = %d (%s)", status, payload)
		}
		if !strings.Contains(payload, "appearance") {
			t.Fatalf("config payload = %s", payload)
		}

		status, payload = widgetRequest(t, server.URL, http.MethodGet, "/api/widget/config/"+channelKey, origin, nil)
		if status != http.StatusOK {
			t.Fatalf("config 2 status = %d (%s)", status, payload)
		}

		status, payload, headers := widgetRequestWithHeaders(t, server.URL, http.MethodGet, "/api/widget/config/"+channelKey, origin, nil)
		if status != http.StatusTooManyRequests {
			t.Fatalf("config 3 status = %d, want 429 (%s)", status, payload)
		}
		if !strings.Contains(payload, "muitas requisições") {
			t.Fatalf("payload 429 = %s", payload)
		}
		if headers.Get("Retry-After") == "" {
			t.Fatal("Retry-After ausente na resposta 429")
		}
		if got := headers.Get("X-Content-Type-Options"); got != "nosniff" {
			t.Fatalf("X-Content-Type-Options = %q", got)
		}
	})

	t.Run("início rejeita origem bloqueada e limita por IP", func(t *testing.T) {
		status, payload := widgetRequest(t, server.URL, http.MethodPost, "/api/widget/start/"+channelKey, "https://blocked.test", map[string]string{})
		if status != http.StatusForbidden {
			t.Fatalf("start bloqueado status = %d, want 403 (%s)", status, payload)
		}

		status, payload = widgetRequest(t, server.URL, http.MethodPost, "/api/widget/start/"+channelKey, origin, map[string]string{"visitorId": "visitor-ok"})
		if status != http.StatusCreated {
			t.Fatalf("start status = %d, want 201 (%s)", status, payload)
		}

		status, payload = widgetRequest(t, server.URL, http.MethodPost, "/api/widget/start/"+channelKey, origin, map[string]string{})
		if status != http.StatusTooManyRequests {
			t.Fatalf("segundo start status = %d, want 429 (%s)", status, payload)
		}
	})

	t.Run("limita corpo, tamanho de mensagem e visitorId", func(t *testing.T) {
		conversations, err := st.Conversations(ctx, workspaceID, store.ConversationFilter{})
		if err != nil {
			t.Fatalf("conversations: %v", err)
		}
		if len(conversations) == 0 {
			t.Fatal("nenhuma conversa criada")
		}
		conversationID := conversations[0].ID
		messagesPath := "/api/widget/conversations/" + conversationID + "/messages"

		status, payload := widgetRequest(t, server.URL, http.MethodPost, messagesPath, origin, map[string]string{"text": strings.Repeat("a", 4001)})
		if status != http.StatusBadRequest || !strings.Contains(payload, "muito longa") {
			t.Fatalf("mensagem longa status = %d (%s)", status, payload)
		}

		status, payload = widgetRequest(t, server.URL, http.MethodPost, messagesPath, origin, map[string]string{"text": "segunda mensagem"})
		if status != http.StatusOK {
			t.Fatalf("mensagem válida status = %d, want 200 (%s)", status, payload)
		}

		status, payload = widgetRequest(t, server.URL, http.MethodGet, "/api/widget/conversations/"+conversationID, origin, nil)
		if status != http.StatusOK {
			t.Fatalf("leitura 1 status = %d, want 200 (%s)", status, payload)
		}

		status, payload = widgetRequest(t, server.URL, http.MethodGet, "/api/widget/conversations/"+conversationID+"/messages", origin, nil)
		if status != http.StatusOK {
			t.Fatalf("leitura 2 status = %d, want 200 (%s)", status, payload)
		}

		// O limite de leitura é 2 por minuto nesta conversa.
		status, payload = widgetRequest(t, server.URL, http.MethodGet, "/api/widget/conversations/"+conversationID, origin, nil)
		if status != http.StatusTooManyRequests {
			t.Fatalf("leitura 3 status = %d, want 429 (%s)", status, payload)
		}

		// Corpo acima do limite de 16 KB.
		status, payload = widgetRequest(t, server.URL, http.MethodPost, messagesPath, origin, map[string]string{"text": strings.Repeat("b", 17*1024)})
		if status != http.StatusRequestEntityTooLarge {
			t.Fatalf("corpo grande status = %d, want 413 (%s)", status, payload)
		}

		// Limite de mensagens por IP (3/min) esgotado.
		status, payload = widgetRequest(t, server.URL, http.MethodPost, messagesPath, origin, map[string]string{"text": "terceira mensagem"})
		if status != http.StatusTooManyRequests {
			t.Fatalf("mensagem extra status = %d, want 429 (%s)", status, payload)
		}
	})

	t.Run("visitorId inválido é rejeitado", func(t *testing.T) {
		permissive := httptest.NewServer(httpapi.New(st, chatService, accountsService, tokens, hub, httpapi.WithWidgetPolicy(httpapi.WidgetPolicy{
			StartsPerTenMinutesPerIP: 100,
			ConfigPerMinute:          100,
			StartsPerHourPerChannel:  1000,
		})))
		defer permissive.Close()

		status, payload := widgetRequest(t, permissive.URL, http.MethodPost, "/api/widget/start/"+channelKey, origin, map[string]string{"visitorId": strings.Repeat("z", 129)})
		if status != http.StatusBadRequest || !strings.Contains(payload, "visitante") {
			t.Fatalf("visitorId longo status = %d (%s)", status, payload)
		}

		status, payload = widgetRequest(t, permissive.URL, http.MethodPost, "/api/widget/start/"+channelKey, origin, map[string]string{"visitorId": "com\nquebra"})
		if status != http.StatusBadRequest || !strings.Contains(payload, "visitante") {
			t.Fatalf("visitorId com controle status = %d (%s)", status, payload)
		}
	})

	t.Run("canais validam origens ao salvar", func(t *testing.T) {
		status, payload := requestJSONStatus(t, server.URL, http.MethodPost, "/api/channels", registered.Token, map[string]any{
			"name":           "Canal origem inválida",
			"allowedOrigins": []string{"https://ok.test", "https://ruim.test/pagina"},
		})
		if status != http.StatusBadRequest || !strings.Contains(payload, "origem") {
			t.Fatalf("canal origem inválida status = %d (%s)", status, payload)
		}

		var created model.Channel
		requestJSON(t, server.URL, http.MethodPost, "/api/channels", registered.Token, map[string]any{
			"name":           "Canal origem válida",
			"allowedOrigins": []string{"exemplo.com/"},
		}, http.StatusCreated, &created)
		if len(created.AllowedOrigins) != 1 || created.AllowedOrigins[0] != "https://exemplo.com" {
			t.Fatalf("origens normalizadas = %+v", created.AllowedOrigins)
		}
	})
}

func TestWidgetFormAndTyping(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL não definido")
	}

	ctx := context.Background()
	pool, err := db.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	st := store.NewPostgres(pool)
	hub := realtime.NewHub()
	tokens := auth.NewManager("test-secret", time.Hour)
	chatService := chat.New(st, hub)
	accountsService := accounts.New(st, tokens)

	server := httptest.NewServer(httpapi.New(st, chatService, accountsService, tokens, hub))
	defer server.Close()

	suffix := time.Now().UnixNano()
	var registered model.AuthResponse
	requestJSON(t, server.URL, http.MethodPost, "/api/auth/register", "", map[string]any{
		"name":          "Formulário",
		"email":         fmt.Sprintf("form_%d@test.dev", suffix),
		"password":      "senha12345",
		"workspaceName": "Workspace Form",
	}, http.StatusCreated, &registered)

	workspaceID := registered.Workspace.ID
	defer func() {
		_, _ = pool.Exec(ctx, `delete from workspaces where id = $1`, workspaceID)
		_, _ = pool.Exec(ctx, `delete from users where id = $1`, registered.User.ID)
	}()

	graph := flow.Sample()
	graph.ID = fmt.Sprintf("flow_sample_%d", suffix)
	if err := st.CreateFlow(ctx, workspaceID, graph); err != nil {
		t.Fatalf("create flow: %v", err)
	}

	channelKey := fmt.Sprintf("pk_form_%d", suffix)
	if err := st.CreateChannel(ctx, model.Channel{
		ID:             fmt.Sprintf("ch_form_%d", suffix),
		WorkspaceID:    workspaceID,
		Key:            channelKey,
		Name:           "Canal formulário",
		FlowID:         graph.ID,
		Appearance:     model.DefaultAppearance,
		AllowedOrigins: []string{}, // lista vazia aceita qualquer origem
		CreatedAt:      time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create channel: %v", err)
	}

	const origin = "https://form.test"

	send := func(t *testing.T, conversationID string, body map[string]any) (int, bundle) {
		t.Helper()
		status, payload := widgetRequest(t, server.URL, http.MethodPost, "/api/widget/conversations/"+conversationID+"/messages", origin, body)
		var result bundle
		if status == http.StatusOK {
			if err := json.Unmarshal([]byte(payload), &result); err != nil {
				t.Fatalf("decode %s: %v", payload, err)
			}
		}
		return status, result
	}

	start := func(t *testing.T) string {
		t.Helper()
		status, payload := widgetRequest(t, server.URL, http.MethodPost, "/api/widget/start/"+channelKey, origin, map[string]string{})
		if status != http.StatusCreated {
			t.Fatalf("start status = %d (%s)", status, payload)
		}
		var result bundle
		if err := json.Unmarshal([]byte(payload), &result); err != nil {
			t.Fatalf("decode start: %v", err)
		}
		return result.Conversation.ID
	}

	t.Run("formulário coleta vários campos em uma interação", func(t *testing.T) {
		conversationID := start(t)

		status, named := send(t, conversationID, map[string]any{"text": "Maria"})
		if status != http.StatusOK {
			t.Fatalf("nome status = %d", status)
		}
		if named.Conversation.Waiting == nil || len(named.Conversation.Waiting.Options) != 3 {
			t.Fatalf("waiting = %+v", named.Conversation.Waiting)
		}

		// "Comercial" segue o caminho longo: condição -> formulário.
		status, asked := send(t, conversationID, map[string]any{"text": "Comercial"})
		if status != http.StatusOK {
			t.Fatalf("assunto status = %d", status)
		}
		if asked.Conversation.Waiting == nil || asked.Conversation.Waiting.InputType != "form" {
			t.Fatalf("waiting = %+v", asked.Conversation.Waiting)
		}
		if len(asked.Conversation.Waiting.Fields) != 2 {
			t.Fatalf("campos = %+v", asked.Conversation.Waiting.Fields)
		}

		// Campo obrigatório vazio -> 400 informando o campo.
		status, _ = send(t, conversationID, map[string]any{"fields": map[string]string{"phone": "11999999999"}})
		if status != http.StatusBadRequest {
			t.Fatalf("campo obrigatório status = %d, want 400", status)
		}

		status, filled := send(t, conversationID, map[string]any{
			"fields": map[string]string{"email": "ana@example.com", "phone": "11999999999"},
		})
		if status != http.StatusOK {
			t.Fatalf("formulário status = %d", status)
		}
		if filled.Conversation.Status != "closed" {
			t.Fatalf("status = %q, want closed", filled.Conversation.Status)
		}

		// A resposta do POST traz só as mensagens do bot; o histórico completo
		// inclui a do visitante, que precisa mostrar os dados preenchidos.
		status, payload := widgetRequest(t, server.URL, http.MethodGet, "/api/widget/conversations/"+conversationID+"/messages", origin, nil)
		if status != http.StatusOK {
			t.Fatalf("histórico status = %d (%s)", status, payload)
		}
		var history []model.Message
		if err := json.Unmarshal([]byte(payload), &history); err != nil {
			t.Fatalf("decode histórico: %v", err)
		}

		var visitorText string
		for _, message := range history {
			if message.Role == "visitor" && strings.Contains(message.Text, "email:") {
				visitorText = message.Text
			}
		}
		if !strings.Contains(visitorText, "ana@example.com") || !strings.Contains(visitorText, "phone") {
			t.Fatalf("mensagem do visitante = %q", visitorText)
		}
	})

	t.Run("opção com atalho pula os passos intermediários", func(t *testing.T) {
		conversationID := start(t)

		status, named := send(t, conversationID, map[string]any{"text": "Maria"})
		if status != http.StatusOK {
			t.Fatalf("nome status = %d", status)
		}
		if named.Conversation.Waiting == nil || named.Conversation.Waiting.Options[1].Next != "route_support" {
			t.Fatalf("atalho não exposto na API: %+v", named.Conversation.Waiting.Options)
		}

		status, routed := send(t, conversationID, map[string]any{"text": "Suporte"})
		if status != http.StatusOK {
			t.Fatalf("atalho status = %d", status)
		}
		if routed.Conversation.Status != "queued" {
			t.Fatalf("status após atalho = %q, want queued", routed.Conversation.Status)
		}
	})
}

func widgetRequest(t *testing.T, base, method, path, origin string, body any) (int, string) {
	status, payload, _ := widgetRequestWithHeaders(t, base, method, path, origin, body)
	return status, payload
}

func widgetRequestWithHeaders(t *testing.T, base, method, path, origin string, body any) (int, string, http.Header) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(payload)
	}

	request, err := http.NewRequest(method, base+path, reader)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if origin != "" {
		request.Header.Set("Origin", origin)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer response.Body.Close()

	payload, _ := io.ReadAll(response.Body)
	return response.StatusCode, string(payload), response.Header
}

func requestJSONStatus(t *testing.T, base, method, path, token string, body any) (int, string) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(payload)
	}

	request, err := http.NewRequest(method, base+path, reader)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer response.Body.Close()

	payload, _ := io.ReadAll(response.Body)
	return response.StatusCode, string(payload)
}
