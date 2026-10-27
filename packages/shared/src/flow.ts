export const NODE_TYPES = [
  "trigger",
  "message",
  "condition",
  "collect_input",
  "llm",
  "handoff",
  "webhook",
  "tag",
  "assign",
  "delay",
  "end",
] as const;

export type NodeType = (typeof NODE_TYPES)[number];

export interface Point {
  x: number;
  y: number;
}

export interface FlowNode {
  id: string;
  type: NodeType;
  label?: string;
  position: Point;
  config?: Record<string, unknown>;
}

export interface FlowEdge {
  id: string;
  source: string;
  target: string;
  sourceHandle?: string;
  label?: string;
}

export interface FlowGraph {
  id: string;
  name: string;
  version: number;
  entryNodeId?: string;
  nodes: FlowNode[];
  edges: FlowEdge[];
}

export interface FlowSummary {
  id: string;
  name: string;
  version: number;
  nodes: number;
  updatedAt: string;
}

export type EngineEffect =
  | { type: "send_message"; text: string }
  | { type: "request_llm"; prompt: string; variable?: string }
  | { type: "handoff"; reason?: string }
  | { type: "assign"; team?: string; agentId?: string }
  | { type: "tag"; tags: string[] }
  | { type: "call_webhook"; url: string }
  | { type: "delay"; ms: number }
  /** Estado de interface: o widget mostra "digitando…" sem gravar mensagem. */
  | { type: "typing"; ms: number };

export const NODE_LABELS: Record<NodeType, string> = {
  trigger: "Início",
  message: "Mensagem",
  condition: "Condição",
  collect_input: "Coletar resposta",
  llm: "IA",
  handoff: "Transferir para humano",
  webhook: "Webhook",
  tag: "Etiquetar",
  assign: "Atribuir",
  delay: "Aguardar",
  end: "Encerrar",
};
