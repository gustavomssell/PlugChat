package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
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
	"chatfree/api/internal/files"
	"chatfree/api/internal/flow"
	"chatfree/api/internal/httpapi"
	"chatfree/api/internal/model"
	"chatfree/api/internal/realtime"
	"chatfree/api/internal/store"
)

// TestCollaborationFeatures percorre notas internas, respostas prontas,
// ações em massa, métricas, exportação CSV e anexos em uma conversa real.
func TestCollaborationFeatures(t *testing.T) {
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

	attachmentStore, err := files.New(t.TempDir())
	if err != nil {
		t.Fatalf("files: %v", err)
	}

	st := store.NewPostgres(pool)
	hub := realtime.NewHub()
	tokens := auth.NewManager("test-secret", time.Hour)
	chatService := chat.New(st, hub)
	accountsService := accounts.New(st, tokens)

	server := httptest.NewServer(httpapi.New(
		st,
		chatService,
		accountsService,
		tokens,
		hub,
		httpapi.WithAttachments(attachmentStore),
	))
	defer server.Close()

	suffix := time.Now().UnixNano()
	email := fmt.Sprintf("collab_%d@test.dev", suffix)

	var registered model.AuthResponse
	requestJSON(t, server.URL, http.MethodPost, "/api/auth/register", "", map[string]any{
		"name":          "Colaborador Teste",
		"email":         email,
		"password":      "senha12345",
		"workspaceName": "Workspace Colaboração",
	}, http.StatusCreated, &registered)

	workspaceID := registered.Workspace.ID
	userID := registered.User.ID
	defer func() {
		_, _ = pool.Exec(ctx, `delete from workspaces where id = $1`, workspaceID)
		_, _ = pool.Exec(ctx, `delete from users where id = $1`, userID)
	}()

	graph := flow.Sample()
	graph.ID = fmt.Sprintf("flow_collab_%d", suffix)
	graph.Name = "Fluxo de colaboração"
	if err := st.CreateFlow(ctx, workspaceID, graph); err != nil {
		t.Fatalf("create flow: %v", err)
	}

	channelKey := fmt.Sprintf("pk_collab_%d", suffix)
	if err := st.CreateChannel(ctx, model.Channel{
		ID:             fmt.Sprintf("ch_collab_%d", suffix),
		WorkspaceID:    workspaceID,
		Key:            channelKey,
		Name:           "Canal de colaboração",
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
	conversationID := started.Conversation.ID

	// Nota interna: aparece para a equipe e não para o visitante.
	var note model.Message
	requestJSON(t, server.URL, http.MethodPost, "/api/conversations/"+conversationID+"/notes", registered.Token, map[string]string{
		"text": "Cliente retorna sempre no mesmo assunto.",
	}, http.StatusCreated, &note)
	if note.Role != "note" || note.AuthorName == "" {
		t.Fatalf("note = %+v", note)
	}

	var agentMessages []model.Message
	requestJSON(t, server.URL, http.MethodGet, "/api/conversations/"+conversationID+"/messages", registered.Token, nil, http.StatusOK, &agentMessages)
	noteCount := 0
	for _, message := range agentMessages {
		if message.Role == "note" {
			noteCount++
		}
	}
	if noteCount != 1 {
		t.Fatalf("mensagens do painel deveriam conter 1 nota, vieram %d", noteCount)
	}

	var visitorMessages []model.Message
	requestJSONWithOrigin(t, server.URL, http.MethodGet, "/api/widget/conversations/"+conversationID+"/messages", "https://allowed.test", http.StatusOK, &visitorMessages)
	for _, message := range visitorMessages {
		if message.Role == "note" {
			t.Fatalf("nota interna vazou para o widget: %+v", message)
		}
	}

	var invalidNote map[string]string
	requestJSON(t, server.URL, http.MethodPost, "/api/conversations/"+conversationID+"/notes", registered.Token, map[string]string{
		"text": "   ",
	}, http.StatusBadRequest, &invalidNote)

	// Respostas prontas: criar, listar, atualizar e remover.
	var reply model.SavedReply
	requestJSON(t, server.URL, http.MethodPost, "/api/replies", registered.Token, map[string]any{
		"title":    "Horário de atendimento",
		"body":     "Atendemos de segunda a sexta, das 9h às 18h.",
		"shortcut": "horario",
		"tags":     []string{"horario"},
	}, http.StatusCreated, &reply)
	if reply.ID == "" || reply.Shortcut != "horario" {
		t.Fatalf("reply = %+v", reply)
	}

	var duplicate map[string]string
	requestJSON(t, server.URL, http.MethodPost, "/api/replies", registered.Token, map[string]any{
		"title":    "Outro horário",
		"body":     "Texto",
		"shortcut": "horario",
	}, http.StatusBadRequest, &duplicate)

	var replies []model.SavedReply
	requestJSON(t, server.URL, http.MethodGet, "/api/replies", registered.Token, nil, http.StatusOK, &replies)
	if len(replies) != 1 {
		t.Fatalf("replies = %+v", replies)
	}

	var updated model.SavedReply
	requestJSON(t, server.URL, http.MethodPut, "/api/replies/"+reply.ID, registered.Token, map[string]any{
		"title":    "Horário",
		"body":     "Atendemos de segunda a sexta, das 9h às 18h (horário de Brasília).",
		"shortcut": "horario",
	}, http.StatusOK, &updated)
	if !strings.Contains(updated.Body, "Brasília") {
		t.Fatalf("updated = %+v", updated)
	}

	// Ação em massa: marca tag em todas as conversas do workspace.
	var bulk map[string]int
	requestJSON(t, server.URL, http.MethodPost, "/api/conversations/bulk", registered.Token, map[string]any{
		"ids":    []string{conversationID},
		"action": "tag",
		"tags":   []string{"follow-up"},
	}, http.StatusOK, &bulk)
	if bulk["updated"] != 1 {
		t.Fatalf("bulk = %+v", bulk)
	}

	var afterBulk model.Conversation
	requestJSON(t, server.URL, http.MethodGet, "/api/conversations/"+conversationID, registered.Token, nil, http.StatusOK, &afterBulk)
	if len(afterBulk.Tags) != 1 || afterBulk.Tags[0] != "follow-up" {
		t.Fatalf("tags após bulk = %+v", afterBulk.Tags)
	}

	var invalidBulk map[string]string
	requestJSON(t, server.URL, http.MethodPost, "/api/conversations/bulk", registered.Token, map[string]any{
		"ids":    []string{conversationID},
		"action": "apagar",
	}, http.StatusBadRequest, &invalidBulk)

	// Anexos: upload pelo painel, envio na resposta e download autenticado.
	var attachment model.Attachment
	uploadMultipart(t, server.URL, "/api/conversations/"+conversationID+"/attachments", registered.Token, "file", "print.png", "image/png", []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, http.StatusCreated, &attachment)
	if attachment.ID == "" || attachment.Kind != model.KindImage {
		t.Fatalf("attachment = %+v", attachment)
	}

	var unsupported map[string]string
	uploadMultipart(t, server.URL, "/api/conversations/"+conversationID+"/attachments", registered.Token, "file", "script.js", "text/javascript", []byte("alert(1)"), http.StatusUnsupportedMediaType, &unsupported)

	var replied struct {
		Conversation model.Conversation `json:"conversation"`
		Messages     []model.Message    `json:"messages"`
	}
	requestJSON(t, server.URL, http.MethodPost, "/api/conversations/"+conversationID+"/messages", registered.Token, map[string]any{
		"text":          "Segue o print do erro.",
		"attachmentIds": []string{attachment.ID},
	}, http.StatusOK, &replied)
	if len(replied.Messages) != 1 || len(replied.Messages[0].Attachments) != 1 {
		t.Fatalf("reply com anexo = %+v", replied)
	}
	if replied.Messages[0].Attachments[0].Filename != "print.png" {
		t.Fatalf("anexo da mensagem = %+v", replied.Messages[0].Attachments[0])
	}

	var foreign map[string]string
	requestJSON(t, server.URL, http.MethodPost, "/api/conversations/"+conversationID+"/messages", registered.Token, map[string]any{
		"text":          "anexo de outra conversa",
		"attachmentIds": []string{"att_inexistente"},
	}, http.StatusBadRequest, &foreign)

	requestAttachmentDownload(t, server.URL, attachment.ID, registered.Token, http.StatusOK)

	// Métricas e exportação.
	var metrics model.Metrics
	requestJSON(t, server.URL, http.MethodGet, "/api/metrics?days=7", registered.Token, nil, http.StatusOK, &metrics)
	if metrics.Conversations < 1 || len(metrics.ConversationsByDay) == 0 {
		t.Fatalf("metrics = %+v", metrics)
	}
	foundChannel := false
	for _, channel := range metrics.ByChannel {
		if channel.Name == "Canal de colaboração" {
			foundChannel = true
			if channel.Conversations != 1 {
				t.Fatalf("canal colaboração = %+v", channel)
			}
		}
	}
	if !foundChannel {
		t.Fatalf("metrics byChannel = %+v", metrics.ByChannel)
	}

	body := requestRaw(t, server.URL, http.MethodGet, "/api/conversations/export", registered.Token, nil, http.StatusOK)
	if !strings.Contains(string(body), "id,canal,visitante") || !strings.Contains(string(body), conversationID) {
		t.Fatalf("csv inesperado: %s", string(body))
	}

	var missing map[string]string
	requestJSON(t, server.URL, http.MethodGet, "/api/conversations/conv_que_nao_existe/messages", registered.Token, nil, http.StatusNotFound, &missing)

	requestJSON(t, server.URL, http.MethodDelete, "/api/replies/"+reply.ID, registered.Token, nil, http.StatusNoContent, nil)
	requestJSON(t, server.URL, http.MethodGet, "/api/replies", registered.Token, nil, http.StatusOK, &replies)
	if len(replies) != 0 {
		t.Fatalf("replies após delete = %+v", replies)
	}
}

func uploadMultipart(t *testing.T, base, path, token, field, filename, mime string, content []byte, status int, out any) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("create part: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	request, err := http.NewRequest(http.MethodPost, base+path, &body)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer response.Body.Close()

	payload, _ := io.ReadAll(response.Body)
	if response.StatusCode != status {
		t.Fatalf("%s %s status = %d, want %d: %s", http.MethodPost, path, response.StatusCode, status, string(payload))
	}
	if out != nil && len(payload) > 0 {
		if err := json.Unmarshal(payload, out); err != nil {
			t.Fatalf("decode %s: %v (%s)", path, err, string(payload))
		}
	}
}

func requestRaw(t *testing.T, base, method, path, token string, body any, status int) []byte {
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
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
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

	return payload
}

func requestAttachmentDownload(t *testing.T, base, id, token string, status int) {
	t.Helper()

	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/attachments/%s?token=%s", base, id, token), nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != status {
		payload, _ := io.ReadAll(response.Body)
		t.Fatalf("download status = %d, want %d: %s", response.StatusCode, status, string(payload))
	}
}
