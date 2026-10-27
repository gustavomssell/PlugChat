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

func TestFullJourney(t *testing.T) {
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
	email := fmt.Sprintf("journey_%d@test.dev", suffix)

	var registered model.AuthResponse
	requestJSON(t, server.URL, http.MethodPost, "/api/auth/register", "", map[string]any{
		"name":          "Joana Teste",
		"email":         email,
		"password":      "senha12345",
		"workspaceName": "Workspace Teste",
	}, http.StatusCreated, &registered)

	workspaceID := registered.Workspace.ID
	userID := registered.User.ID
	defer func() {
		_, _ = pool.Exec(ctx, `delete from workspaces where id = $1`, workspaceID)
		_, _ = pool.Exec(ctx, `delete from users where id = $1`, userID)
	}()

	if registered.Membership.Role != "owner" {
		t.Fatalf("role = %q, want owner", registered.Membership.Role)
	}

	var me model.AuthResult
	requestJSON(t, server.URL, http.MethodGet, "/api/auth/me", registered.Token, nil, http.StatusOK, &me)
	if me.User.Email != email {
		t.Fatalf("me email = %q", me.User.Email)
	}

	graph := flow.Sample()
	graph.ID = fmt.Sprintf("flow_test_%d", suffix)
	graph.Name = "Fluxo de teste"
	if err := st.CreateFlow(ctx, workspaceID, graph); err != nil {
		t.Fatalf("create flow: %v", err)
	}

	channelKey := fmt.Sprintf("pk_test_%d", suffix)
	if err := st.CreateChannel(ctx, model.Channel{
		ID:             fmt.Sprintf("ch_test_%d", suffix),
		WorkspaceID:    workspaceID,
		Key:            channelKey,
		Name:           "Canal de teste",
		FlowID:         graph.ID,
		Appearance:     model.DefaultAppearance,
		AllowedOrigins: []string{"https://allowed.test"},
		CreatedAt:      time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create channel: %v", err)
	}

	var denied map[string]string
	requestJSONWithOrigin(t, server.URL, http.MethodGet, "/api/widget/config/"+channelKey, "https://blocked.test", http.StatusForbidden, &denied)

	var config struct {
		Key        string           `json:"key"`
		Name       string           `json:"name"`
		Appearance model.Appearance `json:"appearance"`
	}
	requestJSONWithOrigin(t, server.URL, http.MethodGet, "/api/widget/config/"+channelKey, "https://allowed.test", http.StatusOK, &config)
	if config.Key != channelKey {
		t.Fatalf("config key = %q", config.Key)
	}

	type messageBundle struct {
		Conversation model.Conversation `json:"conversation"`
		Messages     []model.Message    `json:"messages"`
	}

	var started messageBundle
	requestJSONWithOrigin(t, server.URL, http.MethodPost, "/api/widget/start/"+channelKey, "https://allowed.test", http.StatusCreated, &started)
	conversationID := started.Conversation.ID

	if started.Conversation.Status != flow.StatusWaiting || started.Conversation.Waiting == nil {
		t.Fatalf("start status = %q waiting = %+v", started.Conversation.Status, started.Conversation.Waiting)
	}

	var withName messageBundle
	requestJSONWithOriginBody(t, server.URL, http.MethodPost, "/api/widget/conversations/"+conversationID+"/messages", "https://allowed.test", http.StatusOK, map[string]string{"text": "Maria"}, &withName)
	if withName.Conversation.Waiting == nil || withName.Conversation.Waiting.InputType != "choice" {
		t.Fatalf("waiting = %+v", withName.Conversation.Waiting)
	}
	if len(withName.Conversation.Waiting.Options) != 3 {
		t.Fatalf("options = %d, want 3", len(withName.Conversation.Waiting.Options))
	}

	var hydrated model.Conversation
	requestJSONWithOrigin(t, server.URL, http.MethodGet, "/api/widget/conversations/"+conversationID, "https://allowed.test", http.StatusOK, &hydrated)
	if hydrated.Waiting == nil || hydrated.Waiting.InputType != "choice" || len(hydrated.Waiting.Options) != 3 {
		t.Fatalf("waiting após recarregar = %+v", hydrated.Waiting)
	}

	var handoff messageBundle
	requestJSONWithOriginBody(t, server.URL, http.MethodPost, "/api/widget/conversations/"+conversationID+"/messages", "https://allowed.test", http.StatusOK, map[string]string{"text": "Suporte"}, &handoff)
	if handoff.Conversation.Status != flow.StatusQueued {
		t.Fatalf("handoff status = %q", handoff.Conversation.Status)
	}

	var list []model.Conversation
	requestJSON(t, server.URL, http.MethodGet, "/api/conversations?status=queued", registered.Token, nil, http.StatusOK, &list)
	if len(list) != 1 {
		t.Fatalf("queued conversations = %d", len(list))
	}

	var replied messageBundle
	requestJSON(t, server.URL, http.MethodPost, "/api/conversations/"+conversationID+"/messages", registered.Token, map[string]string{"text": "Oi Maria, aqui é a Ana."}, http.StatusOK, &replied)
	if replied.Conversation.Status != flow.StatusHuman || replied.Conversation.AssignedUserID != userID {
		t.Fatalf("reply conversation = %+v", replied.Conversation)
	}

	var patched model.Conversation
	requestJSON(t, server.URL, http.MethodPatch, "/api/conversations/"+conversationID, registered.Token, map[string]any{
		"status": "closed",
		"tags":   []string{"vip", "teste"},
	}, http.StatusOK, &patched)
	if patched.Status != flow.StatusClosed || len(patched.Tags) != 2 {
		t.Fatalf("patched = %+v", patched)
	}

	var tagged []model.Conversation
	requestJSON(t, server.URL, http.MethodGet, "/api/conversations?search=vip", registered.Token, nil, http.StatusOK, &tagged)
	if len(tagged) != 1 || tagged[0].ID != conversationID {
		t.Fatalf("busca por tag = %+v", tagged)
	}

	var messages []model.Message
	requestJSON(t, server.URL, http.MethodGet, "/api/conversations/"+conversationID+"/messages", registered.Token, nil, http.StatusOK, &messages)
	if len(messages) < 5 {
		t.Fatalf("messages = %d", len(messages))
	}

	var stats model.Stats
	requestJSON(t, server.URL, http.MethodGet, "/api/stats", registered.Token, nil, http.StatusOK, &stats)
	if stats.Total != 1 || stats.Closed != 1 || len(stats.ConversationsByDay) != 7 {
		t.Fatalf("stats = %+v", stats)
	}

	var agents []model.Agent
	requestJSON(t, server.URL, http.MethodGet, "/api/agents", registered.Token, nil, http.StatusOK, &agents)
	if len(agents) != 1 {
		t.Fatalf("agents = %d", len(agents))
	}

	var flows []model.FlowSummary
	requestJSON(t, server.URL, http.MethodGet, "/api/flows", registered.Token, nil, http.StatusOK, &flows)
	if len(flows) != 2 {
		t.Fatalf("flows = %d, want 2 (starter + journey)", len(flows))
	}
}

func TestFlowRuntimeEffects(t *testing.T) {
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

	workerCtx, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()
	go chatService.StartDelayWorker(workerCtx)

	server := httptest.NewServer(httpapi.New(st, chatService, accountsService, tokens, hub))
	defer server.Close()

	webhookEvents := make(chan map[string]any, 1)
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		select {
		case webhookEvents <- payload:
		default:
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer webhook.Close()

	suffix := time.Now().UnixNano()
	email := fmt.Sprintf("effects_%d@test.dev", suffix)

	var registered model.AuthResponse
	requestJSON(t, server.URL, http.MethodPost, "/api/auth/register", "", map[string]any{
		"name":          "Efeitos Teste",
		"email":         email,
		"password":      "senha12345",
		"workspaceName": "Workspace Efeitos",
	}, http.StatusCreated, &registered)

	workspaceID := registered.Workspace.ID
	userID := registered.User.ID
	defer func() {
		_, _ = pool.Exec(ctx, `delete from workspaces where id = $1`, workspaceID)
		_, _ = pool.Exec(ctx, `delete from users where id = $1`, userID)
	}()

	graph := flow.Graph{
		ID:      fmt.Sprintf("flow_effects_%d", suffix),
		Name:    "Fluxo de efeitos",
		Version: 1,
		Nodes: []flow.Node{
			{ID: "start", Type: flow.NodeTrigger},
			{ID: "tag", Type: flow.NodeTag, Config: map[string]any{"tags": []any{"vip", "teste"}}},
			{ID: "assign", Type: flow.NodeAssign, Config: map[string]any{"agentId": userID}},
			{ID: "hook", Type: flow.NodeWebhook, Config: map[string]any{"url": webhook.URL}},
			{ID: "wait", Type: flow.NodeDelay, Config: map[string]any{"ms": 1200}},
			{ID: "message", Type: flow.NodeMessage, Config: map[string]any{"text": "Depois do atraso"}},
			{ID: "end", Type: flow.NodeEnd},
		},
		Edges: []flow.Edge{
			{ID: "e1", Source: "start", Target: "tag"},
			{ID: "e2", Source: "tag", Target: "assign"},
			{ID: "e3", Source: "assign", Target: "hook"},
			{ID: "e4", Source: "hook", Target: "wait"},
			{ID: "e5", Source: "wait", Target: "message"},
			{ID: "e6", Source: "message", Target: "end"},
		},
	}
	graph.EntryNodeID = "start"

	if err := st.CreateFlow(ctx, workspaceID, graph); err != nil {
		t.Fatalf("create flow: %v", err)
	}

	channelKey := fmt.Sprintf("pk_effects_%d", suffix)
	if err := st.CreateChannel(ctx, model.Channel{
		ID:             fmt.Sprintf("ch_effects_%d", suffix),
		WorkspaceID:    workspaceID,
		Key:            channelKey,
		Name:           "Canal de efeitos",
		FlowID:         graph.ID,
		Appearance:     model.DefaultAppearance,
		AllowedOrigins: []string{},
		CreatedAt:      time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create channel: %v", err)
	}

	var started struct {
		Conversation model.Conversation `json:"conversation"`
	}
	requestJSONWithOrigin(t, server.URL, http.MethodPost, "/api/widget/start/"+channelKey, "https://allowed.test", http.StatusCreated, &started)

	if len(started.Conversation.Tags) != 2 {
		t.Fatalf("tags = %+v", started.Conversation.Tags)
	}
	if started.Conversation.AssignedUserID != userID {
		t.Fatalf("assigned = %q", started.Conversation.AssignedUserID)
	}

	select {
	case payload := <-webhookEvents:
		if payload["conversationId"] != started.Conversation.ID {
			t.Fatalf("webhook payload = %+v", payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("webhook não foi chamado")
	}

	conversationID := started.Conversation.ID
	deadline := time.Now().Add(5 * time.Second)
	for {
		var conversation model.Conversation
		requestJSONWithOrigin(t, server.URL, http.MethodGet, "/api/widget/conversations/"+conversationID, "https://allowed.test", http.StatusOK, &conversation)
		if conversation.Status == flow.StatusClosed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("conversa não retomou após o atraso: %+v", conversation)
		}
		time.Sleep(200 * time.Millisecond)
	}

	var messages []model.Message
	requestJSONWithOrigin(t, server.URL, http.MethodGet, "/api/widget/conversations/"+conversationID+"/messages", "https://allowed.test", http.StatusOK, &messages)
	found := false
	for _, message := range messages {
		if message.Text == "Depois do atraso" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("mensagem do atraso não encontrada: %+v", messages)
	}
}

func requestJSON(t *testing.T, base, method, path, token string, body any, status int, out any) {
	t.Helper()
	requestJSONWithOriginBody(t, base, method, path, "", status, body, out, token)
}

func requestJSONWithOrigin(t *testing.T, base, method, path, origin string, status int, out any) {
	t.Helper()
	requestJSONWithOriginBody(t, base, method, path, origin, status, nil, out)
}

func requestJSONWithOriginBody(t *testing.T, base, method, path, origin string, status int, body any, out any, tokens ...string) {
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
	if len(tokens) > 0 && tokens[0] != "" {
		request.Header.Set("Authorization", "Bearer "+tokens[0])
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer response.Body.Close()

	payload, _ := io.ReadAll(response.Body)
	if response.StatusCode != status {
		t.Fatalf("%s %s status = %d, want %d: %s", method, path, response.StatusCode, status, string(payload))
	}

	if out != nil && len(payload) > 0 {
		if err := json.Unmarshal(payload, out); err != nil {
			t.Fatalf("decode %s: %v (%s)", path, err, string(payload))
		}
	}
}
