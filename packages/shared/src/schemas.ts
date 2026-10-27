import { z } from "zod";
import { DISPLAY_MODES, WIDGET_POSITIONS, WIDGET_THEMES } from "./widget";

export const displayModeSchema = z.enum(DISPLAY_MODES);
export const widgetPositionSchema = z.enum(WIDGET_POSITIONS);
export const widgetThemeSchema = z.enum(WIDGET_THEMES);

export const appearanceSchema = z.object({
  mode: displayModeSchema,
  position: widgetPositionSchema,
  theme: widgetThemeSchema,
  accent: z.string().regex(/^#[0-9a-fA-F]{6}$/, "Use uma cor no formato #RRGGBB"),
  greeting: z.string().min(1, "Informe a saudação").max(280),
  launcherLabel: z.string().min(1, "Informe o texto do botão").max(60),
  showBranding: z.boolean(),
});

export const channelSchema = z.object({
  name: z.string().min(2, "Informe um nome").max(80),
  flowId: z.string().min(1, "Selecione um fluxo"),
  appearance: appearanceSchema,
});

export type ChannelInput = z.infer<typeof channelSchema>;
export type AppearanceInput = z.infer<typeof appearanceSchema>;

export const loginSchema = z.object({
  email: z.string().email("Informe um e-mail válido"),
  password: z.string().min(1, "Informe a senha"),
});

export const registerSchema = z.object({
  name: z.string().min(2, "Informe seu nome"),
  workspaceName: z.string().min(2, "Informe o nome do workspace"),
  email: z.string().email("Informe um e-mail válido"),
  password: z.string().min(8, "Use pelo menos 8 caracteres"),
});

export type LoginInput = z.infer<typeof loginSchema>;
export type RegisterInput = z.infer<typeof registerSchema>;

export const flowNodeSchema = z.object({
  id: z.string(),
  type: z.enum([
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
  ]),
  label: z.string().optional(),
  position: z.object({ x: z.number(), y: z.number() }),
  config: z.record(z.string(), z.unknown()).optional(),
});

export const flowEdgeSchema = z.object({
  id: z.string(),
  source: z.string(),
  target: z.string(),
  sourceHandle: z.string().optional(),
  label: z.string().optional(),
});

export const flowGraphSchema = z.object({
  id: z.string(),
  name: z.string(),
  version: z.number(),
  entryNodeId: z.string().optional(),
  nodes: z.array(flowNodeSchema),
  edges: z.array(flowEdgeSchema),
});
