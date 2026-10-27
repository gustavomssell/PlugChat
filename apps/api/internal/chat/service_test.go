package chat

import (
	"context"
	"testing"

	"chatfree/api/internal/flow"
	"chatfree/api/internal/model"
)

func TestLimitsNormalization(t *testing.T) {
	limits := Limits{}.normalized()
	if limits.MaxMessageRunes != DefaultLimits().MaxMessageRunes {
		t.Fatalf("MaxMessageRunes = %d", limits.MaxMessageRunes)
	}
	if limits.MaxVisitorIDRunes != DefaultLimits().MaxVisitorIDRunes {
		t.Fatalf("MaxVisitorIDRunes = %d", limits.MaxVisitorIDRunes)
	}

	custom := Limits{MaxMessageRunes: 10, MaxVisitorIDRunes: 4}.normalized()
	if custom.MaxMessageRunes != 10 || custom.MaxVisitorIDRunes != 4 {
		t.Fatalf("custom = %+v", custom)
	}
}

func TestValidVisitorID(t *testing.T) {
	service := &Service{limits: Limits{MaxMessageRunes: 10, MaxVisitorIDRunes: 20}}

	if !service.validVisitorID("visitante-123") {
		t.Fatal("id válido deveria ser aceito")
	}
	if service.validVisitorID("visitante-1234567890123") {
		t.Fatal("id acima do limite deveria ser inválido")
	}
	if service.validVisitorID("ab\nc") {
		t.Fatal("id com caractere de controle deveria ser inválido")
	}
	if service.validVisitorID("id\tcom\ttab") {
		t.Fatal("id com tabulação deveria ser inválido")
	}
	if !service.validVisitorID("uuid-abc-123") {
		t.Fatal("id com hífen deveria ser válido")
	}
}

func TestApplyEffectsKeepOrder(t *testing.T) {
	service := &Service{}
	conversation := model.Conversation{ID: "conv_test", Tags: []string{}}
	state := flow.State{Variables: map[string]any{}}

	messages, err := service.applyEffects(context.Background(), &conversation, &state, []flow.Effect{
		{Type: "send_message", Text: "primeira"},
		{Type: "send_message", Text: "segunda"},
		{Type: "handoff", Reason: "suporte"},
	})
	if err != nil {
		t.Fatalf("applyEffects: %v", err)
	}

	if len(messages) != 3 {
		t.Fatalf("mensagens = %d, quer 3", len(messages))
	}

	want := []string{"primeira", "segunda", "Encaminhando você para um atendente."}
	for index, message := range messages {
		if message.Text != want[index] {
			t.Fatalf("mensagem %d = %q, quer %q", index, message.Text, want[index])
		}
		if index > 0 && !message.CreatedAt.After(messages[index-1].CreatedAt) {
			t.Fatalf("timestamp da mensagem %d não é posterior à anterior", index)
		}
	}
}

func TestApplyEffectsUpdatesTagsAndLLMFallback(t *testing.T) {
	service := &Service{}
	conversation := model.Conversation{ID: "conv_test", Tags: []string{"vip"}}
	state := flow.State{Variables: map[string]any{}}

	messages, err := service.applyEffects(context.Background(), &conversation, &state, []flow.Effect{
		{Type: "tag", Tags: []string{"suporte", "vip"}},
		{Type: "request_llm", Prompt: "responda", Variable: "answer", Text: "fallback"},
	})
	if err != nil {
		t.Fatalf("applyEffects: %v", err)
	}

	if len(conversation.Tags) != 2 || conversation.Tags[0] != "vip" || conversation.Tags[1] != "suporte" {
		t.Fatalf("tags = %+v", conversation.Tags)
	}
	if state.Variables["answer"] != "fallback" {
		t.Fatalf("variables = %+v", state.Variables)
	}
	if len(messages) != 1 || messages[0].Text != "fallback" {
		t.Fatalf("messages = %+v", messages)
	}
}
