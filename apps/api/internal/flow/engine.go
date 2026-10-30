package flow

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	ErrNotWaiting  = errors.New("flow: conversation is not waiting for input")
	ErrNotPending  = errors.New("flow: conversation has no pending step")
	ErrEmptyGraph  = errors.New("flow: graph has no nodes")
	ErrInvalidNode = errors.New("flow: invalid node")
	// ErrTooManySteps protects the runtime from graphs that loop forever.
	ErrTooManySteps = errors.New("flow: step limit exceeded")
	// ErrMissingField means a required form field came empty.
	ErrMissingField = errors.New("flow: missing required field")
)

const (
	// maxSteps limits one execution burst (Start, Resume or Continue).
	maxSteps = 200
	// maxConversationSteps limits one conversation across its whole life.
	maxConversationSteps = 500
	// maxFormFields keeps a form readable and the payload small.
	maxFormFields = 8
)

func Start(graph Graph, conversationID string) (State, []Effect, error) {
	if len(graph.Nodes) == 0 {
		return State{}, nil, ErrEmptyGraph
	}

	entry := graph.EntryNodeID
	if entry == "" {
		for _, node := range graph.Nodes {
			if node.Type == NodeTrigger {
				entry = node.ID
				break
			}
		}
	}
	if entry == "" {
		entry = graph.Nodes[0].ID
	}

	state := State{
		ConversationID: conversationID,
		FlowID:         graph.ID,
		FlowVersion:    graph.Version,
		CurrentNodeID:  entry,
		Status:         StatusBot,
		Variables:      map[string]any{},
	}

	effects, err := run(graph, &state, entry)
	return state, effects, err
}

// Resume continues a conversation after the visitor answered with free text or
// a single option. It keeps the simple string API for existing callers.
func Resume(graph Graph, state State, text string) (State, []Effect, error) {
	return ResumeInput(graph, state, VisitorInput{Text: text})
}

// ResumeInput is the full entry point: it accepts text, a chosen option or a
// filled form, resolves where the conversation goes next and runs from there.
func ResumeInput(graph Graph, state State, input VisitorInput) (State, []Effect, error) {
	if state.Status != StatusWaiting || state.Waiting == nil {
		return state, nil, ErrNotWaiting
	}

	if state.Variables == nil {
		state.Variables = map[string]any{}
	}

	input.Text = strings.TrimSpace(input.Text)
	waiting := state.Waiting

	if len(waiting.Fields) > 0 {
		if err := fillFields(&state, waiting.Fields, input); err != nil {
			return state, nil, err
		}
	} else if waiting.Variable != "" && input.Text != "" {
		state.Variables[waiting.Variable] = input.Text
	}

	from := waiting.NodeID
	state.Waiting = nil
	state.PendingNodeID = ""
	state.WakeAt = nil
	state.Status = StatusBot

	next := nextNode(graph, from, "")

	// Atalhos: uma opção pode apontar direto para o destino. Quando a resposta
	// não casa com nenhuma opção e existe um nó de recuperação, vai para ele.
	if len(waiting.Options) > 0 && input.Text != "" {
		if option, ok := matchOption(waiting.Options, input.Text); ok {
			if option.Next != "" && nodeExists(graph, option.Next) {
				next = option.Next
			}
		} else if waiting.FallbackNodeID != "" && nodeExists(graph, waiting.FallbackNodeID) {
			next = waiting.FallbackNodeID
		}
	}

	if next == "" {
		state.Status = StatusClosed
		return state, nil, nil
	}

	effects, err := run(graph, &state, next)
	return state, effects, err
}

// fillFields stores the submitted form values. Only field names declared by
// the node are accepted, so unknown keys in the payload are ignored.
func fillFields(state *State, fields []Field, input VisitorInput) error {
	for _, field := range fields {
		value := strings.TrimSpace(input.Fields[field.Name])
		if value == "" {
			if field.Required {
				label := strings.TrimSpace(field.Label)
				if label == "" {
					label = field.Name
				}
				return fmt.Errorf("%w: %s", ErrMissingField, label)
			}
			continue
		}
		state.Variables[field.Name] = value
	}

	return nil
}

func matchOption(options []Option, answer string) (Option, bool) {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return Option{}, false
	}

	for _, option := range options {
		if strings.EqualFold(strings.TrimSpace(option.Value), answer) {
			return option, true
		}
	}
	for _, option := range options {
		if strings.EqualFold(strings.TrimSpace(option.Label), answer) {
			return option, true
		}
	}

	return Option{}, false
}

func nodeExists(graph Graph, id string) bool {
	_, ok := findNode(graph, id)
	return ok
}

// Continue resumes a flow that was paused by a delay node.
func Continue(graph Graph, state State) (State, []Effect, error) {
	if state.PendingNodeID == "" {
		return state, nil, ErrNotPending
	}

	next := state.PendingNodeID
	state.PendingNodeID = ""
	state.WakeAt = nil

	if state.Variables == nil {
		state.Variables = map[string]any{}
	}
	if state.Status == StatusClosed {
		return state, nil, nil
	}
	state.Status = StatusBot

	effects, err := run(graph, &state, next)
	return state, effects, err
}

func run(graph Graph, state *State, startID string) ([]Effect, error) {
	effects := make([]Effect, 0, 4)
	current := startID

	for step := 0; step < maxSteps; step++ {
		state.Steps++
		if state.Steps > maxConversationSteps {
			return effects, ErrTooManySteps
		}

		if current == "" {
			state.Status = StatusClosed
			state.Waiting = nil
			return effects, nil
		}

		node, ok := findNode(graph, current)
		if !ok {
			return effects, fmt.Errorf("%w: %s", ErrInvalidNode, current)
		}

		state.CurrentNodeID = current

		switch node.Type {
		case NodeTrigger:
			current = nextNode(graph, current, "")

		case NodeMessage:
			effects = append(effects, Effect{Type: "send_message", Text: textConfig(node, "text")})
			current = nextNode(graph, current, "")

		case NodeCondition:
			branch := "false"
			if evalCondition(node.Config, state.Variables) {
				branch = "true"
			}
			current = nextNode(graph, current, branch)

		case NodeCollectInput:
			variable := textConfig(node, "variable")
			inputType := textConfig(node, "inputType")
			if inputType == "" {
				inputType = "text"
			}

			fields := fieldsConfig(node.Config)
			if len(fields) > 0 {
				// Formulário: vários campos em uma única tela.
				inputType = "form"
				if variable == "" && len(fields) == 1 {
					variable = fields[0].Name
				}
			}

			if prompt := textConfig(node, "prompt"); prompt != "" {
				effects = append(effects, Effect{Type: "send_message", Text: prompt})
			}

			state.Waiting = &Wait{
				NodeID:         node.ID,
				Variable:       variable,
				InputType:      inputType,
				Options:        optionsConfig(node.Config),
				Fields:         fields,
				FallbackNodeID: textConfig(node, "fallbackNodeId"),
			}
			state.Status = StatusWaiting
			return effects, nil

		case NodeLLM:
			// O indicador de digitação sai antes da chamada: o visitante vê
			// atividade enquanto o modelo pensa, em vez de uma tela parada.
			effects = append(effects, Effect{Type: "typing"})
			effects = append(effects, Effect{
				Type:     "request_llm",
				Prompt:   textConfig(node, "prompt"),
				Variable: textConfig(node, "variable"),
				Text:     textConfig(node, "fallback"),
			})
			current = nextNode(graph, current, "")

		case NodeHandoff:
			effects = append(effects, Effect{Type: "handoff", Reason: textConfig(node, "reason")})
			state.Status = StatusQueued
			state.Waiting = nil
			return effects, nil

		case NodeWebhook:
			effects = append(effects, Effect{Type: "call_webhook", URL: textConfig(node, "url")})
			current = nextNode(graph, current, "")

		case NodeTag:
			effects = append(effects, Effect{Type: "tag", Tags: stringSlice(node.Config["tags"])})
			current = nextNode(graph, current, "")

		case NodeAssign:
			effects = append(effects, Effect{
				Type:    "assign",
				Team:    textConfig(node, "team"),
				AgentID: textConfig(node, "agentId"),
			})
			current = nextNode(graph, current, "")

		case NodeDelay:
			ms := intConfig(node.Config, "ms")
			if ms < 0 {
				ms = 0
			}

			// O atraso agora é visível: o widget mostra "digitando…" enquanto
			// o worker ainda não retomou o fluxo.
			if ms > 0 {
				effects = append(effects, Effect{Type: "typing", MS: ms})
			}
			effects = append(effects, Effect{Type: "delay", MS: ms})
			next := nextNode(graph, current, "")

			if ms > 0 && next != "" {
				wakeAt := time.Now().UTC().Add(time.Duration(ms) * time.Millisecond)
				state.PendingNodeID = next
				state.WakeAt = &wakeAt
				return effects, nil
			}

			current = next

		case NodeEnd:
			state.Status = StatusClosed
			state.Waiting = nil
			return effects, nil

		default:
			return effects, fmt.Errorf("%w: tipo desconhecido %q em %s", ErrInvalidNode, node.Type, node.ID)
		}

		if current == "" {
			state.Status = StatusClosed
			state.Waiting = nil
			return effects, nil
		}
	}

	// Sem ErrTooManySteps o chamador trataria um laço infinito como falha
	// interna (500) em vez de encerrar a conversa com um 400 explicativo.
	return effects, fmt.Errorf("%w: %d passos em uma única execução", ErrTooManySteps, maxSteps)
}

func findNode(graph Graph, id string) (Node, bool) {
	for _, node := range graph.Nodes {
		if node.ID == id {
			return node, true
		}
	}
	return Node{}, false
}

func nextNode(graph Graph, source, handle string) string {
	for _, edge := range graph.Edges {
		if edge.Source != source {
			continue
		}
		if handle == "" {
			return edge.Target
		}
		if edge.SourceHandle == handle {
			return edge.Target
		}
	}
	return ""
}

func evalCondition(config map[string]any, variables map[string]any) bool {
	variable := textConfigValue(config["variable"])
	operator := textConfigValue(config["operator"])
	if operator == "" {
		operator = "eq"
	}

	actual, exists := variables[variable]
	actualText := valueToString(actual)
	expectedText := valueToString(config["value"])

	switch operator {
	case "eq":
		return actualText == expectedText
	case "neq":
		return actualText != expectedText
	case "contains":
		return strings.Contains(strings.ToLower(actualText), strings.ToLower(expectedText))
	case "exists":
		return exists && actualText != ""
	case "gt":
		return compareNumbers(actualText, expectedText) > 0
	case "lt":
		return compareNumbers(actualText, expectedText) < 0
	default:
		return false
	}
}

func compareNumbers(actual, expected string) int {
	a, errA := strconv.ParseFloat(strings.TrimSpace(actual), 64)
	b, errB := strconv.ParseFloat(strings.TrimSpace(expected), 64)
	if errA != nil || errB != nil {
		return 0
	}
	switch {
	case a > b:
		return 1
	case a < b:
		return -1
	default:
		return 0
	}
}

func textConfig(node Node, key string) string {
	return textConfigValue(node.Config[key])
}

func textConfigValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

func valueToString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	default:
		return fmt.Sprint(typed)
	}
}

func intConfig(config map[string]any, key string) int {
	switch typed := config[key].(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case string:
		value, err := strconv.Atoi(typed)
		if err != nil {
			return 0
		}
		return value
	default:
		return 0
	}
}

func boolConfig(value any) bool {
	if typed, ok := value.(bool); ok {
		return typed
	}
	return false
}

func stringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			result = append(result, valueToString(item))
		}
		return result
	case string:
		if typed == "" {
			return nil
		}
		return []string{typed}
	default:
		return nil
	}
}

func fieldsConfig(config map[string]any) []Field {
	raw, ok := config["fields"]
	if !ok {
		return nil
	}

	var items []any
	switch typed := raw.(type) {
	case []Field:
		converted := make([]Field, 0, len(typed))
		for _, field := range typed {
			if field.Name == "" {
				continue
			}
			converted = append(converted, field)
		}
		return truncateFields(converted)
	case []map[string]any:
		items = make([]any, 0, len(typed))
		for _, field := range typed {
			items = append(items, field)
		}
	case []any:
		items = typed
	default:
		return nil
	}

	fields := make([]Field, 0, len(items))
	for _, item := range items {
		field, ok := item.(map[string]any)
		if !ok {
			continue
		}

		name := textConfigValue(field["name"])
		if name == "" {
			continue
		}

		fields = append(fields, Field{
			Name:     name,
			Label:    textConfigValue(field["label"]),
			Type:     textConfigValue(field["type"]),
			Required: boolConfig(field["required"]),
			Options:  optionsFromAny(field["options"]),
		})
	}

	return truncateFields(fields)
}

func truncateFields(fields []Field) []Field {
	if len(fields) > maxFormFields {
		return fields[:maxFormFields]
	}
	return fields
}

func optionsConfig(config map[string]any) []Option {
	raw, ok := config["options"]
	if !ok {
		return nil
	}

	switch typed := raw.(type) {
	case []Option:
		return typed
	case []string:
		options := make([]Option, 0, len(typed))
		for _, item := range typed {
			options = append(options, Option{Label: item, Value: item})
		}
		return options
	case []any:
		options := make([]Option, 0, len(typed))
		for _, item := range typed {
			switch option := item.(type) {
			case string:
				options = append(options, Option{Label: option, Value: option})
			case map[string]any:
				label := textConfigValue(option["label"])
				value := textConfigValue(option["value"])
				if value == "" {
					value = label
				}
				options = append(options, Option{
					Label: label,
					Value: value,
					Next:  textConfigValue(option["nextNodeId"]),
				})
			}
		}
		return options
	default:
		return nil
	}
}

// optionsFromAny reads options from a loosely typed map so both the saved JSON
// graph and the in-memory samples work.
func optionsFromAny(value any) []Option {
	switch typed := value.(type) {
	case []Option:
		return typed
	case []string:
		options := make([]Option, 0, len(typed))
		for _, item := range typed {
			options = append(options, Option{Label: item, Value: item})
		}
		return options
	case []any:
		shim := map[string]any{"options": typed}
		return optionsConfig(shim)
	default:
		return nil
	}
}
