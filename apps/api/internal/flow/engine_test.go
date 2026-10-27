package flow

import (
	"errors"
	"testing"
)

func linearGraph() Graph {
	return Graph{
		ID:      "flow_linear",
		Version: 1,
		Nodes: []Node{
			{ID: "start", Type: NodeTrigger},
			{ID: "hello", Type: NodeMessage, Config: map[string]any{"text": "Olá"}},
			{
				ID:   "ask_name",
				Type: NodeCollectInput,
				Config: map[string]any{
					"variable": "name",
					"prompt":   "Qual seu nome?",
				},
			},
			{ID: "done", Type: NodeMessage, Config: map[string]any{"text": "Obrigado!"}},
			{ID: "end", Type: NodeEnd},
		},
		Edges: []Edge{
			{ID: "e1", Source: "start", Target: "hello"},
			{ID: "e2", Source: "hello", Target: "ask_name"},
			{ID: "e3", Source: "ask_name", Target: "done"},
			{ID: "e4", Source: "done", Target: "end"},
		},
	}
}

func TestStartWaitsForInput(t *testing.T) {
	state, effects, err := Start(linearGraph(), "conv_1")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if state.Status != StatusWaiting {
		t.Fatalf("status = %q, want %q", state.Status, StatusWaiting)
	}
	if state.Waiting == nil || state.Waiting.Variable != "name" {
		t.Fatalf("waiting = %+v", state.Waiting)
	}
	if state.CurrentNodeID != "ask_name" {
		t.Fatalf("current node = %q", state.CurrentNodeID)
	}
	if len(effects) != 2 || effects[0].Text != "Olá" || effects[1].Text != "Qual seu nome?" {
		t.Fatalf("effects = %+v", effects)
	}
}

func TestResumeStoresInputAndFinishes(t *testing.T) {
	state, _, err := Start(linearGraph(), "conv_1")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	state, effects, err := Resume(linearGraph(), state, "Ana")
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}

	if state.Variables["name"] != "Ana" {
		t.Fatalf("variables = %+v", state.Variables)
	}
	if state.Status != StatusClosed {
		t.Fatalf("status = %q, want closed", state.Status)
	}
	if len(effects) != 1 || effects[0].Text != "Obrigado!" {
		t.Fatalf("effects = %+v", effects)
	}
}

func TestResumeRequiresWaitingState(t *testing.T) {
	state, _, err := Start(linearGraph(), "conv_1")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	state, _, err = Resume(linearGraph(), state, "Ana")
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}

	if _, _, err := Resume(linearGraph(), state, "de novo"); !errors.Is(err, ErrNotWaiting) {
		t.Fatalf("err = %v, want ErrNotWaiting", err)
	}
}

func conditionGraph() Graph {
	return Graph{
		ID:      "flow_condition",
		Version: 1,
		Nodes: []Node{
			{ID: "start", Type: NodeTrigger},
			{
				ID:   "ask_topic",
				Type: NodeCollectInput,
				Config: map[string]any{
					"variable": "topic",
					"prompt":   "Assunto?",
				},
			},
			{
				ID:   "check",
				Type: NodeCondition,
				Config: map[string]any{
					"variable": "topic",
					"operator": "eq",
					"value":    "Suporte",
				},
			},
			{ID: "human", Type: NodeHandoff, Config: map[string]any{"reason": "suporte"}},
			{ID: "reply", Type: NodeMessage, Config: map[string]any{"text": "Registrado"}},
			{ID: "end", Type: NodeEnd},
		},
		Edges: []Edge{
			{ID: "e1", Source: "start", Target: "ask_topic"},
			{ID: "e2", Source: "ask_topic", Target: "check"},
			{ID: "e3", Source: "check", Target: "human", SourceHandle: "true"},
			{ID: "e4", Source: "check", Target: "reply", SourceHandle: "false"},
			{ID: "e5", Source: "reply", Target: "end"},
		},
	}
}

func TestConditionTrueBranchesToHandoff(t *testing.T) {
	state, _, err := Start(conditionGraph(), "conv_2")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	state, effects, err := Resume(conditionGraph(), state, "Suporte")
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}

	if state.Status != StatusQueued {
		t.Fatalf("status = %q, want queued", state.Status)
	}
	if len(effects) != 1 || effects[0].Type != "handoff" {
		t.Fatalf("effects = %+v", effects)
	}
}

func TestConditionFalseBranchFinishes(t *testing.T) {
	state, _, err := Start(conditionGraph(), "conv_3")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	state, effects, err := Resume(conditionGraph(), state, "Comercial")
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}

	if state.Status != StatusClosed {
		t.Fatalf("status = %q, want closed", state.Status)
	}
	if len(effects) != 1 || effects[0].Text != "Registrado" {
		t.Fatalf("effects = %+v", effects)
	}
}

func TestDelayPausesAndContinueResumes(t *testing.T) {
	graph := Graph{
		ID:      "flow_delay",
		Version: 1,
		Nodes: []Node{
			{ID: "start", Type: NodeTrigger},
			{ID: "wait", Type: NodeDelay, Config: map[string]any{"ms": 1500}},
			{ID: "done", Type: NodeMessage, Config: map[string]any{"text": "Pronto"}},
			{ID: "end", Type: NodeEnd},
		},
		Edges: []Edge{
			{ID: "e1", Source: "start", Target: "wait"},
			{ID: "e2", Source: "wait", Target: "done"},
			{ID: "e3", Source: "done", Target: "end"},
		},
	}

	state, effects, err := Start(graph, "conv_delay")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if state.PendingNodeID != "done" || state.WakeAt == nil {
		t.Fatalf("state = %+v", state)
	}
	if len(effects) != 2 || effects[0].Type != "typing" || effects[1].Type != "delay" || effects[1].MS != 1500 {
		t.Fatalf("effects = %+v", effects)
	}

	state, effects, err = Continue(graph, state)
	if err != nil {
		t.Fatalf("Continue: %v", err)
	}
	if state.Status != StatusClosed || state.PendingNodeID != "" || state.WakeAt != nil {
		t.Fatalf("state = %+v", state)
	}
	if len(effects) != 1 || effects[0].Text != "Pronto" {
		t.Fatalf("effects = %+v", effects)
	}
}

func shortcutGraph() Graph {
	return Graph{
		ID:      "flow_shortcut",
		Version: 1,
		Nodes: []Node{
			{ID: "start", Type: NodeTrigger},
			{
				ID:   "ask_topic",
				Type: NodeCollectInput,
				Config: map[string]any{
					"prompt":   "Como podemos ajudar?",
					"variable": "topic",
					"options": []any{
						map[string]any{"label": "Comercial", "value": "Comercial"},
						map[string]any{"label": "Suporte", "value": "Suporte", "nextNodeId": "human"},
						map[string]any{"label": "Financeiro", "value": "Financeiro"},
					},
					"fallbackNodeId": "unknown_topic",
				},
			},
			{ID: "commercial", Type: NodeMessage, Config: map[string]any{"text": "Setor comercial"}},
			{ID: "human", Type: NodeHandoff, Config: map[string]any{"reason": "suporte"}},
			{ID: "unknown_topic", Type: NodeMessage, Config: map[string]any{"text": "Não entendi"}},
			{ID: "end", Type: NodeEnd},
		},
		Edges: []Edge{
			{ID: "e1", Source: "start", Target: "ask_topic"},
			{ID: "e2", Source: "ask_topic", Target: "commercial"},
			{ID: "e3", Source: "commercial", Target: "end"},
			{ID: "e4", Source: "unknown_topic", Target: "ask_topic"},
		},
	}
}

func TestOptionShortcutJumpsDirectly(t *testing.T) {
	graph := shortcutGraph()

	state, _, err := Start(graph, "conv_shortcut")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if state.Waiting == nil || len(state.Waiting.Options) != 3 {
		t.Fatalf("waiting = %+v", state.Waiting)
	}
	if state.Waiting.Options[1].Next != "human" {
		t.Fatalf("atalho da opção = %+v", state.Waiting.Options[1])
	}

	state, effects, err := Resume(graph, state, "Suporte")
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if state.Status != StatusQueued {
		t.Fatalf("status = %q, want queued", state.Status)
	}
	if len(effects) != 1 || effects[0].Type != "handoff" {
		t.Fatalf("effects = %+v", effects)
	}
	if state.Variables["topic"] != "Suporte" {
		t.Fatalf("variables = %+v", state.Variables)
	}
}

func TestUnmatchedOptionFallsBack(t *testing.T) {
	graph := shortcutGraph()

	state, _, err := Start(graph, "conv_fallback")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	state, effects, err := Resume(graph, state, "quero falar de outra coisa")
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	// O nó de recuperação explica e volta a perguntar o assunto.
	if len(effects) != 2 || effects[0].Text != "Não entendi" || effects[1].Text != "Como podemos ajudar?" {
		t.Fatalf("effects = %+v", effects)
	}
	if state.Status != StatusWaiting || state.Waiting == nil || state.Waiting.NodeID != "ask_topic" {
		t.Fatalf("state = %+v", state)
	}
	if state.Variables["topic"] != "quero falar de outra coisa" {
		t.Fatalf("variables = %+v", state.Variables)
	}
}

func TestFormCollectsSeveralFieldsAtOnce(t *testing.T) {
	graph := Graph{
		ID:      "flow_form",
		Version: 1,
		Nodes: []Node{
			{ID: "start", Type: NodeTrigger},
			{
				ID:   "ask_contact",
				Type: NodeCollectInput,
				Config: map[string]any{
					"prompt": "Para agilizar, me passa seus dados:",
					"fields": []any{
						map[string]any{"name": "name", "label": "Nome", "type": "text", "required": true},
						map[string]any{"name": "email", "label": "E-mail", "type": "email", "required": true},
						map[string]any{"name": "phone", "label": "Telefone", "type": "phone"},
					},
				},
			},
			{ID: "done", Type: NodeMessage, Config: map[string]any{"text": "Obrigado!"}},
			{ID: "end", Type: NodeEnd},
		},
		Edges: []Edge{
			{ID: "e1", Source: "start", Target: "ask_contact"},
			{ID: "e2", Source: "ask_contact", Target: "done"},
			{ID: "e3", Source: "done", Target: "end"},
		},
	}

	state, effects, err := Start(graph, "conv_form")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if state.Waiting == nil || state.Waiting.InputType != "form" || len(state.Waiting.Fields) != 3 {
		t.Fatalf("waiting = %+v", state.Waiting)
	}
	if len(effects) != 1 || effects[0].Text != "Para agilizar, me passa seus dados:" {
		t.Fatalf("effects = %+v", effects)
	}

	// Campo obrigatório vazio bloqueia o avanço e informa qual.
	if _, _, err := ResumeInput(graph, state, VisitorInput{Fields: map[string]string{"name": "Ana"}}); !errors.Is(err, ErrMissingField) {
		t.Fatalf("err = %v, want ErrMissingField", err)
	}

	// Campos desconhecidos são ignorados.
	state, effects, err = ResumeInput(graph, state, VisitorInput{Fields: map[string]string{
		"name":     "Ana",
		"email":    "ana@example.com",
		"nickname": "aninha",
	}})
	if err != nil {
		t.Fatalf("ResumeInput: %v", err)
	}
	if state.Variables["name"] != "Ana" || state.Variables["email"] != "ana@example.com" {
		t.Fatalf("variables = %+v", state.Variables)
	}
	if _, exists := state.Variables["nickname"]; exists {
		t.Fatalf("campo desconhecido foi aceito: %+v", state.Variables)
	}
	if len(effects) != 1 || effects[0].Text != "Obrigado!" {
		t.Fatalf("effects = %+v", effects)
	}
}

func TestConversationStepLimitStopsLoops(t *testing.T) {
	graph := Graph{
		ID:      "flow_loop",
		Version: 1,
		Nodes: []Node{
			{ID: "start", Type: NodeTrigger},
			{ID: "back", Type: NodeMessage, Config: map[string]any{"text": "de novo"}},
			{ID: "ask", Type: NodeCollectInput, Config: map[string]any{"variable": "x", "prompt": "continua?"}},
		},
		Edges: []Edge{
			{ID: "e1", Source: "start", Target: "ask"},
			{ID: "e2", Source: "ask", Target: "back"},
			{ID: "e3", Source: "back", Target: "ask"},
		},
	}

	state, _, err := Start(graph, "conv_loop")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	var lastErr error
	for i := 0; i < 600; i++ {
		state, _, lastErr = Resume(graph, state, "sim")
		if lastErr != nil {
			break
		}
	}

	if !errors.Is(lastErr, ErrTooManySteps) {
		t.Fatalf("err = %v, want ErrTooManySteps", lastErr)
	}
}

func TestLLMContinuesWithoutWaitingForInput(t *testing.T) {
	graph := Graph{
		ID:      "flow_llm",
		Version: 1,
		Nodes: []Node{
			{ID: "start", Type: NodeTrigger},
			{ID: "ai", Type: NodeLLM, Config: map[string]any{"prompt": "ajude", "variable": "answer"}},
			{ID: "end", Type: NodeEnd},
		},
		Edges: []Edge{
			{ID: "e1", Source: "start", Target: "ai"},
			{ID: "e2", Source: "ai", Target: "end"},
		},
	}

	state, effects, err := Start(graph, "conv_llm")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if state.Status != StatusClosed || state.Waiting != nil {
		t.Fatalf("state = %+v", state)
	}
	if len(effects) != 2 || effects[0].Type != "typing" || effects[1].Type != "request_llm" || effects[1].Variable != "answer" {
		t.Fatalf("effects = %+v", effects)
	}
}
