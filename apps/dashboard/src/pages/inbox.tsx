import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSearch } from "@tanstack/react-router";
import {
  Bot,
  BookOpen,
  CheckCircle2,
  ChevronLeft,
  Download,
  Loader2,
  MessageSquare,
  Paperclip,
  Plus,
  Radio,
  RotateCcw,
  Search,
  SendHorizonal,
  StickyNote,
  Tag,
  Trash2,
  UserCheck,
  UserRound,
  X,
} from "lucide-react";
import type {
  Attachment,
  Conversation,
  ConversationStatus,
  Message,
  MessageRole,
  SavedReply,
} from "@chatfree/shared";
import { useAuth } from "@/lib/auth";
import { api, attachmentUrl, streamUrl, uploadAttachment } from "@/lib/api";
import {
  formatBytes,
  formatDate,
  formatTime,
  initials,
  messageRoleLabel,
  relativeTime,
  visitorLabel,
} from "@/lib/labels";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { EmptyState } from "@/components/empty-state";
import { StatusBadge } from "@/components/status-badge";
import { useToast } from "@/components/ui/toast";

type Tab = "all" | "queue" | "human" | "closed";

const tabs: { id: Tab; label: string }[] = [
  { id: "all", label: "Todas" },
  { id: "queue", label: "Aguardando" },
  { id: "human", label: "Em atendimento" },
  { id: "closed", label: "Encerradas" },
];

function isQueued(status: ConversationStatus) {
  return status === "waiting" || status === "queued";
}

function dayKey(iso: string) {
  const date = new Date(iso);
  return `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}`;
}

function dayHeading(iso: string) {
  const date = new Date(iso);
  const today = new Date();
  const yesterday = new Date();
  yesterday.setDate(today.getDate() - 1);

  if (dayKey(iso) === dayKey(today.toISOString())) return "Hoje";
  if (dayKey(iso) === dayKey(yesterday.toISOString())) return "Ontem";
  return formatDate(iso);
}

export function InboxPage() {
  const { session } = useAuth();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const searchParams = useSearch({ from: "/_app/inbox" });

  const [selectedId, setSelectedId] = useState<string | null>(searchParams.conversation ?? null);
  const [mobileDetail, setMobileDetail] = useState(Boolean(searchParams.conversation));
  const [tab, setTab] = useState<Tab>("all");
  const [search, setSearch] = useState("");
  const [channelFilter, setChannelFilter] = useState("all");
  const [draft, setDraft] = useState("");
  const [tagDraft, setTagDraft] = useState("");
  const [live, setLive] = useState(false);
  // Seleção em massa nas conversas, notas internas e anexos pendentes.
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [bulkTag, setBulkTag] = useState("");
  const [noteDraft, setNoteDraft] = useState("");
  const [showNoteComposer, setShowNoteComposer] = useState(false);
  const [pendingAttachments, setPendingAttachments] = useState<Attachment[]>([]);
  const [showReplies, setShowReplies] = useState(false);

  const conversations = useQuery({
    queryKey: ["conversations"],
    queryFn: () => api.listConversations(),
    refetchInterval: 30_000,
  });

  const agents = useQuery({ queryKey: ["agents"], queryFn: api.listAgents });
  const channels = useQuery({ queryKey: ["channels"], queryFn: api.listChannels });
  const replies = useQuery({ queryKey: ["replies"], queryFn: api.listReplies });

  useEffect(() => {
    if (searchParams.conversation) {
      setSelectedId(searchParams.conversation);
      setMobileDetail(true);
    }
  }, [searchParams.conversation]);

  const scoped = useMemo(() => {
    return (conversations.data ?? []).filter(
      (conversation) => channelFilter === "all" || conversation.channelId === channelFilter,
    );
  }, [conversations.data, channelFilter]);

  const filtered = useMemo(() => {
    const term = search.trim().toLowerCase();
    return scoped.filter((conversation) => {
      if (tab === "queue" && !isQueued(conversation.status)) return false;
      if (tab === "human" && conversation.status !== "human") return false;
      if (tab === "closed" && conversation.status !== "closed") return false;
      if (
        term &&
        !conversation.visitorId.toLowerCase().includes(term) &&
        !conversation.id.toLowerCase().includes(term) &&
        !conversation.tags.some((tag) => tag.toLowerCase().includes(term))
      ) {
        return false;
      }
      return true;
    });
  }, [scoped, tab, search]);

  useEffect(() => {
    if (conversations.isLoading) return;
    if (filtered.length === 0) {
      setSelectedId(null);
      return;
    }
    if (!selectedId || !filtered.some((item) => item.id === selectedId)) {
      setSelectedId(filtered[0]?.id ?? null);
    }
  }, [filtered, selectedId, conversations.isLoading]);

  const selected = useMemo(
    () => conversations.data?.find((item) => item.id === selectedId) ?? null,
    [conversations.data, selectedId],
  );
  const selectedClosed = selected?.status === "closed";

  // A seleção em massa nunca sobrevive a uma conversa que saiu da lista.
  useEffect(() => {
    setSelectedIds((current) => {
      if (current.length === 0) return current;
      const visible = new Set(filtered.map((item) => item.id));
      const kept = current.filter((id) => visible.has(id));
      return kept.length === current.length ? current : kept;
    });
  }, [filtered]);

  const toggleSelected = useCallback((id: string) => {
    setSelectedIds((current) =>
      current.includes(id) ? current.filter((item) => item !== id) : [...current, id],
    );
  }, []);

  const toggleSelectAll = useCallback(() => {
    setSelectedIds((current) =>
      current.length === filtered.length
        ? []
        : filtered.map((item) => item.id),
    );
  }, [filtered]);

  const messages = useQuery({
    queryKey: ["messages", selectedId],
    queryFn: () => api.listMessages(selectedId as string),
    enabled: Boolean(selectedId),
  });

  const messagesRef = useRef<HTMLDivElement>(null);
  const messageCount = messages.data?.length ?? 0;

  useEffect(() => {
    const viewport = messagesRef.current?.querySelector<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );
    if (viewport) {
      viewport.scrollTop = viewport.scrollHeight;
    }
  }, [messageCount, selectedId]);

  const refresh = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: ["conversations"] });
    void queryClient.invalidateQueries({ queryKey: ["stats"] });
    if (selectedId) {
      void queryClient.invalidateQueries({ queryKey: ["messages", selectedId] });
    }
  }, [queryClient, selectedId]);

  useEffect(() => {
    const source = new EventSource(streamUrl("/api/events"));
    source.addEventListener("open", () => setLive(true));
    source.addEventListener("error", () => setLive(false));
    source.addEventListener("conversation", refresh);
    source.addEventListener("agent", () => {
      void queryClient.invalidateQueries({ queryKey: ["agents"] });
    });
    // Notas internas chegam pelo inbox: a conversa aberta recarrega o histórico.
    source.addEventListener("note", (event) => {
      try {
        const note = JSON.parse((event as MessageEvent).data) as Message;
        void queryClient.invalidateQueries({ queryKey: ["messages", note.conversationId] });
      } catch {
        return;
      }
    });
    return () => source.close();
  }, [queryClient, refresh]);

  useEffect(() => {
    if (!selectedId) return;
    const source = new EventSource(streamUrl(`/api/conversations/${selectedId}/events`));
    const handler = () => {
      void queryClient.invalidateQueries({ queryKey: ["messages", selectedId] });
      void queryClient.invalidateQueries({ queryKey: ["conversations"] });
    };
    source.addEventListener("chat", handler);
    source.addEventListener("conversation", handler);
    return () => source.close();
  }, [selectedId, queryClient]);

  const reply = useMutation({
    mutationFn: ({
      conversationId,
      text,
      attachmentIds,
    }: {
      conversationId: string;
      text: string;
      attachmentIds?: string[];
    }) => api.reply(conversationId, text, attachmentIds),
    onSuccess: () => {
      setDraft("");
      refresh();
      void queryClient.invalidateQueries({ queryKey: ["agents"] });
    },
    onError: (error) => {
      toast({
        tone: "error",
        title: "Não foi possível enviar",
        description: error instanceof Error ? error.message : "Tente novamente em instantes.",
      });
    },
  });

  const patch = useMutation({
    mutationFn: ({
      conversationId,
      data,
    }: {
      conversationId: string;
      data: { status?: string; assignedUserId?: string; tags?: string[] };
    }) => api.patchConversation(conversationId, data),
    onSuccess: (_, variables) => {
      refresh();
      if (variables.data.status === "closed") {
        toast({ tone: "success", title: "Conversa encerrada" });
      } else if (variables.data.assignedUserId === session?.user.id) {
        toast({ tone: "success", title: "Conversa atribuída a você" });
      } else if (variables.data.tags) {
        toast({ title: "Tags atualizadas" });
      }
    },
    onError: (error) => {
      toast({
        tone: "error",
        title: "Falha ao atualizar conversa",
        description: error instanceof Error ? error.message : undefined,
      });
    },
  });

  const counts = useMemo(() => {
    return {
      all: scoped.length,
      queue: scoped.filter((item) => isQueued(item.status)).length,
      human: scoped.filter((item) => item.status === "human").length,
      closed: scoped.filter((item) => item.status === "closed").length,
    } satisfies Record<Tab, number>;
  }, [scoped]);

  const addNote = useMutation({
    mutationFn: ({ conversationId, text }: { conversationId: string; text: string }) =>
      api.addNote(conversationId, text),
    onSuccess: () => {
      setNoteDraft("");
      refresh();
    },
    onError: (error) => {
      toast({
        tone: "error",
        title: "Não foi possível salvar a nota",
        description: error instanceof Error ? error.message : undefined,
      });
    },
  });

  const bulk = useMutation({
    mutationFn: (input: {
      ids: string[];
      action: "assign" | "tag" | "status";
      assignedUserId?: string;
      tags?: string[];
      status?: string;
    }) => api.bulkConversations(input),
    onSuccess: (result) => {
      setSelectedIds([]);
      setBulkTag("");
      refresh();
      toast({ tone: "success", title: `${result.updated} conversa(s) atualizada(s)` });
    },
    onError: (error) => {
      toast({
        tone: "error",
        title: "Falha na ação em massa",
        description: error instanceof Error ? error.message : undefined,
      });
    },
  });

  const upload = useMutation({
    mutationFn: ({ conversationId, file }: { conversationId: string; file: File }) =>
      uploadAttachment(conversationId, file),
    onSuccess: (attachment) => {
      setPendingAttachments((current) => [...current, attachment]);
    },
    onError: (error) => {
      toast({
        tone: "error",
        title: "Falha no envio do anexo",
        description: error instanceof Error ? error.message : undefined,
      });
    },
  });

  const agentName = (userId?: string) => {
    if (!userId) return "Não atribuída";
    return agents.data?.find((agent) => agent.userId === userId)?.name ?? userId;
  };

  const channelName = (channelId: string) => {
    return channels.data?.find((channel) => channel.id === channelId)?.name ?? "Canal";
  };

  const assignToMe = () => {
    if (!selected || !session) return;
    patch.mutate({
      conversationId: selected.id,
      data: { assignedUserId: session.user.id, status: "human" },
    });
  };

  const selectConversation = (id: string) => {
    setSelectedId(id);
    setMobileDetail(true);
    setDraft("");
    setTagDraft("");
    setPendingAttachments([]);
    setShowNoteComposer(false);
    setShowReplies(false);
  };

  const submitReply = (event: React.FormEvent) => {
    event.preventDefault();
    sendNow();
  };

  const sendNow = () => {
    const text = draft.trim();
    if ((!text && pendingAttachments.length === 0) || !selectedId || selectedClosed) return;
    reply.mutate({
      conversationId: selectedId,
      text,
      attachmentIds: pendingAttachments.map((item) => item.id),
    });
    setPendingAttachments([]);
  };

  const submitNote = (event: React.FormEvent) => {
    event.preventDefault();
    const text = noteDraft.trim();
    if (!text || !selectedId) return;
    addNote.mutate({ conversationId: selectedId, text });
  };

  const handleFiles = (files: FileList | null) => {
    if (!files || files.length === 0 || !selectedId) return;
    for (const file of Array.from(files)) {
      if (file.size > 8 * 1024 * 1024) {
        toast({ tone: "error", title: `"${file.name}" passa de 8 MB`, description: "Escolha um arquivo menor." });
        continue;
      }
      upload.mutate({ conversationId: selectedId, file });
    }
  };

  const applyReply = (saved: SavedReply) => {
    setDraft((current) => (current.trim() ? `${current.trimEnd()}\n${saved.body}` : saved.body));
    setShowReplies(false);
  };

  const exportHref = api.exportConversations({
    search: search.trim() || undefined,
    channelId: channelFilter !== "all" ? channelFilter : undefined,
  });

  const handleComposerKeyDown = (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      sendNow();
    }
  };

  const addTag = (event: React.FormEvent) => {
    event.preventDefault();
    if (!selected) return;
    const tag = tagDraft.trim();
    if (!tag || selected.tags.includes(tag)) {
      setTagDraft("");
      return;
    }
    patch.mutate({
      conversationId: selected.id,
      data: { tags: [...selected.tags, tag] },
    });
    setTagDraft("");
  };

  const removeTag = (tag: string) => {
    if (!selected) return;
    patch.mutate({
      conversationId: selected.id,
      data: { tags: selected.tags.filter((item) => item !== tag) },
    });
  };

  return (
    <div className="mx-auto flex w-full max-w-6xl flex-col lg:h-[calc(100svh-6.5rem)] lg:min-h-0">
      <div className="grid min-h-0 gap-4 lg:flex-1 lg:grid-cols-[340px_minmax(0,1fr)]">
        <div
          className={cn(
            "flex-col overflow-hidden rounded-xl border border-border/70 bg-card shadow-sm lg:min-h-0",
            mobileDetail ? "hidden lg:flex" : "flex",
          )}
        >
          <div className="flex flex-col gap-3 border-b border-border/60 p-3">
            <div className="flex items-center justify-between gap-2">
              <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-foreground">
                Conversas
              </span>
              <span
                className={cn(
                  "inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-[10px] font-medium",
                  live
                    ? "border-success/30 bg-success/5 text-success"
                    : "border-border text-muted-foreground",
                )}
              >
                <Radio className={cn("size-3", !live && "opacity-50")} aria-hidden="true" />
                {live ? "Tempo real" : "Reconectando"}
              </span>
            </div>

            <div className="relative">
              <Search
                className="pointer-events-none absolute left-3 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground"
                aria-hidden="true"
              />
              <Input
                id="inbox-search"
                name="search"
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder="Buscar visitante, tag ou id"
                className="h-9 pl-9 text-sm"
                aria-label="Buscar conversas"
              />
            </div>

            <Select value={channelFilter} onValueChange={setChannelFilter}>
              <SelectTrigger className="h-9 w-full text-sm" aria-label="Filtrar por canal">
                <SelectValue placeholder="Todos os canais" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">Todos os canais</SelectItem>
                {channels.data?.map((channel) => (
                  <SelectItem key={channel.id} value={channel.id}>
                    {channel.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>

            <div className="flex flex-wrap gap-1" role="tablist" aria-label="Filtros de status">
              {tabs.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  role="tab"
                  aria-selected={tab === item.id}
                  onClick={() => setTab(item.id)}
                  className={cn(
                    "flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-[11px] font-medium transition-colors duration-fast",
                    tab === item.id
                      ? "bg-primary text-primary-foreground shadow-xs"
                      : "bg-muted/60 text-muted-foreground hover:bg-muted hover:text-foreground",
                  )}
                >
                  {item.label}
                  <span
                    className={cn(
                      "rounded-full px-1.5 text-[10px] tabular-nums",
                      tab === item.id ? "bg-primary-foreground/15" : "bg-background/70",
                    )}
                  >
                    {counts[item.id]}
                  </span>
                </button>
              ))}
            </div>
          </div>

            {selectedIds.length > 0 ? (
              <div className="flex flex-col gap-2 rounded-lg border border-border/70 bg-muted/40 p-2">
                <div className="flex items-center justify-between gap-2">
                  <span className="text-[11px] font-medium text-foreground">
                    {selectedIds.length} selecionada(s)
                  </span>
                  <button
                    type="button"
                    onClick={() => setSelectedIds([])}
                    className="text-[11px] text-muted-foreground transition-colors hover:text-foreground"
                  >
                    Limpar
                  </button>
                </div>
                <div className="flex flex-wrap gap-1">
                  <Button
                    size="sm"
                    variant="outline"
                    className="h-7 text-[11px]"
                    disabled={bulk.isPending || !session}
                    onClick={() =>
                      bulk.mutate({
                        ids: selectedIds,
                        action: "assign",
                        assignedUserId: session!.user.id,
                        status: "human",
                      })
                    }
                  >
                    <UserCheck data-icon="inline-start" />
                    Atribuir a mim
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    className="h-7 text-[11px]"
                    disabled={bulk.isPending}
                    onClick={() => bulk.mutate({ ids: selectedIds, action: "status", status: "closed" })}
                  >
                    <CheckCircle2 data-icon="inline-start" />
                    Encerrar
                  </Button>
                </div>
                <form
                  className="flex items-center gap-1"
                  onSubmit={(event) => {
                    event.preventDefault();
                    const tag = bulkTag.trim();
                    if (!tag) return;
                    bulk.mutate({ ids: selectedIds, action: "tag", tags: [tag] });
                  }}
                >
                  <Tag className="size-3 text-muted-foreground" aria-hidden="true" />
                  <input
                    value={bulkTag}
                    onChange={(event) => setBulkTag(event.target.value)}
                    placeholder="tag para todas"
                    aria-label="Tag em massa"
                    className="h-7 min-w-0 flex-1 rounded border border-transparent bg-transparent px-1 text-[11px] outline-none transition-colors placeholder:text-muted-foreground focus:border-border"
                  />
                  <Button
                    type="submit"
                    size="sm"
                    variant="outline"
                    className="h-7 text-[11px]"
                    disabled={bulk.isPending || !bulkTag.trim()}
                  >
                    <Plus data-icon="inline-start" />
                    Aplicar
                  </Button>
                </form>
              </div>
            ) : null}

            {filtered.length > 0 ? (
              <button
                type="button"
                onClick={toggleSelectAll}
                className="self-start text-[10px] text-muted-foreground transition-colors hover:text-foreground"
              >
                {selectedIds.length === filtered.length ? "Limpar seleção" : "Selecionar todas"}
              </button>
            ) : null}

          <ScrollArea className="h-[420px] scrollbar-thin lg:h-auto lg:min-h-0 lg:flex-1">
            <div className="flex flex-col gap-0.5 p-2">
              {conversations.isLoading ? (
                <ConversationListSkeleton />
              ) : filtered.length === 0 ? (
                <EmptyState
                  icon={MessageSquare}
                  title="Nenhuma conversa neste filtro"
                  description="Ajuste a busca, o canal ou o status para ver outras conversas."
                  className="mx-1 my-2 border-0 bg-transparent px-3"
                />
              ) : (
                filtered.map((conversation) => (
                  <ConversationRow
                    key={conversation.id}
                    conversation={conversation}
                    selected={conversation.id === selectedId}
                    checked={selectedIds.includes(conversation.id)}
                    channelName={channelName(conversation.channelId)}
                    onSelect={() => selectConversation(conversation.id)}
                    onToggle={() => toggleSelected(conversation.id)}
                  />
                ))
              )}
            </div>
          </ScrollArea>
        </div>

        <div
          className={cn(
            "min-h-[560px] flex-col overflow-hidden rounded-xl border border-border/70 bg-card shadow-sm lg:min-h-0",
            mobileDetail ? "flex" : "hidden lg:flex",
          )}
        >
          {selected ? (
            <>
              <div className="flex flex-col gap-3 border-b border-border/60 px-4 py-3">
                <div className="flex items-start justify-between gap-3">
                  <div className="flex min-w-0 items-center gap-2.5">
                    <Button
                      variant="ghost"
                      size="icon"
                      className="-ml-2 size-8 shrink-0 lg:hidden"
                      onClick={() => setMobileDetail(false)}
                      aria-label="Voltar para a lista de conversas"
                    >
                      <ChevronLeft />
                    </Button>
                    <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-accent text-[11px] font-semibold text-accent-foreground">
                      {initials(visitorLabel(selected.visitorId))}
                    </span>
                    <div className="min-w-0">
                      <p className="truncate text-sm font-semibold">
                        {visitorLabel(selected.visitorId)}
                      </p>
                      <p className="truncate text-xs text-muted-foreground">
                        {channelName(selected.channelId)}
                        <span className="mx-1.5 text-muted-foreground/40">·</span>
                        {relativeTime(selected.updatedAt)}
                        <span className="mx-1.5 text-muted-foreground/40">·</span>
                        {agentName(selected.assignedUserId)}
                      </p>
                    </div>
                  </div>
                  <StatusBadge status={selected.status} className="mt-0.5" />
                </div>

                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="flex flex-wrap items-center gap-1.5">
                    <Select
                      value={selected.assignedUserId || "_none"}
                      onValueChange={(value) =>
                        patch.mutate({
                          conversationId: selected.id,
                          data: { assignedUserId: value === "_none" ? "" : value },
                        })
                      }
                    >
                      <SelectTrigger className="h-8 w-44 text-xs" aria-label="Responsável">
                        <UserCheck className="size-3.5 text-muted-foreground" aria-hidden="true" />
                        <SelectValue placeholder="Responsável" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="_none">Não atribuída</SelectItem>
                        {agents.data?.map((agent) => (
                          <SelectItem key={agent.userId} value={agent.userId}>
                            {agent.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>

                    <div className="flex flex-wrap items-center gap-1">
                      {selected.tags.map((tag) => (
                        <span
                          key={tag}
                          className="flex items-center gap-1 rounded-full border border-border/70 bg-muted/60 px-2 py-0.5 text-[11px] text-muted-foreground"
                        >
                          {tag}
                          <button
                            type="button"
                            onClick={() => removeTag(tag)}
                            aria-label={`Remover tag ${tag}`}
                            className="transition-colors hover:text-destructive"
                          >
                            <X className="size-3" aria-hidden="true" />
                          </button>
                        </span>
                      ))}
                      <form onSubmit={addTag} className="flex items-center gap-1">
                        <Tag className="size-3 text-muted-foreground" aria-hidden="true" />
                        <input
                          value={tagDraft}
                          onChange={(event) => setTagDraft(event.target.value)}
                          placeholder="nova tag"
                          aria-label="Nova tag"
                          className="h-7 w-20 rounded border border-transparent bg-transparent px-1 text-[11px] outline-none transition-colors placeholder:text-muted-foreground focus:border-border"
                        />
                        <button
                          type="submit"
                          disabled={!tagDraft.trim()}
                          aria-label="Adicionar tag"
                          className="flex size-5 items-center justify-center rounded text-muted-foreground transition-colors hover:bg-accent hover:text-foreground disabled:pointer-events-none disabled:opacity-0"
                        >
                          <Plus className="size-3" aria-hidden="true" />
                        </button>
                      </form>
                    </div>
                  </div>

                  <div className="flex flex-wrap items-center gap-1.5">
                    {selected.status !== "closed" && selected.assignedUserId !== session?.user.id ? (
                      <Button size="sm" variant="outline" onClick={assignToMe} disabled={patch.isPending}>
                        <UserCheck data-icon="inline-start" />
                        Assumir
                      </Button>
                    ) : null}
                    {selected.status === "closed" ? (
                      <Button size="sm" variant="outline" onClick={assignToMe} disabled={patch.isPending}>
                        <RotateCcw data-icon="inline-start" />
                        Reabrir
                      </Button>
                    ) : (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() =>
                          patch.mutate({
                            conversationId: selected.id,
                            data: { status: "closed" },
                          })
                        }
                        disabled={patch.isPending}
                      >
                        <CheckCircle2 data-icon="inline-start" />
                        Encerrar
                      </Button>
                    )}
                    <Button size="sm" variant="ghost" className="text-muted-foreground" asChild>
                      <a href={exportHref} download>
                        <Download data-icon="inline-start" />
                        Exportar
                      </a>
                    </Button>
                  </div>
                </div>
              </div>

              <ScrollArea ref={messagesRef} className="h-[360px] flex-1 scrollbar-thin lg:h-auto lg:min-h-0">
                <div className="flex flex-col gap-5 p-4">
                  {messages.isLoading ? (
                    <MessagesSkeleton />
                  ) : messageCount === 0 ? (
                    <EmptyState
                      icon={MessageSquare}
                      title="Sem mensagens"
                      description="As mensagens desta conversa aparecem aqui."
                      className="border-0 bg-transparent"
                    />
                  ) : (
                    groupMessages(messages.data ?? []).map((group) => (
                      <div key={group.key} className="flex flex-col gap-3">
                        <div className="flex items-center gap-3">
                          <span className="h-px flex-1 bg-border/70" aria-hidden="true" />
                          <span className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                            {group.label}
                          </span>
                          <span className="h-px flex-1 bg-border/70" aria-hidden="true" />
                        </div>
                        {group.messages.map((message) => (
                          <MessageBubble key={message.id} message={message} />
                        ))}
                      </div>
                    ))
                  )}
                </div>
              </ScrollArea>

              <Separator />

              {showNoteComposer ? (
                <form
                  className="flex flex-col gap-2 border-b border-border/60 bg-warning/5 p-3"
                  onSubmit={submitNote}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="flex items-center gap-1.5 text-[11px] font-medium text-warning">
                      <StickyNote className="size-3.5" aria-hidden="true" />
                      Nota interna — só a equipe vê
                    </span>
                    <button
                      type="button"
                      onClick={() => setShowNoteComposer(false)}
                      className="text-[11px] text-muted-foreground transition-colors hover:text-foreground"
                    >
                      Cancelar
                    </button>
                  </div>
                  <Textarea
                    value={noteDraft}
                    onChange={(event) => setNoteDraft(event.target.value)}
                    placeholder="Contexto, combinados com o time, próximos passos..."
                    aria-label="Nota interna"
                    rows={2}
                    className="resize-none bg-card text-sm"
                  />
                  <div className="flex justify-end">
                    <Button
                      type="submit"
                      size="sm"
                      disabled={addNote.isPending || !noteDraft.trim()}
                    >
                      {addNote.isPending ? <Loader2 className="animate-spin" /> : null}
                      Salvar nota
                    </Button>
                  </div>
                </form>
              ) : null}

              {pendingAttachments.length > 0 ? (
                <div className="flex flex-wrap gap-2 border-b border-border/60 px-3 pt-3">
                  {pendingAttachments.map((attachment) => (
                    <span
                      key={attachment.id}
                      className="flex items-center gap-1.5 rounded-lg border border-border/70 bg-muted/50 px-2 py-1 text-[11px]"
                    >
                      <Paperclip className="size-3 shrink-0" aria-hidden="true" />
                      <span className="max-w-32 truncate">{attachment.filename}</span>
                      <button
                        type="button"
                        onClick={() =>
                          setPendingAttachments((current) =>
                            current.filter((item) => item.id !== attachment.id),
                          )
                        }
                        aria-label={`Remover anexo ${attachment.filename}`}
                        className="text-muted-foreground transition-colors hover:text-destructive"
                      >
                        <X className="size-3" aria-hidden="true" />
                      </button>
                    </span>
                  ))}
                </div>
              ) : null}

              {showReplies && replies.data && replies.data.length > 0 ? (
                <div className="max-h-48 overflow-y-auto border-b border-border/60 p-2">
                  {replies.data.map((saved) => (
                    <div
                      key={saved.id}
                      className="flex items-start gap-2 rounded-lg px-2 py-1.5 transition-colors hover:bg-accent/50"
                    >
                      <button
                        type="button"
                        className="min-w-0 flex-1 text-left"
                        onClick={() => applyReply(saved)}
                      >
                        <p className="truncate text-xs font-medium">{saved.title}</p>
                        <p className="truncate text-[11px] text-muted-foreground">{saved.body}</p>
                      </button>
                      {saved.shortcut ? (
                        <code className="shrink-0 self-center rounded bg-muted px-1 py-0.5 text-[10px] text-muted-foreground">
                          /{saved.shortcut}
                        </code>
                      ) : null}
                      {saved.tags.length > 0 && selected ? (
                        <Button
                          size="xs"
                          variant="outline"
                          className="shrink-0 self-center"
                          disabled={patch.isPending}
                          onClick={() =>
                            patch.mutate({
                              conversationId: selected.id,
                              data: {
                                tags: [
                                  ...selected.tags,
                                  ...saved.tags.filter((tag) => !selected.tags.includes(tag)),
                                ],
                              },
                            })
                          }
                        >
                          <Tag data-icon="inline-start" />
                          tags
                        </Button>
                      ) : null}
                    </div>
                  ))}
                </div>
              ) : null}

              <form className="flex flex-col gap-1.5 p-3" onSubmit={submitReply}>
                <div className="flex items-end gap-2">
                  <div className="flex items-center gap-0.5">
                    <label
                      className="flex size-9 shrink-0 cursor-pointer items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                      title="Anexar arquivo (até 8 MB)"
                    >
                      <Paperclip className="size-4" aria-hidden="true" />
                      <span className="sr-only">Anexar arquivo</span>
                      <input
                        type="file"
                        multiple
                        hidden
                        disabled={!selectedId || selectedClosed || upload.isPending}
                        onChange={(event) => {
                          handleFiles(event.target.files);
                          event.target.value = "";
                        }}
                      />
                    </label>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className={cn("size-9", showReplies && "bg-accent text-accent-foreground")}
                      onClick={() => setShowReplies((value) => !value)}
                      aria-label="Respostas prontas"
                      title="Respostas prontas"
                    >
                      <BookOpen />
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className={cn("size-9", showNoteComposer && "bg-warning/10 text-warning")}
                      onClick={() => setShowNoteComposer((value) => !value)}
                      aria-label="Nota interna"
                      title="Nota interna (só a equipe vê)"
                    >
                      <StickyNote />
                    </Button>
                  </div>
                  <Textarea
                    value={draft}
                    onChange={(event) => setDraft(event.target.value)}
                    onKeyDown={handleComposerKeyDown}
                    placeholder={
                      selectedClosed
                        ? "Conversa encerrada. Reabra para responder."
                        : "Responder como agente..."
                    }
                    aria-label="Responder como agente"
                    disabled={selectedClosed}
                    rows={1}
                    className="max-h-32 min-h-9 flex-1 resize-none py-2 text-sm"
                  />
                  <Button
                    type="submit"
                    size="icon"
                    disabled={
                      reply.isPending ||
                      (!draft.trim() && pendingAttachments.length === 0) ||
                      selectedClosed
                    }
                    aria-label="Enviar mensagem"
                  >
                    {reply.isPending ? (
                      <Loader2 className="animate-spin" />
                    ) : (
                      <SendHorizonal />
                    )}
                  </Button>
                </div>
                <p className="px-1 text-[10px] text-muted-foreground">
                  Enter envia · Shift + Enter cria uma nova linha
                </p>
              </form>
            </>
          ) : (
            <div className="flex flex-1 flex-col items-center justify-center gap-2 p-8 text-center">
              <span className="flex size-12 items-center justify-center rounded-xl border border-border/70 bg-muted/40 text-muted-foreground">
                <MessageSquare className="size-6" aria-hidden="true" />
              </span>
              <p className="mt-1 text-sm font-medium">Selecione uma conversa</p>
              <p className="max-w-xs text-xs leading-relaxed text-muted-foreground">
                As mensagens aparecem aqui em tempo real, com histórico e ações de atendimento.
              </p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function ConversationRow({
  conversation,
  selected,
  checked,
  channelName,
  onSelect,
  onToggle,
}: {
  conversation: Conversation;
  selected: boolean;
  checked: boolean;
  channelName: string;
  onSelect: () => void;
  onToggle: () => void;
}) {
  return (
    <div
      className={cn(
        "group flex w-full items-center gap-1 rounded-lg pr-2.5 transition-colors duration-fast",
        selected ? "bg-accent text-accent-foreground shadow-xs" : "hover:bg-accent/50",
      )}
    >
      <label className="flex size-8 shrink-0 cursor-pointer items-center justify-center">
        <input
          type="checkbox"
          checked={checked}
          onChange={onToggle}
          aria-label={`Selecionar conversa de ${visitorLabel(conversation.visitorId)}`}
          className="size-3.5 accent-[hsl(var(--primary))]"
        />
      </label>
      <button
        type="button"
        onClick={onSelect}
        className="flex min-w-0 flex-1 items-center gap-2.5 py-2.5 text-left"
      >
        <span
          className={cn(
            "flex size-8 shrink-0 items-center justify-center rounded-full text-[10px] font-semibold",
            selected ? "bg-primary text-primary-foreground" : "bg-muted text-muted-foreground",
          )}
        >
          {initials(visitorLabel(conversation.visitorId))}
        </span>
        <span className="min-w-0 flex-1">
          <span className="flex items-center justify-between gap-2">
            <span className="truncate text-sm font-medium">
              {visitorLabel(conversation.visitorId)}
            </span>
            <span className="shrink-0 text-[10px] tabular-nums text-muted-foreground">
              {relativeTime(conversation.updatedAt)}
            </span>
          </span>
          <span className="mt-1 flex items-center justify-between gap-2">
            <span className="min-w-0 truncate text-[11px] text-muted-foreground">
              {channelName}
              {conversation.tags.length > 0 ? ` · ${conversation.tags.join(" · ")}` : ""}
            </span>
            <StatusBadge status={conversation.status} className="px-1.5 py-0 text-[10px]" />
          </span>
        </span>
      </button>
    </div>
  );
}

function groupMessages(messages: Message[]) {
  const groups: { key: string; label: string; messages: Message[] }[] = [];
  for (const message of messages) {
    const key = dayKey(message.createdAt);
    const last = groups.at(-1);
    if (last?.key === key) {
      last.messages.push(message);
    } else {
      groups.push({ key, label: dayHeading(message.createdAt), messages: [message] });
    }
  }
  return groups;
}

function MessageBubble({ message }: { message: Message }) {
  if (message.role === "system") {
    return (
      <p className="mx-auto max-w-[85%] rounded-full border border-dashed border-border/80 bg-muted/30 px-3 py-1 text-center text-[11px] text-muted-foreground">
        {message.text}
      </p>
    );
  }

  // Nota interna: só a equipe vê. Fica fora do fluxo de bolhas do chat.
  if (message.role === "note") {
    return (
      <div className="mx-auto max-w-[92%] rounded-xl border border-dashed border-warning/40 bg-warning/5 px-3 py-2">
        <span className="flex items-center gap-1.5 text-[10px] font-medium uppercase tracking-wide text-warning">
          <StickyNote className="size-3" aria-hidden="true" />
          Nota interna{message.authorName ? ` · ${message.authorName}` : ""}
          <span aria-hidden="true">·</span>
          {formatTime(message.createdAt)}
        </span>
        <p className="mt-1 whitespace-pre-wrap break-words text-sm leading-relaxed text-foreground">
          {message.text}
        </p>
      </div>
    );
  }

  const isAgent = message.role === "agent";
  const Icon = message.role === "bot" ? Bot : UserRound;

  return (
    <div className={cn("flex max-w-[85%] gap-2 sm:max-w-[76%]", isAgent ? "ml-auto flex-row-reverse" : "self-start")}>
      <span
        className={cn(
          "mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-full",
          isAgent
            ? "bg-primary text-primary-foreground"
            : message.role === "bot"
              ? "bg-primary/10 text-primary"
              : "bg-muted text-muted-foreground",
        )}
        aria-hidden="true"
      >
        {isAgent ? <UserCheck className="size-3.5" /> : <Icon className="size-3.5" />}
      </span>
      <div
        className={cn(
          "flex min-w-0 flex-col gap-1 rounded-2xl px-3 py-2 text-sm",
          isAgent
            ? "rounded-tr-sm bg-primary text-primary-foreground"
            : message.role === "bot"
              ? "rounded-tl-sm border border-border/70 bg-card"
              : "rounded-tl-sm bg-muted",
        )}
      >
        <span
          className={cn(
            "flex items-center gap-1.5 text-[10px] font-medium uppercase tracking-wide",
            isAgent ? "text-primary-foreground/70" : "text-muted-foreground",
          )}
        >
          {messageRoleLabel[message.role as MessageRole]}
          <span aria-hidden="true">·</span>
          {formatTime(message.createdAt)}
        </span>
        <p className="whitespace-pre-wrap break-words text-sm leading-relaxed">{message.text}</p>
        {message.attachments && message.attachments.length > 0 ? (
          <MessageAttachments attachments={message.attachments} muted={isAgent} />
        ) : null}
      </div>
    </div>
  );
}

function MessageAttachments({ attachments, muted }: { attachments: Attachment[]; muted?: boolean }) {
  return (
    <div className="mt-1.5 flex flex-col gap-2">
      {attachments.map((attachment) => {
        const url = attachmentUrl(attachment.id);

        if (attachment.kind === "image") {
          return (
            <a
              key={attachment.id}
              href={url}
              target="_blank"
              rel="noreferrer"
              className="block overflow-hidden rounded-lg border border-border/70"
            >
              <img
                src={url}
                alt={attachment.filename}
                loading="lazy"
                className="max-h-48 w-full object-cover"
              />
            </a>
          );
        }

        if (attachment.kind === "audio") {
          return (
            <audio key={attachment.id} controls src={url} preload="none" className="w-full" />
          );
        }

        return (
          <a
            key={attachment.id}
            href={url}
            download={attachment.filename}
            className={cn(
              "flex items-center gap-2 rounded-lg border px-2 py-1.5 text-xs",
              muted
                ? "border-primary-foreground/25 text-primary-foreground"
                : "border-border/70 text-foreground",
            )}
          >
            <Paperclip className="size-3.5 shrink-0" aria-hidden="true" />
            <span className="min-w-0 flex-1 truncate">{attachment.filename}</span>
            <span className={cn("shrink-0", muted ? "text-primary-foreground/70" : "text-muted-foreground")}>
              {formatBytes(attachment.sizeBytes)}
            </span>
          </a>
        );
      })}
    </div>
  );
}

function ConversationListSkeleton() {
  return (
    <div className="flex flex-col gap-2 p-1">
      {[0, 1, 2, 3, 4, 5].map((item) => (
        <div key={item} className="flex items-center gap-2.5 px-2 py-2">
          <Skeleton className="size-8 rounded-full" />
          <div className="flex-1 space-y-1.5">
            <Skeleton className="h-3.5 w-28" />
            <Skeleton className="h-3 w-20" />
          </div>
          <Skeleton className="h-4 w-14 rounded-full" />
        </div>
      ))}
    </div>
  );
}

function MessagesSkeleton() {
  return (
    <div className="flex flex-col gap-4">
      {[0, 1, 2, 3].map((item) => (
        <div key={item} className={cn("flex gap-2", item % 2 === 1 ? "ml-auto flex-row-reverse" : "")}>
          <Skeleton className="size-7 shrink-0 rounded-full" />
          <Skeleton className={cn("h-16 rounded-2xl", item % 2 === 1 ? "w-48" : "w-64")} />
        </div>
      ))}
    </div>
  );
}
