import { useEffect, useMemo, useRef, useState } from "preact/hooks";
import { DEFAULT_APPEARANCE } from "@chatfree/shared/widget";
import type {
  Attachment,
  Conversation,
  ConversationStatus,
  Message,
  Wait,
  WidgetAppearance,
  WidgetBootstrap,
  WidgetConfig,
} from "@chatfree/shared";
import { createWidgetClient } from "./client";

type Status = "idle" | "loading" | "ready" | "error";

function initialAppearance(bootstrap: WidgetBootstrap): WidgetAppearance {
  return {
    mode: bootstrap.mode ?? DEFAULT_APPEARANCE.mode,
    position: bootstrap.position ?? DEFAULT_APPEARANCE.position,
    theme: bootstrap.theme ?? DEFAULT_APPEARANCE.theme,
    accent: bootstrap.accent ?? DEFAULT_APPEARANCE.accent,
    greeting: bootstrap.greeting ?? DEFAULT_APPEARANCE.greeting,
    launcherLabel: bootstrap.launcherLabel ?? DEFAULT_APPEARANCE.launcherLabel,
    showBranding: DEFAULT_APPEARANCE.showBranding,
  };
}

function mergeMessages(current: Message[], incoming: Message[]) {
  const known = new Set(current.map((message) => message.id));
  return [...current, ...incoming.filter((message) => !known.has(message.id))];
}

function storageKey(key: string, suffix: string) {
  return `chatfree:${key}:${suffix}`;
}

function readStorage(key: string) {
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
}

function writeStorage(key: string, value: string) {
  try {
    window.localStorage.setItem(key, value);
  } catch {
    return;
  }
}

function clearStorage(key: string) {
  try {
    window.localStorage.removeItem(key);
  } catch {
    return;
  }
}

function formatFormEcho(values: Record<string, string>) {
  return Object.entries(values)
    .map(([name, value]) => `${name}: ${value}`)
    .join(" · ");
}

function ensureVisitorId(key: string) {
  const storage = storageKey(key, "visitor");
  const existing = readStorage(storage);
  if (existing) {
    return existing;
  }

  const visitorId =
    typeof crypto !== "undefined" && "randomUUID" in crypto
      ? crypto.randomUUID()
      : `visitor-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  writeStorage(storage, visitorId);
  return visitorId;
}

function mergeAppearance(
  remote: WidgetAppearance,
  bootstrap: WidgetBootstrap,
): WidgetAppearance {
  return {
    mode: bootstrap.mode ?? remote.mode,
    position: bootstrap.position ?? remote.position,
    theme: bootstrap.theme ?? remote.theme,
    accent: bootstrap.accent ?? remote.accent,
    greeting: bootstrap.greeting ?? remote.greeting,
    launcherLabel: bootstrap.launcherLabel ?? remote.launcherLabel,
    showBranding: remote.showBranding,
  };
}

export function Widget({ bootstrap }: { bootstrap: WidgetBootstrap }) {
  const client = useMemo(
    () => createWidgetClient(bootstrap.apiUrl, bootstrap.key),
    [bootstrap.apiUrl, bootstrap.key],
  );

  const [config, setConfig] = useState<WidgetConfig>({
    key: bootstrap.key,
    name: "Atendimento",
    appearance: initialAppearance(bootstrap),
  });
  const [open, setOpen] = useState(
    bootstrap.mode === "inline" || bootstrap.mode === "fullscreen",
  );
  const [status, setStatus] = useState<Status>("idle");
  const [conversationId, setConversationId] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [waiting, setWaiting] = useState<Wait | null>(null);
  const [conversationStatus, setConversationStatus] = useState<ConversationStatus | null>(null);
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [typing, setTyping] = useState(false);
  const [formValues, setFormValues] = useState<Record<string, string>>({});
  // Anexos esperando para entrar na próxima mensagem do visitante.
  const [pendingAttachments, setPendingAttachments] = useState<Attachment[]>([]);
  const [uploading, setUploading] = useState(false);
  const [recording, setRecording] = useState(false);
  const recorderRef = useRef<MediaRecorder | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  const appearance = config.appearance;
  const closed = conversationStatus === "closed";
  const activeForm =
    waiting?.inputType === "form" && waiting.fields && waiting.fields.length > 0
      ? waiting
      : null;
  const formReady = activeForm
    ? (activeForm.fields ?? []).some((field) => (formValues[field.name] ?? "").trim() !== "")
    : false;
  const statusLabel =
    conversationStatus === "queued"
      ? "Aguardando atendente"
      : conversationStatus === "human"
        ? "Atendimento humano"
        : closed
          ? "Atendimento encerrado"
          : "Online agora";

  useEffect(() => {
    let alive = true;
    client
      .config()
      .then((remote) => {
        if (alive) {
          setConfig({
            ...remote,
            appearance: mergeAppearance(remote.appearance, bootstrap),
          });
        }
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, [client, bootstrap]);

  useEffect(() => {
    if (!open || conversationId) {
      return;
    }

    let alive = true;
    const conversationStorage = storageKey(bootstrap.key, "conversation");

    const start = async () => {
      setStatus("loading");

      const savedID = readStorage(conversationStorage);
      if (savedID) {
        try {
          const [conversation, history] = await Promise.all([
            client.conversation(savedID),
            client.messages(savedID),
          ]);

          if (!alive) {
            return;
          }

          if (conversation.status !== "closed") {
            setConversationId(conversation.id);
            setMessages(history);
            setWaiting(conversation.waiting ?? null);
            setConversationStatus(conversation.status);
            setStatus("ready");
            return;
          }

          clearStorage(conversationStorage);
        } catch {
          clearStorage(conversationStorage);
        }
      }

      try {
        const response = await client.start(ensureVisitorId(bootstrap.key));
        if (!alive) {
          return;
        }
        writeStorage(conversationStorage, response.conversation.id);
        setConversationId(response.conversation.id);
        setMessages(response.messages);
        setWaiting(response.conversation.waiting ?? null);
        setConversationStatus(response.conversation.status);
        setStatus("ready");
      } catch {
        if (alive) {
          setStatus("error");
        }
      }
    };

    void start();

    return () => {
      alive = false;
    };
  }, [open, client, bootstrap.key, conversationId]);

  useEffect(() => {
    if (!conversationId) {
      return;
    }

    const source = new EventSource(client.eventsUrl(conversationId));

    source.addEventListener("chat", (event) => {
      try {
        const message = JSON.parse((event as MessageEvent).data) as Message;
        if (message.role === "visitor") {
          return;
        }
        setTyping(false);
        setMessages((current) => mergeMessages(current, [message]));
      } catch {
        return;
      }
    });

    // "Digitando…": nasce no atraso e na chamada de IA, morre na resposta.
    source.addEventListener("typing", (event) => {
      try {
        const payload = JSON.parse((event as MessageEvent).data) as { ms?: number };
        const ms = typeof payload.ms === "number" ? payload.ms : 0;
        setTyping(true);
        if (ms > 0) {
          window.setTimeout(() => setTyping(false), ms + 250);
        }
      } catch {
        return;
      }
    });

    source.addEventListener("conversation", (event) => {
      try {
        const conversation = JSON.parse((event as MessageEvent).data) as Conversation;
        setWaiting(conversation.waiting ?? null);
        setConversationStatus(conversation.status);
      } catch {
        return;
      }
    });

    return () => source.close();
  }, [conversationId, client]);

  useEffect(() => {
    const element = scrollRef.current;
    if (element) {
      element.scrollTop = element.scrollHeight;
    }
  }, [messages, status, typing]);

  // Cada formulário começa limpo, sem herdar valores de uma etapa anterior.
  useEffect(() => {
    setFormValues({});
  }, [activeForm?.nodeId]);

  const send = async (override?: string) => {
    const text = (override ?? draft).trim();
    const isForm = waiting?.inputType === "form";
    const filled = isForm
      ? Object.fromEntries(
          Object.entries(formValues)
            .map(([name, value]) => [name, value.trim()])
            .filter(([, value]) => value !== ""),
        )
      : {};

    const attachmentIds = pendingAttachments.map((item) => item.id);
    if ((!text && Object.keys(filled).length === 0 && attachmentIds.length === 0) || !conversationId || sending) {
      return;
    }

    const display = text || formatFormEcho(filled);
    const optimistic: Message = {
      id: `local-${Date.now()}`,
      conversationId,
      role: "visitor",
      text: display,
      attachments: pendingAttachments,
      createdAt: new Date().toISOString(),
    };

    // Estado anterior: se o envio falhar (ex.: campo obrigatório vazio) o
    // formulário volta exatamente como estava, com os dados já digitados.
    const previousWaiting = waiting;
    const previousValues = formValues;
    const previousAttachments = pendingAttachments;

    if (!override) {
      setDraft("");
    }
    if (isForm) {
      setFormValues({});
    }
    setPendingAttachments([]);
    setWaiting(null);
    setSending(true);
    // Indicador otimista: o visitante vê atividade enquanto a API responde.
    setTyping(true);
    setMessages((current) => [...current, optimistic]);

    try {
      const response = await client.send(conversationId, {
        text: text || undefined,
        fields: filled,
        attachmentIds: attachmentIds.length > 0 ? attachmentIds : undefined,
      });
      setTyping(false);
      setMessages((current) => mergeMessages(current, response.messages));
      setWaiting(response.conversation.waiting ?? null);
      setConversationStatus(response.conversation.status);
    } catch (error) {
      setTyping(false);
      setWaiting(previousWaiting);
      setFormValues(previousValues);
      setPendingAttachments(previousAttachments);
      const detail = error instanceof Error ? error.message.trim() : "";
      setMessages((current) => [
        ...current,
        {
          id: `error-${Date.now()}`,
          conversationId,
          role: "system",
          text: detail || "Não foi possível enviar sua mensagem. Tente novamente.",
          createdAt: new Date().toISOString(),
        },
      ]);
    } finally {
      setSending(false);
    }
  };

  // Upload de imagem ou arquivo escolhido no aparelho.
  const handleFiles = (files: FileList | null) => {
    if (!files || files.length === 0 || !conversationId) return;
    for (const file of Array.from(files)) {
      if (file.size > 8 * 1024 * 1024) {
        setMessages((current) => [
          ...current,
          {
            id: `error-${Date.now()}`,
            conversationId,
            role: "system",
            text: `"${file.name}" passa de 8 MB.`,
            createdAt: new Date().toISOString(),
          },
        ]);
        continue;
      }
      setUploading(true);
      client
        .upload(conversationId, file)
        .then((attachment) => {
          setPendingAttachments((current) => [...current, attachment]);
        })
        .catch((error) => {
          const detail = error instanceof Error ? error.message.trim() : "";
          setMessages((current) => [
            ...current,
            {
              id: `error-${Date.now()}`,
              conversationId,
              role: "system",
              text: detail || "Não foi possível enviar o anexo.",
              createdAt: new Date().toISOString(),
            },
          ]);
        })
        .finally(() => setUploading(false));
    }
  };

  // Gravação de áudio pelo microfone, quando o navegador permite.
  const toggleRecording = async () => {
    if (recording) {
      recorderRef.current?.stop();
      return;
    }

    if (!conversationId) return;
    if (typeof MediaRecorder === "undefined" || !navigator.mediaDevices?.getUserMedia) {
      setMessages((current) => [
        ...current,
        {
          id: `error-${Date.now()}`,
          conversationId,
          role: "system",
          text: "Seu navegador não permite gravar áudio aqui.",
          createdAt: new Date().toISOString(),
        },
      ]);
      return;
    }

    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      const recorder = new MediaRecorder(stream);
      const chunks: Blob[] = [];

      recorder.ondataavailable = (event) => {
        if (event.data.size > 0) chunks.push(event.data);
      };
      recorder.onstop = () => {
        stream.getTracks().forEach((track) => track.stop());
        setRecording(false);
        recorderRef.current = null;

        const blob = new Blob(chunks, { type: recorder.mimeType || "audio/webm" });
        if (blob.size === 0) return;
        const name = `audio-${new Date().toISOString()}.webm`;
        const file = new File([blob], name, { type: blob.type });
        setUploading(true);
        client
          .upload(conversationId!, file)
          .then((attachment) => setPendingAttachments((current) => [...current, attachment]))
          .catch(() => undefined)
          .finally(() => setUploading(false));
      };

      recorderRef.current = recorder;
      recorder.start();
      setRecording(true);
    } catch {
      setMessages((current) => [
        ...current,
        {
          id: `error-${Date.now()}`,
          conversationId,
          role: "system",
          text: "Não foi possível acessar o microfone.",
          createdAt: new Date().toISOString(),
        },
      ]);
    }
  };

  const resetConversation = () => {
    clearStorage(storageKey(bootstrap.key, "conversation"));
    setConversationId(null);
    setMessages([]);
    setWaiting(null);
    setConversationStatus(null);
    setStatus("idle");
  };

  const retry = () => {
    resetConversation();
    setOpen(false);
    window.setTimeout(() => setOpen(true), 0);
  };

  const startNew = () => {
    resetConversation();
  };

  const panel = (
    <section
      class={`cf-panel cf-mode-${appearance.mode} cf-pos-${appearance.position} cf-theme-${appearance.theme}`}
      aria-label={`Chat de ${config.name}`}
    >
      <header class="cf-header">
        <div class="cf-header-info">
          <span class="cf-avatar" aria-hidden="true">
            {config.name.slice(0, 1).toUpperCase()}
          </span>
          <span class="cf-header-text">
            <strong>{config.name}</strong>
            <span class="cf-status">
              <span class={closed ? "cf-dot cf-dot-off" : "cf-dot"} aria-hidden="true" />
              {statusLabel}
            </span>
          </span>
        </div>
        {appearance.mode !== "inline" && appearance.mode !== "fullscreen" ? (
          <button
            type="button"
            class="cf-close"
            aria-label="Fechar chat"
            onClick={() => setOpen(false)}
          >
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M6 6l12 12M18 6L6 18" />
            </svg>
          </button>
        ) : null}
      </header>

      <div class="cf-messages" ref={scrollRef} aria-live="polite">
        {messages.length === 0 ? (
          <div class="cf-message cf-message-bot">
            <p>{appearance.greeting}</p>
          </div>
        ) : null}

        {status === "loading" || typing ? (
          <div class="cf-message cf-message-bot cf-typing" aria-label="Digitando">
            <span />
            <span />
            <span />
          </div>
        ) : null}

        {status === "error" ? (
          <div class="cf-message cf-message-system">
            <p>Não conseguimos iniciar o atendimento.</p>
            <button type="button" class="cf-retry" onClick={retry}>
              Tentar novamente
            </button>
          </div>
        ) : null}

        {messages.map((message) => (
          <div
            key={message.id}
            class={
              message.role === "visitor"
                ? "cf-message cf-message-visitor"
                : message.role === "system"
                  ? "cf-message cf-message-system"
                  : "cf-message cf-message-bot"
            }
          >
            {message.text ? <p>{message.text}</p> : null}
            {message.attachments && message.attachments.length > 0 && conversationId ? (
              <div class="cf-attachments">
                {message.attachments.map((attachment) => {
                  const url = client.attachmentUrl(attachment.id, conversationId);
                  if (attachment.kind === "image") {
                    return (
                      <a
                        key={attachment.id}
                        class="cf-attachment-image"
                        href={url}
                        target="_blank"
                        rel="noreferrer"
                      >
                        <img src={url} alt={attachment.filename} loading="lazy" />
                      </a>
                    );
                  }
                  if (attachment.kind === "audio") {
                    return (
                      <audio
                        key={attachment.id}
                        class="cf-attachment-audio"
                        controls
                        src={url}
                        preload="none"
                      />
                    );
                  }
                  return (
                    <a
                      key={attachment.id}
                      class="cf-attachment-file"
                      href={url}
                      download={attachment.filename}
                    >
                      {attachment.filename}
                    </a>
                  );
                })}
              </div>
            ) : null}
          </div>
        ))}

        {closed ? (
          <div class="cf-message cf-message-system">
            <p>Este atendimento foi encerrado.</p>
            <button type="button" class="cf-retry" onClick={startNew}>
              Iniciar nova conversa
            </button>
          </div>
        ) : null}
      </div>

      {waiting?.inputType === "choice" && waiting.options && waiting.options.length > 0 ? (
        <div class="cf-options" aria-label="Opções de resposta">
          {waiting.options.map((option) => (
            <button
              key={option.value}
              type="button"
              class="cf-option"
              disabled={sending}
              onClick={() => void send(option.value || option.label)}
            >
              {option.label}
            </button>
          ))}
        </div>
      ) : null}

      {activeForm ? (
        <form
          class="cf-form"
          aria-label="Formulário de contato"
          onSubmit={(event) => {
            event.preventDefault();
            void send();
          }}
        >
          {(activeForm.fields ?? []).map((field) => (
            <label key={field.name} class="cf-form-field">
              <span>
                {field.label || field.name}
                {field.required ? " *" : ""}
              </span>
              {field.type === "choice" && field.options && field.options.length > 0 ? (
                <select
                  value={formValues[field.name] ?? ""}
                  onChange={(event) =>
                    setFormValues((current) => ({
                      ...current,
                      [field.name]: (event.target as HTMLSelectElement).value,
                    }))
                  }
                >
                  <option value="">Selecione…</option>
                  {field.options.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              ) : (
                <input
                  type={
                    field.type === "email"
                      ? "email"
                      : field.type === "phone"
                        ? "tel"
                        : field.type === "number"
                          ? "number"
                          : "text"
                  }
                  value={formValues[field.name] ?? ""}
                  placeholder={field.label || field.name}
                  onInput={(event) =>
                    setFormValues((current) => ({
                      ...current,
                      [field.name]: (event.target as HTMLInputElement).value,
                    }))
                  }
                />
              )}
            </label>
          ))}
          <button type="submit" class="cf-send" disabled={sending || !formReady}>
            Enviar
          </button>
        </form>
      ) : null}

      {activeForm ? null : (
      <form
        class="cf-composer"
        onSubmit={(event) => {
          event.preventDefault();
          void send();
        }}
      >
        {pendingAttachments.length > 0 ? (
          <div class="cf-pending" aria-label="Anexos pendentes">
            {pendingAttachments.map((attachment) => (
              <span key={attachment.id} class="cf-pending-item">
                {attachment.filename}
                <button
                  type="button"
                  aria-label={`Remover anexo ${attachment.filename}`}
                  onClick={() =>
                    setPendingAttachments((current) =>
                      current.filter((item) => item.id !== attachment.id),
                    )
                  }
                >
                  ×
                </button>
              </span>
            ))}
          </div>
        ) : null}

        <div class="cf-composer-row">
          <label
            class="cf-icon-button"
            title="Anexar imagem ou arquivo"
            aria-label="Anexar imagem ou arquivo"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M21.4 11.05l-9.19 9.19a5 5 0 01-7.07-7.07l9.19-9.19a3.5 3.5 0 014.95 4.95l-9.2 9.19a1.5 1.5 0 01-2.12-2.12l8.49-8.49" />
            </svg>
            <input
              type="file"
              multiple
              hidden
              disabled={!conversationId || uploading || closed}
              onChange={(event) => {
                const input = event.target as HTMLInputElement;
                handleFiles(input.files);
                input.value = "";
              }}
            />
          </label>

          <button
            type="button"
            class={recording ? "cf-icon-button cf-recording" : "cf-icon-button"}
            title={recording ? "Parar gravação" : "Gravar áudio"}
            aria-label={recording ? "Parar gravação" : "Gravar áudio"}
            disabled={!conversationId || closed}
            onClick={() => void toggleRecording()}
          >
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M12 15a3 3 0 003-3V6a3 3 0 10-6 0v6a3 3 0 003 3z" />
              <path d="M19 11a7 7 0 01-14 0M12 18v3" />
            </svg>
          </button>

          <input
            value={draft}
            onInput={(event) => setDraft((event.target as HTMLInputElement).value)}
            placeholder={
              closed
                ? "Atendimento encerrado"
                : status === "ready"
                  ? "Escreva sua mensagem..."
                  : "Aguardando atendimento..."
            }
            disabled={status !== "ready" || !conversationId || closed}
            aria-label="Mensagem"
          />
          <button
            type="submit"
            class="cf-send"
            disabled={
              (!draft.trim() && pendingAttachments.length === 0) ||
              status !== "ready" ||
              sending ||
              closed
            }
            aria-label="Enviar mensagem"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M4 12l16-8-5 8 5 8-16-8z" />
            </svg>
          </button>
        </div>
      </form>
      )}

      {appearance.showBranding ? (
        <footer class="cf-branding">
          Atendimento por <strong>ChatFree</strong>
        </footer>
      ) : null}
    </section>
  );

  if (appearance.mode === "inline") {
    return <div class="cf-inline-wrap">{panel}</div>;
  }

  if (appearance.mode === "fullscreen") {
    return <div class="cf-fullscreen-wrap">{panel}</div>;
  }

  if (appearance.mode === "popup") {
    return (
      <>
        {open ? (
          <div
            class="cf-backdrop"
            role="presentation"
            onClick={() => setOpen(false)}
          >
            <div onClick={(event) => event.stopPropagation()}>{panel}</div>
          </div>
        ) : (
          <Launcher label={appearance.launcherLabel} position={appearance.position} onClick={() => setOpen(true)} />
        )}
      </>
    );
  }

  return (
    <>
      {open ? panel : null}
      {open ? null : (
        <Launcher label={appearance.launcherLabel} position={appearance.position} onClick={() => setOpen(true)} />
      )}
    </>
  );
}

function Launcher({
  label,
  position,
  onClick,
}: {
  label: string;
  position: WidgetAppearance["position"];
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      class={`cf-launcher cf-pos-${position}`}
      aria-label={label}
      onClick={onClick}
    >
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <path d="M4 5.5A2.5 2.5 0 016.5 3h11A2.5 2.5 0 0120 5.5v8a2.5 2.5 0 01-2.5 2.5H9l-5 4V5.5z" />
      </svg>
      <span class="cf-launcher-pulse" aria-hidden="true" />
    </button>
  );
}
