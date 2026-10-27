import type {
  ApiError,
  Attachment,
  Conversation,
  Message,
  SendMessageResponse,
  StartConversationResponse,
} from "@chatfree/shared/api";
import type { WidgetConfig } from "@chatfree/shared/widget";

export function createWidgetClient(apiUrl: string, key: string) {
  const base = apiUrl.replace(/\/$/, "");

  async function request<T>(path: string, init?: RequestInit): Promise<T> {
    const response = await fetch(`${base}${path}`, {
      ...init,
      headers: {
        "Content-Type": "application/json",
        ...(init?.headers ?? {}),
      },
    });

    if (!response.ok) {
      // A mensagem da API (ex.: "preencha o campo: E-mail") é mais útil que um
      // texto genérico, então ela vira a mensagem do erro.
      let detail = "";
      try {
        const payload = (await response.json()) as ApiError;
        if (typeof payload?.error === "string") {
          detail = payload.error;
        }
      } catch {
        // corpo não-JSON: fica só o status
      }
      throw new Error(detail || `ChatFree API ${response.status}`);
    }

    return (await response.json()) as T;
  }

  async function upload<T>(path: string, form: FormData): Promise<T> {
    // O cabeçalho de JSON não pode ir junto com o multipart: o navegador
    // precisa definir o boundary sozinho.
    const response = await fetch(`${base}${path}`, { method: "POST", body: form });

    if (!response.ok) {
      let detail = "";
      try {
        const payload = (await response.json()) as ApiError;
        if (typeof payload?.error === "string") {
          detail = payload.error;
        }
      } catch {
        // corpo não-JSON: fica só o status
      }
      throw new Error(detail || `ChatFree API ${response.status}`);
    }

    return (await response.json()) as T;
  }

  return {
    config: () => request<WidgetConfig>(`/api/widget/config/${key}`),
    start: (visitorId?: string) =>
      request<StartConversationResponse>(`/api/widget/start/${key}`, {
        method: "POST",
        body: JSON.stringify(visitorId ? { visitorId } : {}),
      }),
    conversation: (conversationId: string) =>
      request<Conversation>(`/api/widget/conversations/${conversationId}`),
    messages: (conversationId: string) =>
      request<Message[]>(`/api/widget/conversations/${conversationId}/messages`),
    send: (
      conversationId: string,
      input: { text?: string; fields?: Record<string, string>; attachmentIds?: string[] },
    ) =>
      request<SendMessageResponse>(`/api/widget/conversations/${conversationId}/messages`, {
        method: "POST",
        body: JSON.stringify(input),
      }),
    upload: (conversationId: string, file: File) => {
      const form = new FormData();
      form.append("file", file);
      return upload<Attachment>(`/api/widget/conversations/${conversationId}/attachments`, form);
    },
    attachmentUrl: (attachmentId: string, conversationId: string) =>
      `${base}/api/widget/attachments/${attachmentId}?conversation=${encodeURIComponent(conversationId)}`,
    eventsUrl: (conversationId: string) =>
      `${base}/api/widget/conversations/${conversationId}/events`,
  };
}

export type WidgetClient = ReturnType<typeof createWidgetClient>;
