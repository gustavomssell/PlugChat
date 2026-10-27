import type {
  Agent,
  ApiError,
  Attachment,
  AuthResponse,
  AuthResult,
  Availability,
  BulkResult,
  Channel,
  Conversation,
  FlowGraph,
  FlowSummary,
  Message,
  Metrics,
  SavedReply,
  SendMessageResponse,
  Stats,
} from "@chatfree/shared";
import { getToken } from "@/lib/session";

const API_URL = import.meta.env.VITE_API_URL ?? "";

export class ApiRequestError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "ApiRequestError";
    this.status = status;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const token = getToken();
  const headers = new Headers(init?.headers);
  headers.set("Content-Type", "application/json");
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }

  const response = await fetch(`${API_URL}${path}`, {
    ...init,
    headers,
  });

  if (response.status === 401 && token) {
    window.dispatchEvent(new Event("chatfree:unauthorized"));
  }

  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as ApiError | null;
    throw new ApiRequestError(body?.error ?? `Erro ${response.status}`, response.status);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}

export interface ConversationFilters {
  status?: string;
  channelId?: string;
  search?: string;
}

export interface ChannelInputPayload {
  name: string;
  flowId: string;
  appearance: Channel["appearance"];
  allowedOrigins: string[];
}

export const api = {
  login: (email: string, password: string) =>
    request<AuthResponse>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    }),
  register: (input: { name: string; email: string; password: string; workspaceName: string }) =>
    request<AuthResponse>("/api/auth/register", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  me: () => request<AuthResult>("/api/auth/me"),
  setAvailability: (availability: Availability) =>
    request<{ availability: Availability }>("/api/auth/me/availability", {
      method: "PATCH",
      body: JSON.stringify({ availability }),
    }),

  stats: () => request<Stats>("/api/stats"),

  listAgents: () => request<Agent[]>("/api/agents"),
  createAgent: (input: { name: string; email: string; password: string }) =>
    request<Agent>("/api/agents", {
      method: "POST",
      body: JSON.stringify(input),
    }),

  listChannels: () => request<Channel[]>("/api/channels"),
  getChannel: (id: string) => request<Channel>(`/api/channels/${id}`),
  createChannel: (input: ChannelInputPayload) =>
    request<Channel>("/api/channels", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  updateChannel: (id: string, input: ChannelInputPayload) =>
    request<Channel>(`/api/channels/${id}`, {
      method: "PUT",
      body: JSON.stringify(input),
    }),
  deleteChannel: (id: string) =>
    request<void>(`/api/channels/${id}`, { method: "DELETE" }),

  listFlows: () => request<FlowSummary[]>("/api/flows"),
  getFlow: (flowId: string) => request<FlowGraph>(`/api/flows/${flowId}`),
  createFlow: (name: string) =>
    request<FlowGraph>("/api/flows", {
      method: "POST",
      body: JSON.stringify({ name }),
    }),
  saveFlow: (flow: FlowGraph) =>
    request<FlowGraph>(`/api/flows/${flow.id}`, {
      method: "PUT",
      body: JSON.stringify(flow),
    }),
  deleteFlow: (flowId: string) =>
    request<void>(`/api/flows/${flowId}`, { method: "DELETE" }),

  listConversations: (filters: ConversationFilters = {}) => {
    const params = new URLSearchParams();
    if (filters.status) params.set("status", filters.status);
    if (filters.channelId) params.set("channelId", filters.channelId);
    if (filters.search) params.set("search", filters.search);
    const query = params.toString();
    return request<Conversation[]>(`/api/conversations${query ? `?${query}` : ""}`);
  },
  getConversation: (id: string) => request<Conversation>(`/api/conversations/${id}`),
  patchConversation: (
    id: string,
    patch: { status?: string; assignedUserId?: string; tags?: string[] },
  ) =>
    request<Conversation>(`/api/conversations/${id}`, {
      method: "PATCH",
      body: JSON.stringify(patch),
    }),
  bulkConversations: (input: {
    ids: string[];
    action: "assign" | "tag" | "status";
    assignedUserId?: string;
    tags?: string[];
    status?: string;
  }) =>
    request<BulkResult>("/api/conversations/bulk", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  listMessages: (conversationId: string) =>
    request<Message[]>(`/api/conversations/${conversationId}/messages`),
  reply: (conversationId: string, text: string, attachmentIds?: string[]) =>
    request<SendMessageResponse>(`/api/conversations/${conversationId}/messages`, {
      method: "POST",
      body: JSON.stringify({ text, attachmentIds }),
    }),
  addNote: (conversationId: string, text: string) =>
    request<Message>(`/api/conversations/${conversationId}/notes`, {
      method: "POST",
      body: JSON.stringify({ text }),
    }),
  exportConversations: (filters: ConversationFilters = {}) => {
    const params = new URLSearchParams();
    if (filters.status) params.set("status", filters.status);
    if (filters.channelId) params.set("channelId", filters.channelId);
    if (filters.search) params.set("search", filters.search);
    const query = params.toString();
    return `${apiUrl(`/api/conversations/export${query ? `?${query}` : ""}`)}${
      getToken() ? `${query ? "&" : "?"}token=${encodeURIComponent(getToken() ?? "")}` : ""
    }`;
  },

  listReplies: () => request<SavedReply[]>("/api/replies"),
  createReply: (input: { title: string; body: string; shortcut?: string; tags?: string[] }) =>
    request<SavedReply>("/api/replies", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  updateReply: (
    id: string,
    input: { title: string; body: string; shortcut?: string; tags?: string[] },
  ) =>
    request<SavedReply>(`/api/replies/${id}`, {
      method: "PUT",
      body: JSON.stringify(input),
    }),
  deleteReply: (id: string) => request<void>(`/api/replies/${id}`, { method: "DELETE" }),

  metrics: (days = 7) => request<Metrics>(`/api/metrics?days=${days}`),
};

export function attachmentUrl(id: string) {
  return `${apiUrl(`/api/attachments/${id}`)}${
    getToken() ? `?token=${encodeURIComponent(getToken() ?? "")}` : ""
  }`;
}

/** Envia um arquivo como formulário multiparte e devolve o anexo pendente. */
export async function uploadAttachment(conversationId: string, file: File): Promise<Attachment> {
  const form = new FormData();
  form.append("file", file);

  const response = await fetch(`${API_URL}/api/conversations/${conversationId}/attachments`, {
    method: "POST",
    headers: tokenHeader(),
    body: form,
  });

  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as ApiError | null;
    throw new ApiRequestError(body?.error ?? `Erro ${response.status}`, response.status);
  }

  return (await response.json()) as Attachment;
}

function tokenHeader(): Record<string, string> {
  const token = getToken();
  return token ? { Authorization: `Bearer ${token}` } : {};
}

export function apiUrl(path: string) {
  return `${API_URL}${path}`;
}

export function streamUrl(path: string) {
  const token = getToken();
  const separator = path.includes("?") ? "&" : "?";
  return `${API_URL}${path}${token ? `${separator}token=${encodeURIComponent(token)}` : ""}`;
}
