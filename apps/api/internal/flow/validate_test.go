package flow

import "testing"

func TestValidateSample(t *testing.T) {
	if err := Validate(Sample()); err != nil {
		t.Fatalf("Validate(Sample) = %v", err)
	}
	if err := Validate(Starter("flow", "Fluxo")); err != nil {
		t.Fatalf("Validate(Starter) = %v", err)
	}
}

func TestValidateRejectsUnknownNodeType(t *testing.T) {
	graph := Starter("flow", "Fluxo")
	graph.Nodes[1].Type = NodeType("invalid")

	if err := Validate(graph); err == nil {
		t.Fatal("esperava erro para tipo de node desconhecido")
	}
}

func TestValidateRejectsInvalidEdge(t *testing.T) {
	graph := Starter("flow", "Fluxo")
	graph.Edges = append(graph.Edges, Edge{ID: "e3", Source: "greeting", Target: "missing"})

	if err := Validate(graph); err == nil {
		t.Fatal("esperava erro para destino inexistente")
	}

	graph = Starter("flow", "Fluxo")
	graph.Edges[1].SourceHandle = "maybe"

	if err := Validate(graph); err == nil {
		t.Fatal("esperava erro para saída inválida")
	}
}
