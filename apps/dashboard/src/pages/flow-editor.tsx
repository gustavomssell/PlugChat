import { useCallback, useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useParams } from "@tanstack/react-router";
import {
  Background,
  Controls,
  ReactFlow,
  addEdge,
  useEdgesState,
  useNodesState,
  type Connection,
  type Edge,
  type EdgeChange,
  type Node,
  type NodeChange,
  type NodeTypes,
} from "@xyflow/react";
import { ArrowLeft, Check, Loader2, Plus, Save, Trash2, Workflow } from "lucide-react";
import {
  NODE_LABELS,
  NODE_TYPES,
  type Agent,
  type FlowEdge,
  type FlowGraph,
  type FlowNode,
  type NodeType,
  type WaitField,
  type WaitOption,
} from "@chatfree/shared";
import { api } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Skeleton } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/toast";
import { EmptyState } from "@/components/empty-state";
import { FlowNodeCard, type FlowNodeData } from "@/components/flow-node-card";

const nodeTypes = { flow: FlowNodeCard } satisfies NodeTypes;

function defaultConfig(type: NodeType): Record<string, unknown> {
  switch (type) {
    case "message":
      return { text: "Nova mensagem" };
    case "condition":
      return { variable: "topic", operator: "eq", value: "" };
    case "collect_input":
      return { variable: "answer", prompt: "Digite sua resposta", inputType: "text" };
    case "llm":
      return { prompt: "Responda ao visitante", variable: "answer" };
    case "handoff":
      return { reason: "Visitante pediu atendimento" };
    case "webhook":
      return { url: "https://" };
    case "tag":
      return { tags: [] };
    case "assign":
      return { agentId: "" };
    case "delay":
      return { ms: 1000 };
    default:
      return {};
  }
}

function previewFor(node: FlowNode) {
  const config = node.config ?? {};
  const text = config.text ?? config.prompt ?? config.reason ?? config.variable;
  return typeof text === "string" ? text : undefined;
}

function toFlowNode(node: FlowNode): Node<FlowNodeData> {
  return {
    id: node.id,
    type: "flow",
    position: node.position,
    data: {
      label: node.label ?? NODE_LABELS[node.type],
      type: node.type,
      preview: previewFor(node),
      config: node.config ?? {},
    },
  };
}

function toFlowEdge(edge: FlowEdge): Edge {
  return {
    id: edge.id,
    source: edge.source,
    target: edge.target,
    sourceHandle: edge.sourceHandle,
    label: edge.label,
    type: "smoothstep",
  };
}

export function FlowEditorPage() {
  const { flowId } = useParams({ from: "/_app/flows/$flowId" });
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const [savedAt, setSavedAt] = useState<string | null>(null);
  const [dirty, setDirty] = useState(false);
  const [newNodeType, setNewNodeType] = useState<NodeType>("message");
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);

  const flowQuery = useQuery({
    queryKey: ["flow", flowId],
    queryFn: () => api.getFlow(flowId),
  });

  const agents = useQuery({ queryKey: ["agents"], queryFn: api.listAgents });

  const [nodes, setNodes, onNodesChange] = useNodesState<Node<FlowNodeData>>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);

  useEffect(() => {
    if (!flowQuery.data) {
      return;
    }
    setNodes(flowQuery.data.nodes.map(toFlowNode));
    setEdges(flowQuery.data.edges.map(toFlowEdge));
    setDirty(false);
  }, [flowQuery.data, setNodes, setEdges]);

  useEffect(() => {
    if (!dirty) return;
    const handleBeforeUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", handleBeforeUnload);
    return () => window.removeEventListener("beforeunload", handleBeforeUnload);
  }, [dirty]);

  const save = useMutation({
    mutationFn: async () => {
      const current = flowQuery.data;
      if (!current) {
        throw new Error("Fluxo não carregado");
      }

      const graph: FlowGraph = {
        ...current,
        nodes: nodes.map((node) => ({
          id: node.id,
          type: node.data.type,
          label: node.data.label,
          position: { x: node.position.x, y: node.position.y },
          config: node.data.config ?? {},
        })),
        edges: edges.map((edge) => ({
          id: edge.id,
          source: edge.source,
          target: edge.target,
          sourceHandle: edge.sourceHandle ?? undefined,
          label: typeof edge.label === "string" ? edge.label : undefined,
        })),
      };

      return api.saveFlow(graph);
    },
    onSuccess: (graph) => {
      void queryClient.invalidateQueries({ queryKey: ["flow", flowId] });
      void queryClient.invalidateQueries({ queryKey: ["flows"] });
      setSavedAt(new Date().toLocaleTimeString("pt-BR"));
      setDirty(false);
      setNodes(graph.nodes.map(toFlowNode));
      setEdges(graph.edges.map(toFlowEdge));
      toast({ tone: "success", title: "Fluxo salvo", description: "As alterações já valem para novas conversas." });
    },
    onError: (error) => {
      toast({
        tone: "error",
        title: "Falha ao salvar fluxo",
        description: error instanceof Error ? error.message : undefined,
      });
    },
  });

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "s") {
        event.preventDefault();
        if (!save.isPending && flowQuery.data) {
          save.mutate();
        }
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [flowQuery.data, save]);

  const handleNodesChange = useCallback(
    (changes: NodeChange<Node<FlowNodeData>>[]) => {
      if (changes.some((change) => change.type !== "select" && change.type !== "dimensions")) {
        setDirty(true);
      }
      onNodesChange(changes);
    },
    [onNodesChange],
  );

  const handleEdgesChange = useCallback(
    (changes: EdgeChange<Edge>[]) => {
      if (changes.some((change) => change.type !== "select")) {
        setDirty(true);
      }
      onEdgesChange(changes);
    },
    [onEdgesChange],
  );

  const onConnect = useCallback(
    (connection: Connection) => {
      setDirty(true);
      setEdges((current) =>
        addEdge(
          {
            ...connection,
            id: crypto.randomUUID(),
            type: "smoothstep",
            label:
              connection.sourceHandle === "true"
                ? "Sim"
                : connection.sourceHandle === "false"
                  ? "Não"
                  : undefined,
          },
          current,
        ),
      );
    },
    [setEdges],
  );

  const addNode = () => {
    setDirty(true);
    const id = crypto.randomUUID();
    const config = defaultConfig(newNodeType);
    const preview =
      typeof config.text === "string"
        ? config.text
        : typeof config.prompt === "string"
          ? config.prompt
          : undefined;

    setNodes((current) => [
      ...current,
      {
        id,
        type: "flow",
        position: { x: 320 + current.length * 24, y: 120 + current.length * 24 },
        data: {
          label: NODE_LABELS[newNodeType],
          type: newNodeType,
          preview,
          config,
        },
      },
    ]);
    setSelectedNodeId(id);
  };

  const selectedNode = nodes.find((node) => node.id === selectedNodeId) ?? null;

  const updateSelected = (patch: Partial<FlowNodeData>) => {
    if (!selectedNodeId) return;
    setDirty(true);
    setNodes((current) =>
      current.map((node) =>
        node.id === selectedNodeId ? { ...node, data: { ...node.data, ...patch } } : node,
      ),
    );
  };

  // updateConfig aplica os patches de forma funcional. A versão anterior
  // reconstruía `config` a partir de selectedNode.data.config, que é uma
  // captura do render atual: chamadas em sequência no mesmo evento (trocar o
  // inputType e redefinir options/fields) faziam cada uma descartar a
  // mudança da anterior, gravando um config corrompido.
  const updateConfig = (patch: Record<string, unknown>) => {
    if (!selectedNodeId) return;
    setDirty(true);
    setNodes((current) =>
      current.map((node) => {
        if (node.id !== selectedNodeId) return node;

        const config = { ...(node.data.config ?? {}), ...patch };
        const preview =
          typeof config.text === "string"
            ? config.text
            : typeof config.prompt === "string"
              ? config.prompt
              : typeof config.reason === "string"
                ? config.reason
                : nodePreview(node.data.type, config, agents.data);

        return { ...node, data: { ...node.data, config, preview } };
      }),
    );
  };

  const deleteSelected = () => {
    if (!selectedNodeId) return;
    setDirty(true);
    setNodes((current) => current.filter((node) => node.id !== selectedNodeId));
    setEdges((current) =>
      current.filter((edge) => edge.source !== selectedNodeId && edge.target !== selectedNodeId),
    );
    setSelectedNodeId(null);
  };

  if (flowQuery.isLoading) {
    return (
      <div className="mx-auto flex w-full max-w-6xl flex-col gap-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <Skeleton className="size-9 rounded-lg" />
            <div className="space-y-2">
              <Skeleton className="h-5 w-48" />
              <Skeleton className="h-3 w-64" />
            </div>
          </div>
          <Skeleton className="h-9 w-56" />
        </div>
        <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_300px]">
          <Skeleton className="h-[70svh] rounded-xl" />
          <Skeleton className="h-64 rounded-xl" />
        </div>
      </div>
    );
  }

  if (flowQuery.isError || !flowQuery.data) {
    return (
      <div className="mx-auto w-full max-w-6xl">
        <EmptyState
          icon={Workflow}
          title="Fluxo não encontrado"
          description="O fluxo pode ter sido removido ou você não tem acesso a ele neste workspace."
          action={
            <Button asChild variant="outline" size="sm">
              <Link to="/flows">
                <ArrowLeft data-icon="inline-start" />
                Voltar para fluxos
              </Link>
            </Button>
          }
        />
      </div>
    );
  }

  const flow = flowQuery.data;

  return (
    <div className="mx-auto flex w-full max-w-6xl flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <Button asChild variant="ghost" size="icon">
            <Link to="/flows" aria-label="Voltar para fluxos">
              <ArrowLeft />
            </Link>
          </Button>
          <div>
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-xl font-semibold tracking-tight">{flow.name}</h1>
              <Badge variant="outline">v{flow.version}</Badge>
              {dirty ? (
                <Badge variant="outline" className="border-warning/30 bg-warning/10 text-warning">
                  Alterações não salvas
                </Badge>
              ) : savedAt ? (
                <span className="flex items-center gap-1 text-xs text-success">
                  <Check className="size-3.5" aria-hidden="true" />
                  salvo às {savedAt}
                </span>
              ) : null}
            </div>
            <p className="mt-0.5 text-xs text-muted-foreground">
              {nodes.length} {nodes.length === 1 ? "node" : "nodes"}
              <span className="mx-1.5 text-muted-foreground/40">·</span>
              {edges.length} {edges.length === 1 ? "conexão" : "conexões"}
              <span className="mx-1.5 text-muted-foreground/40">·</span>
              arraste, conecte e salve para publicar
            </p>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Select value={newNodeType} onValueChange={(value) => setNewNodeType(value as NodeType)}>
            <SelectTrigger className="h-8 w-40 text-xs" aria-label="Tipo de node">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {NODE_TYPES.map((type) => (
                <SelectItem key={type} value={type}>
                  {NODE_LABELS[type]}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button variant="outline" size="sm" onClick={addNode}>
            <Plus data-icon="inline-start" />
            Adicionar
          </Button>
          <Button
            size="sm"
            onClick={() => save.mutate()}
            disabled={save.isPending || nodes.length === 0}
            title="Salvar fluxo (Ctrl+S)"
          >
            {save.isPending ? (
              <Loader2 data-icon="inline-start" className="animate-spin" />
            ) : (
              <Save data-icon="inline-start" />
            )}
            {save.isPending ? "Salvando..." : "Salvar"}
          </Button>
        </div>
      </div>

      {save.isError ? (
        <p className="text-sm text-destructive">
          {save.error instanceof Error ? save.error.message : "Não foi possível salvar o fluxo."}
        </p>
      ) : null}

      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_300px]">
        <div className="relative h-[70svh] overflow-hidden rounded-xl border border-border/70 bg-muted/20">
          <ReactFlow
            nodes={nodes}
            edges={edges}
            nodeTypes={nodeTypes}
            onNodesChange={handleNodesChange}
            onEdgesChange={handleEdgesChange}
            onConnect={onConnect}
            onNodeClick={(_, node) => setSelectedNodeId(node.id)}
            onPaneClick={() => setSelectedNodeId(null)}
            fitView
            minZoom={0.3}
            maxZoom={1.5}
            defaultEdgeOptions={{ type: "smoothstep" }}
            proOptions={{ hideAttribution: true }}
          >
            <Background gap={22} size={1.5} color="var(--border)" />
            <Controls showInteractive={false} />
          </ReactFlow>
          {nodes.length === 0 ? (
            <div className="pointer-events-none absolute inset-0 flex items-center justify-center p-6">
              <EmptyState
                icon={Workflow}
                title="Canvas vazio"
                description="Escolha um tipo de node acima e clique em Adicionar para começar."
                className="pointer-events-auto bg-card/90 shadow-sm backdrop-blur"
              />
            </div>
          ) : null}
        </div>

        <Card className="h-fit lg:sticky lg:top-20">
          <CardHeader className="flex flex-row items-start justify-between gap-2">
            <div>
              <CardTitle className="text-sm">
                {selectedNode ? NODE_LABELS[selectedNode.data.type] : "Inspector"}
              </CardTitle>
              <CardDescription>
                {selectedNode ? "Edite o comportamento deste node." : "Selecione um node no canvas."}
              </CardDescription>
            </div>
            {selectedNode ? (
              <Button
                variant="ghost"
                size="icon"
                className="size-7 text-muted-foreground hover:text-destructive"
                onClick={deleteSelected}
                aria-label="Remover node"
              >
                <Trash2 className="size-3.5" />
              </Button>
            ) : null}
          </CardHeader>
          <CardContent>
            {selectedNode ? (
              <div className="flex flex-col gap-4">
                <Field>
                  <FieldLabel htmlFor="node-label">Rótulo</FieldLabel>
                  <Input
                    id="node-label"
                    value={selectedNode.data.label}
                    onChange={(event) => updateSelected({ label: event.target.value })}
                  />
                </Field>

                <NodeConfigFields
                  type={selectedNode.data.type}
                  config={selectedNode.data.config ?? {}}
                  agents={agents.data}
                  nodes={nodes}
                  currentId={selectedNode.id}
                  onChange={updateConfig}
                />
              </div>
            ) : (
              <p className="text-xs text-muted-foreground">
                Clique em um node para editar mensagens, condições e transferências.
              </p>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function nodePreview(type: NodeType, config: Record<string, unknown>, agents?: Agent[]) {
  if (type === "tag" && Array.isArray(config.tags)) {
    return (config.tags as string[]).join(", ");
  }
  if (type === "webhook" && typeof config.url === "string") return config.url;
  if (type === "assign") {
    const agentId = typeof config.agentId === "string" ? config.agentId : "";
    return agents?.find((agent) => agent.userId === agentId)?.name ?? agentId;
  }
  if (type === "delay" && typeof config.ms === "number") return `${config.ms} ms`;
  return undefined;
}

function stringValue(config: Record<string, unknown>, key: string, fallback = "") {
  const value = config[key];
  return typeof value === "string" ? value : fallback;
}

type OptionDraft = { label: string; value: string; nextNodeId: string };

function toOptionDrafts(options: (string | WaitOption)[] | undefined): OptionDraft[] {
  if (!Array.isArray(options)) {
    return [{ label: "", value: "", nextNodeId: "" }];
  }

  const drafts = options.map((option) =>
    typeof option === "string"
      ? { label: option, value: option, nextNodeId: "" }
      : {
          label: option.label ?? "",
          value: option.value ?? option.label ?? "",
          nextNodeId: option.nextNodeId ?? "",
        },
  );

  return drafts.length > 0 ? drafts : [{ label: "", value: "", nextNodeId: "" }];
}

function OptionsEditor({
  options,
  nodes,
  currentId,
  onChange,
}: {
  options: (string | WaitOption)[] | undefined;
  nodes: Node<FlowNodeData>[];
  currentId: string | undefined;
  onChange: (options: OptionDraft[]) => void;
}) {
  const drafts = toOptionDrafts(options);

  const update = (index: number, patch: Partial<OptionDraft>) => {
    const next = drafts.map((draft, position) =>
      position === index ? { ...draft, ...patch } : draft,
    );
    onChange(next);
  };

  return (
    <Field>
      <FieldLabel>Opções</FieldLabel>
      <FieldDescription>
        Cada opção pode pular direto para um nó: um clique e a conversa segue
        sem passar pelos passos intermediários.
      </FieldDescription>
      <div className="space-y-2">
        {drafts.map((draft, index) => (
          <div key={index} className="flex items-center gap-2">
            <Input
              aria-label={`Rótulo da opção ${index + 1}`}
              placeholder="Rótulo"
              value={draft.label}
              onChange={(event) => update(index, { label: event.target.value })}
            />
            <Select
              value={draft.nextNodeId || "none"}
              onValueChange={(value) =>
                update(index, { nextNodeId: value === "none" ? "" : value })
              }
            >
              <SelectTrigger aria-label={`Destino da opção ${index + 1}`} className="w-40">
                <SelectValue placeholder="Seguir fluxo" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="none">Seguir fluxo</SelectItem>
                {nodes
                  .filter((node) => node.id !== currentId)
                  .map((node) => (
                    <SelectItem key={node.id} value={node.id}>
                      {node.data.label}
                    </SelectItem>
                  ))}
              </SelectContent>
            </Select>
            {drafts.length > 1 ? (
              <Button
                type="button"
                variant="ghost"
                size="icon"
                aria-label={`Remover opção ${index + 1}`}
                onClick={() =>
                  onChange(drafts.filter((_, position) => position !== index))
                }
              >
                <Trash2 className="size-4" />
              </Button>
            ) : null}
          </div>
        ))}
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() =>
            onChange([...drafts, { label: "", value: "", nextNodeId: "" }])
          }
        >
          <Plus className="size-4" />
          Adicionar opção
        </Button>
      </div>
    </Field>
  );
}

type FieldDraft = {
  name: string;
  label: string;
  type: string;
  required: boolean;
};

function toFieldDrafts(fields: WaitField[] | undefined): FieldDraft[] {
  if (!Array.isArray(fields) || fields.length === 0) {
    return [{ name: "", label: "", type: "text", required: false }];
  }

  return fields.map((field) => ({
    name: field.name ?? "",
    label: field.label ?? "",
    type: field.type ?? "text",
    required: field.required ?? false,
  }));
}

function FieldsEditor({
  fields,
  onChange,
}: {
  fields: WaitField[] | undefined;
  onChange: (fields: FieldDraft[]) => void;
}) {
  const drafts = toFieldDrafts(fields);

  const update = (index: number, patch: Partial<FieldDraft>) => {
    onChange(drafts.map((draft, position) => (position === index ? { ...draft, ...patch } : draft)));
  };

  return (
    <Field>
      <FieldLabel>Campos do formulário</FieldLabel>
      <FieldDescription>
        O visitante preenche tudo em uma única tela — uma interação em vez de
        várias.
      </FieldDescription>
      <div className="space-y-2">
        {drafts.map((draft, index) => (
          <div key={index} className="flex items-center gap-2">
            <Input
              aria-label={`Rótulo do campo ${index + 1}`}
              placeholder="Rótulo (ex.: E-mail)"
              value={draft.label}
              onChange={(event) => {
                const label = event.target.value;
                const slug = slugify(label);
                update(index, {
                  label,
                  name: draft.name === "" || slugify(draft.name) === slugify(draft.label) ? slug : draft.name,
                });
              }}
            />
            <Select
              value={draft.type}
              onValueChange={(value) => update(index, { type: value })}
            >
              <SelectTrigger aria-label={`Tipo do campo ${index + 1}`} className="w-32">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="text">Texto</SelectItem>
                <SelectItem value="email">E-mail</SelectItem>
                <SelectItem value="phone">Telefone</SelectItem>
                <SelectItem value="number">Número</SelectItem>
                <SelectItem value="textarea">Texto longo</SelectItem>
                <SelectItem value="choice">Escolha</SelectItem>
              </SelectContent>
            </Select>
            <label className="flex items-center gap-1 text-xs text-muted-foreground">
              <input
                type="checkbox"
                checked={draft.required}
                aria-label={`Campo ${index + 1} obrigatório`}
                onChange={(event) => update(index, { required: event.target.checked })}
              />
              Obrigatório
            </label>
            {drafts.length > 1 ? (
              <Button
                type="button"
                variant="ghost"
                size="icon"
                aria-label={`Remover campo ${index + 1}`}
                onClick={() => onChange(drafts.filter((_, position) => position !== index))}
              >
                <Trash2 className="size-4" />
              </Button>
            ) : null}
          </div>
        ))}
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => onChange([...drafts, { name: "", label: "", type: "text", required: false }])}
        >
          <Plus className="size-4" />
          Adicionar campo
        </Button>
      </div>
    </Field>
  );
}

function slugify(value: string) {
  return value
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "")
    .slice(0, 40);
}

function NodeConfigFields({
  type,
  config,
  agents,
  nodes,
  currentId,
  onChange,
}: {
  type: NodeType;
  config: Record<string, unknown>;
  agents?: Agent[];
  nodes: Node<FlowNodeData>[];
  currentId?: string;
  onChange: (patch: Record<string, unknown>) => void;
}) {
  switch (type) {
    case "message":
      return (
        <Field>
          <FieldLabel htmlFor="node-text">Mensagem</FieldLabel>
          <Textarea
            id="node-text"
            rows={4}
            value={stringValue(config, "text")}
            onChange={(event) => onChange({ text: event.target.value })}
          />
        </Field>
      );
    case "condition":
      return (
        <>
          <Field>
            <FieldLabel htmlFor="node-variable">Variável</FieldLabel>
            <Input
              id="node-variable"
              value={stringValue(config, "variable")}
              onChange={(event) => onChange({ variable: event.target.value })}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="node-operator">Operador</FieldLabel>
            <Select
              value={stringValue(config, "operator", "eq")}
              onValueChange={(value) => onChange({ operator: value })}
            >
              <SelectTrigger id="node-operator" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="eq">Igual a</SelectItem>
                <SelectItem value="neq">Diferente de</SelectItem>
                <SelectItem value="contains">Contém</SelectItem>
                <SelectItem value="exists">Existe</SelectItem>
                <SelectItem value="gt">Maior que</SelectItem>
                <SelectItem value="lt">Menor que</SelectItem>
              </SelectContent>
            </Select>
          </Field>
          <Field>
            <FieldLabel htmlFor="node-value">Valor</FieldLabel>
            <Input
              id="node-value"
              value={stringValue(config, "value")}
              onChange={(event) => onChange({ value: event.target.value })}
            />
          </Field>
        </>
      );
    case "collect_input":
      return (
        <>
          <Field>
            <FieldLabel htmlFor="node-prompt">Pergunta</FieldLabel>
            <Textarea
              id="node-prompt"
              rows={3}
              value={stringValue(config, "prompt")}
              onChange={(event) => onChange({ prompt: event.target.value })}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="node-input-type">Tipo de resposta</FieldLabel>
            <Select
              value={stringValue(config, "inputType", "text")}
              onValueChange={(value) => {
                // Um nó é um ou outro: um único patch mantém o config coerente.
                // Múltiplas chamadas de onChange aqui sobrescreveriam umas às
                // outras, porque cada uma derrubaria a anterior.
                if (value === "form") {
                  onChange({
                    inputType: "form",
                    options: undefined,
                    fields: [{ name: "", label: "", type: "text", required: false }],
                  });
                  return;
                }
                onChange({
                  inputType: value,
                  fields: undefined,
                  options:
                    value === "choice" && !Array.isArray(config.options)
                      ? [{ label: "", value: "", nextNodeId: "" }]
                      : config.options,
                });
              }}
            >
              <SelectTrigger id="node-input-type" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="text">Texto livre</SelectItem>
                <SelectItem value="choice">Escolha uma opção</SelectItem>
                <SelectItem value="form">Formulário em uma tela</SelectItem>
              </SelectContent>
            </Select>
          </Field>

          {stringValue(config, "inputType", "text") === "form" ? (
            <FieldsEditor
              fields={config.fields as WaitField[] | undefined}
              onChange={(fields) => onChange({ fields })}
            />
          ) : (
            <>
              <OptionsEditor
                options={config.options as (string | WaitOption)[] | undefined}
                nodes={nodes}
                currentId={currentId}
                onChange={(options) => onChange({ options })}
              />
              <Field>
                <FieldLabel htmlFor="node-fallback">
                  Se a resposta não casar, ir para
                </FieldLabel>
                <Select
                  value={stringValue(config, "fallbackNodeId")}
                  onValueChange={(value) =>
                    onChange({ fallbackNodeId: value === "none" ? "" : value })
                  }
                >
                  <SelectTrigger id="node-fallback" className="w-full">
                    <SelectValue placeholder="Continuar no fluxo" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="none">Continuar no fluxo</SelectItem>
                    {nodes
                      .filter((node) => node.id !== currentId)
                      .map((node) => (
                        <SelectItem key={node.id} value={node.id}>
                          {node.data.label}
                        </SelectItem>
                      ))}
                  </SelectContent>
                </Select>
                <FieldDescription>
                  Use para reenviar a pergunta ou escalar quando o visitante
                  responde fora das opções.
                </FieldDescription>
              </Field>
              {/* Antes a condição era invertida: o campo só aparecia quando
                  `variable` estava vazio, então sumia ao primeiro caractere
                  digitado. Agora ele é exibido sempre que o tipo aceita texto. */}
              {stringValue(config, "inputType", "text") !== "choice" ? (
                <Field>
                  <FieldLabel htmlFor="node-variable">Guardar resposta em</FieldLabel>
                  <Input
                    id="node-variable"
                    placeholder="ex.: topic"
                    value={stringValue(config, "variable")}
                    onChange={(event) => onChange({ variable: event.target.value })}
                  />
                </Field>
              ) : null}
            </>
          )}
        </>
      );
    case "llm":
      return (
        <>
          <Field>
            <FieldLabel htmlFor="node-prompt">Prompt</FieldLabel>
            <Textarea
              id="node-prompt"
              rows={4}
              value={stringValue(config, "prompt")}
              onChange={(event) => onChange({ prompt: event.target.value })}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="node-variable">Salvar resposta em</FieldLabel>
            <Input
              id="node-variable"
              value={stringValue(config, "variable")}
              onChange={(event) => onChange({ variable: event.target.value })}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="node-fallback">Resposta de reserva</FieldLabel>
            <Textarea
              id="node-fallback"
              rows={3}
              value={stringValue(config, "fallback")}
              onChange={(event) => onChange({ fallback: event.target.value })}
            />
            <FieldDescription>
              Usada quando a IA não estiver configurada ou falhar.
            </FieldDescription>
          </Field>
        </>
      );
    case "handoff":
      return (
        <Field>
          <FieldLabel htmlFor="node-reason">Motivo da transferência</FieldLabel>
          <Textarea
            id="node-reason"
            rows={3}
            value={stringValue(config, "reason")}
            onChange={(event) => onChange({ reason: event.target.value })}
          />
        </Field>
      );
    case "webhook":
      return (
        <Field>
          <FieldLabel htmlFor="node-url">URL do webhook</FieldLabel>
          <Input
            id="node-url"
            value={stringValue(config, "url")}
            onChange={(event) => onChange({ url: event.target.value })}
          />
        </Field>
      );
    case "tag":
      return (
        <Field>
          <FieldLabel htmlFor="node-tags">Tags (separadas por vírgula)</FieldLabel>
          <Input
            id="node-tags"
            value={Array.isArray(config.tags) ? (config.tags as string[]).join(", ") : ""}
            onChange={(event) =>
              onChange({
                tags: event.target.value
                  .split(",")
                  .map((item) => item.trim())
                  .filter(Boolean),
              })
            }
          />
        </Field>
      );
    case "assign":
      return (
        <>
          <Field>
            <FieldLabel htmlFor="node-agent">Atendente</FieldLabel>
            <Select
              value={stringValue(config, "agentId", "_none") || "_none"}
              onValueChange={(value) => onChange({ agentId: value === "_none" ? "" : value })}
            >
              <SelectTrigger id="node-agent" className="w-full">
                <SelectValue placeholder="Selecione um atendente" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="_none">Manter sem atribuição</SelectItem>
                {agents?.map((agent) => (
                  <SelectItem key={agent.userId} value={agent.userId}>
                    {agent.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <FieldDescription>
              A conversa fica atribuída ao atendente escolhido sem interromper o fluxo.
            </FieldDescription>
          </Field>
        </>
      );
    case "delay":
      return (
        <Field>
          <FieldLabel htmlFor="node-ms">Espera (ms)</FieldLabel>
          <Input
            id="node-ms"
            type="number"
            min={0}
            value={typeof config.ms === "number" ? config.ms : 1000}
            onChange={(event) => onChange({ ms: Number(event.target.value) })}
          />
          <FieldDescription>
            O fluxo continua automaticamente depois desse intervalo, mesmo sem mensagem do visitante.
          </FieldDescription>
        </Field>
      );
    default:
      return (
        <p className="text-xs text-muted-foreground">
          Este node não tem configuração adicional.
        </p>
      );
  }
}
