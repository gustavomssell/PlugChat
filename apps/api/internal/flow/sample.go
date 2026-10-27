package flow

import "fmt"

func Starter(id, name string) Graph {
	return Graph{
		ID:          id,
		Name:        name,
		Version:     1,
		EntryNodeID: "start",
		Nodes: []Node{
			{ID: "start", Type: NodeTrigger, Label: "Início", Position: Point{X: 0, Y: 0}},
			{
				ID:       "greeting",
				Type:     NodeMessage,
				Label:    "Saudação",
				Position: Point{X: 0, Y: 160},
				Config:   map[string]any{"text": "Olá! Como podemos ajudar você hoje?"},
			},
			{ID: "end", Type: NodeEnd, Label: "Encerrar", Position: Point{X: 0, Y: 320}},
		},
		Edges: []Edge{
			{ID: "e1", Source: "start", Target: "greeting"},
			{ID: "e2", Source: "greeting", Target: "end"},
		},
	}
}

func Validate(graph Graph) error {
	if len(graph.Nodes) == 0 {
		return ErrEmptyGraph
	}

	validTypes := map[NodeType]struct{}{
		NodeTrigger:      {},
		NodeMessage:      {},
		NodeCondition:    {},
		NodeCollectInput: {},
		NodeLLM:          {},
		NodeHandoff:      {},
		NodeWebhook:      {},
		NodeTag:          {},
		NodeAssign:       {},
		NodeDelay:        {},
		NodeEnd:          {},
	}

	ids := make(map[string]struct{}, len(graph.Nodes))
	for _, node := range graph.Nodes {
		if node.ID == "" {
			return ErrInvalidNode
		}
		if _, exists := ids[node.ID]; exists {
			return fmt.Errorf("%w: id duplicado %q", ErrInvalidNode, node.ID)
		}
		if _, ok := validTypes[node.Type]; !ok {
			return fmt.Errorf("%w: tipo desconhecido %q em %s", ErrInvalidNode, node.Type, node.ID)
		}
		ids[node.ID] = struct{}{}
	}

	if graph.EntryNodeID != "" {
		if _, ok := ids[graph.EntryNodeID]; !ok {
			return fmt.Errorf("%w: entrada %q não encontrada", ErrInvalidNode, graph.EntryNodeID)
		}
	}

	for _, edge := range graph.Edges {
		if edge.ID == "" {
			return fmt.Errorf("%w: aresta sem id", ErrInvalidNode)
		}
		if _, ok := ids[edge.Source]; !ok {
			return fmt.Errorf("%w: origem %q não encontrada", ErrInvalidNode, edge.Source)
		}
		if _, ok := ids[edge.Target]; !ok {
			return fmt.Errorf("%w: destino %q não encontrado", ErrInvalidNode, edge.Target)
		}
		if edge.SourceHandle != "" && edge.SourceHandle != "true" && edge.SourceHandle != "false" {
			return fmt.Errorf("%w: saída %q inválida", ErrInvalidNode, edge.SourceHandle)
		}
	}

	return nil
}

func Sample() Graph {
	return Graph{
		ID:          "flow_support",
		Name:        "Triagem de atendimento",
		Version:     1,
		EntryNodeID: "start",
		Nodes: []Node{
			{ID: "start", Type: NodeTrigger, Label: "Início", Position: Point{X: 0, Y: 0}},
			{
				ID:       "greeting",
				Type:     NodeMessage,
				Label:    "Saudação",
				Position: Point{X: 0, Y: 140},
				Config:   map[string]any{"text": "Olá! Como podemos ajudar você hoje?"},
			},
			{
				ID:       "ask_name",
				Type:     NodeCollectInput,
				Label:    "Nome do visitante",
				Position: Point{X: 0, Y: 280},
				Config: map[string]any{
					"variable":  "name",
					"prompt":    "Antes de começar, qual é o seu nome?",
					"inputType": "text",
				},
			},
			{
				ID:       "ask_topic",
				Type:     NodeCollectInput,
				Label:    "Assunto",
				Position: Point{X: 0, Y: 420},
				Config: map[string]any{
					"variable":  "topic",
					"prompt":    "Sobre o que você quer falar?",
					"inputType": "choice",
					// "Suporte" aponta direto para a fila: um clique, zero
					// passos extras.
					"options": []any{
						map[string]any{"label": "Comercial", "value": "Comercial"},
						map[string]any{"label": "Suporte", "value": "Suporte", "nextNodeId": "route_support"},
						map[string]any{"label": "Financeiro", "value": "Financeiro"},
					},
				},
			},
			{
				ID:       "ask_contact",
				Type:     NodeCollectInput,
				Label:    "Dados de contato",
				Position: Point{X: 0, Y: 560},
				Config: map[string]any{
					"prompt": "Para agilizar, me passa seus dados:",
					"fields": []any{
						map[string]any{"name": "email", "label": "E-mail", "type": "email", "required": true},
						map[string]any{"name": "phone", "label": "Telefone", "type": "phone"},
					},
				},
			},
			{
				ID:       "check_support",
				Type:     NodeCondition,
				Label:    "É suporte?",
				Position: Point{X: 0, Y: 560},
				Config: map[string]any{
					"variable": "topic",
					"operator": "eq",
					"value":    "Suporte",
				},
			},
			{
				ID:       "route_support",
				Type:     NodeHandoff,
				Label:    "Fila de suporte",
				Position: Point{X: -220, Y: 700},
				Config:   map[string]any{"reason": "Cliente pediu suporte"},
			},
			{
				ID:       "route_sales",
				Type:     NodeMessage,
				Label:    "Mensagem comercial",
				Position: Point{X: 220, Y: 700},
				Config: map[string]any{
					"text": "Certo. Já registrei seu contato e um especialista vai te responder por aqui.",
				},
			},
			{ID: "finish", Type: NodeEnd, Label: "Encerrar", Position: Point{X: 220, Y: 840}},
		},
		Edges: []Edge{
			{ID: "e1", Source: "start", Target: "greeting"},
			{ID: "e2", Source: "greeting", Target: "ask_name"},
			{ID: "e3", Source: "ask_name", Target: "ask_topic"},
			{ID: "e4", Source: "ask_topic", Target: "check_support"},
			{ID: "e5", Source: "check_support", Target: "route_support", SourceHandle: "true", Label: "Sim"},
			{ID: "e6", Source: "check_support", Target: "ask_contact", SourceHandle: "false", Label: "Não"},
			{ID: "e7", Source: "ask_contact", Target: "route_sales"},
			{ID: "e8", Source: "route_sales", Target: "finish"},
		},
	}
}
