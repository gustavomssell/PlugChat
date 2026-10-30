import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { BookOpen, Loader2, Pencil, Plus, Tag, Trash2 } from "lucide-react";
import type { SavedReply } from "@chatfree/shared";
import { useAuth } from "@/lib/auth";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Skeleton } from "@/components/ui/skeleton";
import { PageHeader } from "@/components/page-header";
import { EmptyState } from "@/components/empty-state";
import { useToast } from "@/components/ui/toast";

type Draft = {
  id: string | null;
  title: string;
  body: string;
  shortcut: string;
  tags: string;
};

const emptyDraft: Draft = { id: null, title: "", body: "", shortcut: "", tags: "" };

function parseTags(value: string) {
  return value
    .split(/[,;]/)
    .map((tag) => tag.trim())
    .filter((tag) => tag !== "");
}

export function RepliesPage() {
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const { session } = useAuth();
  // O backend exige owner/admin na escrita. Sem esse guarda a UI mostrava um
  // formulário que só sabia responder 403 para atendentes.
  const canManage = session?.membership.role === "owner" || session?.membership.role === "admin";

  const replies = useQuery({ queryKey: ["replies"], queryFn: api.listReplies });
  const [draft, setDraft] = useState<Draft>(emptyDraft);

  useEffect(() => {
    if (draft.id && replies.data && !replies.data.some((item) => item.id === draft.id)) {
      setDraft(emptyDraft);
    }
  }, [draft.id, replies.data]);

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: ["replies"] });
  };

  const save = useMutation({
    mutationFn: (input: Draft) => {
      const payload = {
        title: input.title,
        body: input.body,
        shortcut: input.shortcut,
        tags: parseTags(input.tags),
      };
      return input.id ? api.updateReply(input.id, payload) : api.createReply(payload);
    },
    onSuccess: () => {
      setDraft(emptyDraft);
      invalidate();
      toast({ tone: "success", title: "Resposta pronta salva" });
    },
    onError: (error) => {
      toast({
        tone: "error",
        title: "Não foi possível salvar",
        description: error instanceof Error ? error.message : undefined,
      });
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => api.deleteReply(id),
    onSuccess: () => {
      invalidate();
      toast({ tone: "success", title: "Resposta pronta removida" });
    },
    onError: (error) => {
      toast({
        tone: "error",
        title: "Não foi possível remover",
        description: error instanceof Error ? error.message : undefined,
      });
    },
  });

  const edit = (reply: SavedReply) => {
    setDraft({
      id: reply.id,
      title: reply.title,
      body: reply.body,
      shortcut: reply.shortcut,
      tags: reply.tags.join(", "),
    });
  };

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-6">
      <PageHeader
        title="Respostas prontas"
        description="Textos e macros que a equipe insere com um clique na inbox. Atalhos são opcionais e únicos."
      />

      <div className="grid items-start gap-4 lg:grid-cols-[minmax(0,360px)_minmax(0,1fr)]">
        <Card>
          <CardHeader className="border-b border-border/60 pb-4">
            <CardTitle className="text-base">
              {draft.id ? "Editar resposta" : "Nova resposta"}
            </CardTitle>
            <CardDescription>
              {draft.id
                ? "Altere o texto e salve para atualizar em toda a equipe."
                : "Fica disponível no compositor da inbox, com atalho e tags opcionais."}
            </CardDescription>
            {!canManage ? (
              <p className="mt-2 rounded-lg border border-warning/30 bg-warning/5 px-3 py-2 text-xs text-warning">
                Somente proprietários e administradores podem criar ou editar respostas prontas.
              </p>
            ) : null}
          </CardHeader>
          <CardContent className="pt-4">
            <form
              className="flex flex-col gap-3"
              onSubmit={(event) => {
                event.preventDefault();
                save.mutate(draft);
              }}
            >
              <div className="flex flex-col gap-1.5">
                <label htmlFor="reply-title" className="text-xs font-medium text-foreground">
                  Título
                </label>
                <Input
                  id="reply-title"
                  value={draft.title}
                  onChange={(event) => setDraft({ ...draft, title: event.target.value })}
                  placeholder="Horário de atendimento"
                  maxLength={80}
                  disabled={!canManage}
                  required
                />
              </div>

              <div className="flex flex-col gap-1.5">
                <label htmlFor="reply-body" className="text-xs font-medium text-foreground">
                  Corpo
                </label>
                <Textarea
                  id="reply-body"
                  value={draft.body}
                  onChange={(event) => setDraft({ ...draft, body: event.target.value })}
                  placeholder="Atendemos de segunda a sexta, das 9h às 18h."
                  rows={4}
                  className="resize-none text-sm"
                  disabled={!canManage}
                  required
                />
              </div>

              <div className="grid gap-3 sm:grid-cols-2">
                <div className="flex flex-col gap-1.5">
                  <label htmlFor="reply-shortcut" className="text-xs font-medium text-foreground">
                    Atalho (opcional)
                  </label>
                  <div className="flex items-center gap-1.5">
                    <span className="text-sm text-muted-foreground">/</span>
                    <Input
                      id="reply-shortcut"
                      value={draft.shortcut}
                      onChange={(event) => setDraft({ ...draft, shortcut: event.target.value })}
                      placeholder="horario"
                      pattern="[a-z0-9][a-z0-9-]*"
                      maxLength={32}
                      disabled={!canManage}
                    />
                  </div>
                </div>

                <div className="flex flex-col gap-1.5">
                  <label htmlFor="reply-tags" className="text-xs font-medium text-foreground">
                    Tags (opcional)
                  </label>
                  <Input
                    id="reply-tags"
                    value={draft.tags}
                    onChange={(event) => setDraft({ ...draft, tags: event.target.value })}
                    placeholder="horario, comercial"
                    disabled={!canManage}
                  />
                </div>
              </div>

              <p className="text-[11px] leading-relaxed text-muted-foreground">
                Atalho usa letras minúsculas, números e hífen. Tags podem ser aplicadas à conversa
                direto do seletor na inbox.
              </p>

              <div className="flex items-center gap-2">
                <Button type="submit" disabled={!canManage || save.isPending}>
                  {save.isPending ? <Loader2 className="animate-spin" /> : <Plus data-icon="inline-start" />}
                  {draft.id ? "Salvar alterações" : "Criar resposta"}
                </Button>
                {draft.id ? (
                  <Button type="button" variant="ghost" onClick={() => setDraft(emptyDraft)}>
                    Cancelar
                  </Button>
                ) : null}
              </div>
            </form>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="border-b border-border/60 pb-4">
            <div className="flex items-center justify-between gap-3">
              <div>
                <CardTitle className="text-base">Biblioteca do workspace</CardTitle>
                <CardDescription>
                  {replies.data?.length ?? 0} resposta(s) disponível(is) para a equipe.
                </CardDescription>
              </div>
              <Badge variant="outline" className="shrink-0 text-[10px] text-muted-foreground">
                máx. 200
              </Badge>
            </div>
          </CardHeader>
          <CardContent className="flex flex-col gap-2 pt-4">
            {replies.isLoading ? (
              <div className="flex flex-col gap-2">
                {[0, 1, 2].map((item) => (
                  <Skeleton key={item} className="h-20 w-full" />
                ))}
              </div>
            ) : (replies.data?.length ?? 0) === 0 ? (
              <EmptyState
                icon={BookOpen}
                title="Nenhuma resposta pronta"
                description="Crie a primeira ao lado: ela aparece no compositor da inbox com um clique."
                className="border-0 bg-transparent py-8"
              />
            ) : (
              replies.data?.map((reply) => (
                <div
                  key={reply.id}
                  className={cn(
                    "flex flex-col gap-2 rounded-lg border p-3 transition-colors",
                    draft.id === reply.id
                      ? "border-primary/40 bg-accent/40"
                      : "border-border/60 hover:bg-accent/30",
                  )}
                >
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">{reply.title}</p>
                      {reply.shortcut ? (
                        <code className="text-[11px] text-muted-foreground">/{reply.shortcut}</code>
                      ) : null}
                    </div>
                    <div className="flex shrink-0 items-center gap-1">
                      {canManage ? (
                        <>
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            onClick={() => edit(reply)}
                            aria-label={`Editar ${reply.title}`}
                          >
                            <Pencil />
                          </Button>
                          <ConfirmDialog
                            trigger={
                              <Button
                                variant="ghost"
                                size="icon-sm"
                                className="hover:text-destructive"
                                aria-label={`Remover ${reply.title}`}
                              >
                                <Trash2 />
                              </Button>
                            }
                            title="Remover resposta pronta?"
                            description={`"${reply.title}" deixa de aparecer no compositor da inbox.`}
                            confirmLabel="Remover"
                            onConfirm={() => remove.mutate(reply.id)}
                          />
                        </>
                      ) : null}
                    </div>
                  </div>
                  <p className="whitespace-pre-wrap text-xs leading-relaxed text-muted-foreground">
                    {reply.body}
                  </p>
                  {reply.tags.length > 0 ? (
                    <div className="flex flex-wrap items-center gap-1">
                      <Tag className="size-3 text-muted-foreground" aria-hidden="true" />
                      {reply.tags.map((tag) => (
                        <Badge key={tag} variant="secondary" className="px-1.5 py-0 text-[10px]">
                          {tag}
                        </Badge>
                      ))}
                    </div>
                  ) : null}
                </div>
              ))
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
