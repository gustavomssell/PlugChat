import type { WidgetAppearance } from "./widget";

export const CONVERSATION_STATUSES = [
  "bot",
  "waiting",
  "queued",
  "human",
  "closed",
] as const;
export type ConversationStatus = (typeof CONVERSATION_STATUSES)[number];

export const MESSAGE_ROLES = ["visitor", "bot", "agent", "system", "note"] as const;
export type MessageRole = (typeof MESSAGE_ROLES)[number];

export const ATTACHMENT_KINDS = ["image", "audio", "file"] as const;
export type AttachmentKind = (typeof ATTACHMENT_KINDS)[number];

export const MEMBER_ROLES = ["owner", "admin", "agent"] as const;
export type MemberRole = (typeof MEMBER_ROLES)[number];

export const AVAILABILITIES = ["online", "busy", "offline"] as const;
export type Availability = (typeof AVAILABILITIES)[number];

export interface AuthUser {
  id: string;
  email: string;
  name: string;
  createdAt: string;
}

export interface AuthWorkspace {
  id: string;
  name: string;
  createdAt: string;
}

export interface Membership {
  role: MemberRole;
  availability: Availability;
}

export interface AuthResult {
  user: AuthUser;
  workspace: AuthWorkspace;
  membership: Membership;
}

export interface AuthResponse extends AuthResult {
  token: string;
}

export interface Agent {
  userId: string;
  name: string;
  email: string;
  role: MemberRole;
  availability: Availability;
}

export interface Channel {
  id: string;
  workspaceId: string;
  key: string;
  name: string;
  flowId: string;
  appearance: WidgetAppearance;
  allowedOrigins: string[];
  createdAt: string;
}

export interface WaitOption {
  label: string;
  value: string;
  /** Atalho: ao escolher, o fluxo pula direto para este nó. */
  nextNodeId?: string;
}

export interface WaitField {
  name: string;
  label: string;
  type?: "text" | "email" | "phone" | "number" | "choice" | "textarea";
  required?: boolean;
  options?: WaitOption[];
}

export interface Wait {
  nodeId: string;
  variable?: string;
  inputType?: string;
  options?: WaitOption[];
  /** Presente quando o nó coleta vários campos em uma única tela. */
  fields?: WaitField[];
  /** Destino de respostas que não casam com nenhuma opção. */
  fallbackNodeId?: string;
}

export interface Conversation {
  id: string;
  workspaceId: string;
  channelId: string;
  visitorId: string;
  status: ConversationStatus;
  flowId: string;
  flowVersion: number;
  currentNodeId: string;
  assignedUserId?: string;
  tags: string[];
  waiting?: Wait;
  createdAt: string;
  updatedAt: string;
}

export interface Attachment {
  id: string;
  conversationId: string;
  kind: AttachmentKind;
  filename: string;
  mime: string;
  sizeBytes: number;
}

export interface Message {
  id: string;
  conversationId: string;
  role: MessageRole;
  text: string;
  /** Preenchido em notas internas. */
  authorUserId?: string;
  authorName?: string;
  attachments?: Attachment[];
  createdAt: string;
}

export interface SavedReply {
  id: string;
  workspaceId: string;
  title: string;
  body: string;
  shortcut: string;
  tags: string[];
  createdAt: string;
  updatedAt: string;
}

export interface BulkResult {
  updated: number;
}

export interface DayCount {
  date: string;
  count: number;
}

export interface ChannelMetric {
  channelId: string;
  name: string;
  conversations: number;
  closed: number;
  firstResponseMs: number;
}

export interface AgentMetric {
  userId: string;
  name: string;
  conversations: number;
  closed: number;
  firstResponseMs: number;
}

export interface Metrics {
  days: number;
  conversations: number;
  closed: number;
  messages: number;
  firstResponseMs: number;
  conversationsByDay: DayCount[];
  messagesByDay: DayCount[];
  byChannel: ChannelMetric[];
  byAgent: AgentMetric[];
}

export interface StartConversationResponse {
  conversation: Conversation;
  messages: Message[];
}

export interface SendMessageResponse {
  conversation: Conversation;
  messages: Message[];
}

/** Corpo aceito por POST /api/widget/conversations/{id}/messages. */
export interface VisitorMessageBody {
  text?: string;
  fields?: Record<string, string>;
  /** Anexos enviados antes da mensagem que os reivindica. */
  attachmentIds?: string[];
}

export interface Stats {
  total: number;
  active: number;
  waiting: number;
  closed: number;
  messagesToday: number;
  conversationsByDay: DayCount[];
}

export interface ApiError {
  error: string;
}
