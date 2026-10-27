package chat

import (
	"strings"
	"testing"
)

func TestBuildReplyValidation(t *testing.T) {
	valid := SavedReplyInput{Title: "Horário", Body: "Atendemos das 9h às 18h.", Shortcut: "horario"}
	reply, err := buildReply("ws_test", valid)
	if err != nil {
		t.Fatalf("buildReply: %v", err)
	}
	if reply.ID == "" || reply.WorkspaceID != "ws_test" {
		t.Fatalf("reply = %+v", reply)
	}
	if reply.Shortcut != "horario" || len(reply.Tags) != 0 {
		t.Fatalf("reply = %+v", reply)
	}
	if reply.CreatedAt.IsZero() || !reply.UpdatedAt.Equal(reply.CreatedAt) {
		t.Fatalf("timestamps = %+v", reply)
	}

	if _, err := buildReply("ws_test", SavedReplyInput{Title: "x", Body: "corpo"}); err == nil {
		t.Fatal("título curto deveria ser recusado")
	}
	if _, err := buildReply("ws_test", SavedReplyInput{Title: "Titulo", Body: "   "}); err == nil {
		t.Fatal("corpo vazio deveria ser recusado")
	}
	if _, err := buildReply("ws_test", SavedReplyInput{Title: "Titulo", Body: "corpo", Shortcut: "Atalho Invalido"}); err == nil {
		t.Fatal("atalho com maiúsculas deveria ser recusado")
	}

	long := strings.Repeat("a", MaxNoteRunes+1)
	if _, err := buildReply("ws_test", SavedReplyInput{Title: "Titulo", Body: long}); err == nil {
		t.Fatal("corpo acima do limite deveria ser recusado")
	}
}

func TestSanitizeFilename(t *testing.T) {
	cases := map[string]string{
		"  relatorio.pdf":     "relatorio.pdf",
		"../../etc/passwd":    "etcpasswd",
		"foto.jpeg":           "foto.jpeg",
		"nome\ncom\tcontrole": "nomecomcontrole",
		"":                    "",
	}

	for input, want := range cases {
		if got := sanitizeFilename(input); got != want {
			t.Fatalf("sanitizeFilename(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestBulkActionValidation(t *testing.T) {
	service := &Service{}

	if _, err := service.BulkPatch(nil, "ws_test", BulkAction{IDs: []string{"conv_1"}, Action: "apagar"}); err != ErrInvalidBulkAction {
		t.Fatalf("ação inválida deveria falhar, veio %v", err)
	}
	if _, err := service.BulkPatch(nil, "ws_test", BulkAction{Action: BulkAssign}); err != ErrBulkTooManyIDs {
		t.Fatalf("lista vazia deveria falhar, veio %v", err)
	}

	tooMany := make([]string, 0, MaxBulkIDs+1)
	for i := 0; i <= MaxBulkIDs; i++ {
		tooMany = append(tooMany, "conv_x_"+strings.Repeat("d", i+1))
	}
	if _, err := service.BulkPatch(nil, "ws_test", BulkAction{IDs: tooMany, Action: BulkTag}); err != ErrBulkTooManyIDs {
		t.Fatalf("lista grande deveria falhar, veio %v", err)
	}

	if _, err := service.BulkPatch(nil, "ws_test", BulkAction{IDs: []string{"conv_1"}, Action: BulkStatus, Status: "inexistente"}); err != ErrInvalidStatus {
		t.Fatalf("status inválido deveria falhar, veio %v", err)
	}
}
