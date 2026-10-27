import type { FlowGraph } from "./flow";

export const sampleFlow: FlowGraph = {
  id: "flow_support",
  name: "Triagem de atendimento",
  version: 1,
  entryNodeId: "start",
  nodes: [
    {
      id: "start",
      type: "trigger",
      label: "Início",
      position: { x: 0, y: 0 },
      config: {},
    },
    {
      id: "greeting",
      type: "message",
      label: "Saudação",
      position: { x: 0, y: 140 },
      config: { text: "Olá! Como podemos ajudar você hoje?" },
    },
    {
      id: "ask_name",
      type: "collect_input",
      label: "Nome do visitante",
      position: { x: 0, y: 280 },
      config: {
        variable: "name",
        prompt: "Antes de começar, qual é o seu nome?",
        inputType: "text",
      },
    },
    {
      id: "ask_topic",
      type: "collect_input",
      label: "Assunto",
      position: { x: 0, y: 420 },
      config: {
        variable: "topic",
        prompt: "Sobre o que você quer falar?",
        inputType: "choice",
        // "Suporte" aponta direto para a fila: um clique, zero passos extras.
        options: [
          { label: "Comercial", value: "Comercial" },
          { label: "Suporte", value: "Suporte", nextNodeId: "route_support" },
          { label: "Financeiro", value: "Financeiro" },
        ],
      },
    },
    {
      id: "ask_contact",
      type: "collect_input",
      label: "Dados de contato",
      position: { x: 0, y: 560 },
      config: {
        prompt: "Para agilizar, me passa seus dados:",
        fields: [
          { name: "email", label: "E-mail", type: "email", required: true },
          { name: "phone", label: "Telefone", type: "phone" },
        ],
      },
    },
    {
      id: "check_support",
      type: "condition",
      label: "É suporte?",
      position: { x: 0, y: 560 },
      config: { variable: "topic", operator: "eq", value: "Suporte" },
    },
    {
      id: "route_support",
      type: "handoff",
      label: "Fila de suporte",
      position: { x: -220, y: 700 },
      config: { reason: "Cliente pediu suporte" },
    },
    {
      id: "route_sales",
      type: "message",
      label: "Mensagem comercial",
      position: { x: 220, y: 700 },
      config: {
        text: "Certo. Já registrei seu contato e um especialista vai te responder por aqui.",
      },
    },
    {
      id: "finish",
      type: "end",
      label: "Encerrar",
      position: { x: 220, y: 840 },
      config: {},
    },
  ],
  edges: [
    { id: "e1", source: "start", target: "greeting" },
    { id: "e2", source: "greeting", target: "ask_name" },
    { id: "e3", source: "ask_name", target: "ask_topic" },
    { id: "e4", source: "ask_topic", target: "check_support" },
    {
      id: "e5",
      source: "check_support",
      target: "route_support",
      sourceHandle: "true",
      label: "Sim",
    },
    {
      id: "e6",
      source: "check_support",
      target: "ask_contact",
      sourceHandle: "false",
      label: "Não",
    },
    { id: "e7", source: "ask_contact", target: "route_sales" },
    { id: "e8", source: "route_sales", target: "finish" },
  ],
};
