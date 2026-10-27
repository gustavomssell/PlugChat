import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { ArrowUpRight, Plus, RefreshCw, Trash2, Workflow } from "lucide-react";
import { useAuth } from "@/lib/auth";
import { api } from "@/lib/api";
import { relativeTime } from "@/lib/labels";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/toast";
import { PageHeader } from "@/components/page-header";
import { EmptyState } from "@/components/empty-state";

export function FlowsPage() {
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const { session } = useAuth();
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const canManage = session?.membership.role === "owner" || session?.membership.role === "admin";

  const flows = useQuery({ queryKey: ["flows"], queryFn: api.listFlows });

  const create = useMutation({
    mutationFn: api.createFlow,
    onSuccess: (flow) => {
      setName("");
      setError(null);
      toast({ tone: "success", title: "Fluxo criado", description: flow.name });
      void queryClient.invalidateQueries({ queryKey: ["flows"] });
    },
    onError: (value) => {
      setError(value instanceof Error ? value.message : "Não foi possível criar o fluxo.");
    },
  });

  const remove = useMutation({
    mutationFn: api.deleteFlow,
    onSuccess: () => {
      setError(null);
      toast({ tone: "success", title: "Fluxo removido" });
      void queryClient.invalidateQueries({ queryKey: ["flows"] });
      void queryClient.invalidateQueries({ queryKey: ["channels"] });
    },
    onError: (value) => {
      setError(value instanceof Error ? value.message : "Não foi possível remover o fluxo.");
    },
  });

  const submit = (event: React.FormEvent) => {
    event.preventDefault();
    const value = name.trim();
    if (!value) return;
    create.mutate(value);
  };

  return (
    <div className="mx-auto flex w-full max-w-6xl flex-col gap-6">
      <PageHeader
        eyebrow="Automação"
        title="Fluxos"
        description="Automações que conduzem a conversa antes do atendimento humano."
        actions={
          canManage ? (
            <form className="flex w-full gap-2 sm:w-auto" onSubmit={submit}>
              <Input
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder="Nome do novo fluxo"
                className="h-9 sm:w-56"
                aria-label="Nome do novo fluxo"
              />
              <Button type="submit" disabled={create.isPending || !name.trim()}>
                <Plus data-icon="inline-start" />
                {create.isPending ? "Criando..." : "Criar"}
              </Button>
            </form>
          ) : undefined
        }
      />

      {error ? (
        <p role="alert" className="rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
          {error}
        </p>
      ) : null}

      {flows.isLoading ? (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {[0, 1, 2].map((item) => (
            <Card key={item}>
              <CardContent className="space-y-4 p-5">
                <Skeleton className="size-10 rounded-lg" />
                <div className="space-y-2">
                  <Skeleton className="h-4 w-40" />
                  <Skeleton className="h-3 w-28" />
                </div>
                <Skeleton className="h-5 w-16 rounded-full" />
              </CardContent>
            </Card>
          ))}
        </div>
      ) : flows.isError ? (
        <div className="flex flex-col items-center gap-3 rounded-xl border border-destructive/30 bg-destructive/5 px-6 py-10 text-center">
          <p className="text-sm font-medium text-destructive">Não foi possível carregar os fluxos.</p>
          <p className="text-xs text-muted-foreground">Verifique se a API está rodando e tente novamente.</p>
          <Button variant="outline" size="sm" onClick={() => void flows.refetch()}>
            <RefreshCw data-icon="inline-start" />
            Tentar novamente
          </Button>
        </div>
      ) : (flows.data?.length ?? 0) === 0 ? (
        <EmptyState
          icon={Workflow}
          title="Nenhum fluxo ainda"
          description="Todo fluxo começa com um gatilho, uma saudação e um encerramento. Crie o primeiro para automatizar a triagem."
          action={
            canManage ? (
              <Button size="sm" onClick={() => document.querySelector<HTMLInputElement>('input[aria-label="Nome do novo fluxo"]')?.focus()}>
                <Plus data-icon="inline-start" />
                Criar primeiro fluxo
              </Button>
            ) : undefined
          }
        />
      ) : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {flows.data?.map((flow) => (
            <Card
              key={flow.id}
              className="group relative h-full overflow-hidden transition-colors duration-fast hover:border-primary/35 hover:shadow-md"
            >
              <Link
                to="/flows/$flowId"
                params={{ flowId: flow.id }}
                className="absolute inset-0 z-0 rounded-xl"
                aria-label={`Abrir fluxo ${flow.name}`}
              />
              <CardHeader className="flex flex-row items-start justify-between gap-3 pb-0">
                <span className="flex size-10 items-center justify-center rounded-lg bg-accent text-accent-foreground">
                  <Workflow className="size-5" aria-hidden="true" />
                </span>
                <div className="flex items-center gap-1">
                  {canManage ? (
                    <ConfirmDialog
                      title={`Remover o fluxo "${flow.name}"?`}
                      description="Canais que usam este fluxo podem ficar sem automação. Esta ação não pode ser desfeita."
                      confirmLabel="Remover fluxo"
                      onConfirm={() => remove.mutate(flow.id)}
                      trigger={
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          className="relative z-10 size-7 text-muted-foreground hover:text-destructive"
                          aria-label={`Remover fluxo ${flow.name}`}
                        >
                          <Trash2 className="size-3.5" />
                        </Button>
                      }
                    />
                  ) : null}
                  <ArrowUpRight
                    className="size-4 text-muted-foreground transition-transform duration-fast group-hover:-translate-y-0.5 group-hover:translate-x-0.5"
                    aria-hidden="true"
                  />
                </div>
              </CardHeader>
              <CardContent className="flex flex-col gap-2 pt-5">
                <CardTitle className="text-base">{flow.name}</CardTitle>
                <p className="text-xs text-muted-foreground">
                  {flow.nodes} {flow.nodes === 1 ? "node" : "nodes"}
                  <span className="mx-1.5 text-muted-foreground/40">·</span>
                  atualizado {relativeTime(flow.updatedAt)}
                </p>
                <Badge variant="outline" className="mt-1 w-fit text-[10px]">
                  versão {flow.version}
                </Badge>
              </CardContent>
            </Card>
          ))}

          {canManage ? (
            <button
              type="button"
              onClick={() => document.querySelector<HTMLInputElement>('input[aria-label="Nome do novo fluxo"]')?.focus()}
              className="flex min-h-[182px] flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-border/80 bg-transparent p-6 text-center transition-colors duration-fast hover:border-primary/35 hover:bg-accent/30"
            >
              <span className="flex size-10 items-center justify-center rounded-lg border border-dashed border-border/80 text-muted-foreground">
                <Plus className="size-5" aria-hidden="true" />
              </span>
              <span className="text-sm font-medium">Criar um novo fluxo</span>
              <span className="max-w-[220px] text-xs leading-relaxed text-muted-foreground">
                Dê um nome acima e comece a desenhar a automação.
              </span>
            </button>
          ) : null}
        </div>
      )}
    </div>
  );
}
