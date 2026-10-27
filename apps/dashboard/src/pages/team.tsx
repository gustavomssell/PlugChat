import { useEffect, useState } from "react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { RefreshCw, UserPlus, Users } from "lucide-react";
import { z } from "zod";
import type { Availability, MemberRole } from "@chatfree/shared";
import { useAuth } from "@/lib/auth";
import { api, streamUrl } from "@/lib/api";
import { availabilityDot, availabilityLabel, initials, memberRoleLabel } from "@/lib/labels";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { useToast } from "@/components/ui/toast";
import { PageHeader } from "@/components/page-header";
import { EmptyState } from "@/components/empty-state";

const agentSchema = z.object({
  name: z.string().min(2, "Informe o nome"),
  email: z.string().email("Informe um e-mail válido"),
  password: z.string().min(8, "Use pelo menos 8 caracteres"),
});

type AgentInput = z.infer<typeof agentSchema>;

const roleTone: Record<MemberRole, string> = {
  owner: "border-primary/25 bg-primary/10 text-primary",
  admin: "border-info/25 bg-info/10 text-info",
  agent: "border-border bg-muted text-muted-foreground",
};

export function TeamPage() {
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const { session } = useAuth();
  const [formError, setFormError] = useState<string | null>(null);
  const canManage = session?.membership.role === "owner" || session?.membership.role === "admin";

  const agents = useQuery({ queryKey: ["agents"], queryFn: api.listAgents });

  useEffect(() => {
    const source = new EventSource(streamUrl("/api/events"));
    source.addEventListener("agent", () => {
      void queryClient.invalidateQueries({ queryKey: ["agents"] });
    });
    return () => source.close();
  }, [queryClient]);

  const form = useForm<AgentInput>({
    resolver: zodResolver(agentSchema),
    defaultValues: { name: "", email: "", password: "" },
  });

  const create = useMutation({
    mutationFn: (values: AgentInput) => api.createAgent(values),
    onSuccess: (agent) => {
      form.reset();
      setFormError(null);
      toast({
        tone: "success",
        title: "Agente adicionado",
        description: `${agent.name} entra offline e pode ativar a disponibilidade ao entrar.`,
      });
      void queryClient.invalidateQueries({ queryKey: ["agents"] });
    },
    onError: (error) => {
      setFormError(error instanceof Error ? error.message : "Não foi possível adicionar.");
    },
  });

  const activeCount = agents.data?.filter((agent) => agent.availability !== "offline").length ?? 0;

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-6">
      <PageHeader
        eyebrow="Workspace"
        title="Equipe"
        description="Pessoas que podem assumir conversas neste workspace."
        actions={
          <Badge variant="outline" className="gap-1.5 px-2 text-[11px] text-muted-foreground">
            <span className="size-1.5 rounded-full bg-success" aria-hidden="true" />
            {activeCount} {activeCount === 1 ? "pessoa online" : "pessoas online"}
          </Badge>
        }
      />

      <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_360px]">
        <Card>
          <CardHeader className="border-b border-border/60 pb-4">
            <div className="flex items-center justify-between gap-3">
              <div>
                <CardTitle className="text-base">Membros</CardTitle>
                <CardDescription>
                  {agents.data?.length ?? 0} {(agents.data?.length ?? 0) === 1 ? "pessoa" : "pessoas"} no
                  workspace.
                </CardDescription>
              </div>
              <span className="flex size-9 items-center justify-center rounded-lg bg-accent text-accent-foreground">
                <Users className="size-4" aria-hidden="true" />
              </span>
            </div>
          </CardHeader>
          <CardContent className="pt-4">
            {agents.isLoading ? (
              <MembersSkeleton />
            ) : agents.isError ? (
              <div className="flex flex-col items-center gap-3 py-8 text-center">
                <p className="text-sm font-medium text-destructive">Não foi possível carregar a equipe.</p>
                <Button variant="outline" size="sm" onClick={() => void agents.refetch()}>
                  <RefreshCw data-icon="inline-start" />
                  Tentar novamente
                </Button>
              </div>
            ) : (agents.data?.length ?? 0) === 0 ? (
              <EmptyState
                icon={Users}
                title="Nenhum membro encontrado"
                description="Adicione o primeiro agente ao lado para começar a atender."
                className="border-0 bg-transparent"
              />
            ) : (
              <div className="flex flex-col gap-2">
                {agents.data?.map((agent) => (
                  <div
                    key={agent.userId}
                    className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border/60 bg-background/40 px-3 py-2.5 transition-colors duration-fast hover:bg-accent/40"
                  >
                    <div className="flex min-w-0 items-center gap-2.5">
                      <span className="relative flex size-9 shrink-0 items-center justify-center rounded-full bg-primary/10 text-xs font-semibold text-primary">
                        {initials(agent.name)}
                        <span
                          className={cn(
                            "absolute -bottom-0.5 -right-0.5 size-2.5 rounded-full border-2 border-card",
                            availabilityDot[agent.availability as Availability],
                          )}
                          aria-hidden="true"
                        />
                      </span>
                      <div className="min-w-0">
                        <p className="flex items-center gap-1.5 truncate text-sm font-medium">
                          {agent.name}
                          {agent.userId === session?.user.id ? (
                            <span className="rounded bg-muted px-1 py-0.5 text-[10px] font-medium text-muted-foreground">
                              você
                            </span>
                          ) : null}
                        </p>
                        <p className="truncate text-xs text-muted-foreground">{agent.email}</p>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
                        <span
                          className={cn("size-1.5 rounded-full", availabilityDot[agent.availability])}
                          aria-hidden="true"
                        />
                        {availabilityLabel[agent.availability]}
                      </span>
                      <Badge
                        variant="outline"
                        className={cn("text-[10px]", roleTone[agent.role])}
                      >
                        {memberRoleLabel[agent.role]}
                      </Badge>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="border-b border-border/60 pb-4">
            <CardTitle className="text-base">Adicionar agente</CardTitle>
            <CardDescription>
              {canManage
                ? "O novo membro entra offline e pode ativar a disponibilidade ao entrar."
                : "Somente proprietários e administradores podem adicionar membros."}
            </CardDescription>
          </CardHeader>
          <CardContent className="pt-4">
            <form
              className="flex flex-col gap-4"
              onSubmit={form.handleSubmit((values) => create.mutate(values))}
            >
              <FieldGroup>
                <Field data-invalid={Boolean(form.formState.errors.name)}>
                  <FieldLabel htmlFor="agent-name">Nome</FieldLabel>
                  <Input
                    id="agent-name"
                    autoComplete="name"
                    placeholder="Maria Silva"
                    disabled={!canManage}
                    aria-invalid={Boolean(form.formState.errors.name)}
                    {...form.register("name")}
                  />
                  <FieldError errors={[form.formState.errors.name]} />
                </Field>
                <Field data-invalid={Boolean(form.formState.errors.email)}>
                  <FieldLabel htmlFor="agent-email">E-mail</FieldLabel>
                  <Input
                    id="agent-email"
                    type="email"
                    autoComplete="email"
                    placeholder="maria@empresa.com"
                    disabled={!canManage}
                    aria-invalid={Boolean(form.formState.errors.email)}
                    {...form.register("email")}
                  />
                  <FieldError errors={[form.formState.errors.email]} />
                </Field>
                <Field data-invalid={Boolean(form.formState.errors.password)}>
                  <FieldLabel htmlFor="agent-password">Senha inicial</FieldLabel>
                  <Input
                    id="agent-password"
                    type="password"
                    autoComplete="new-password"
                    placeholder="Mínimo de 8 caracteres"
                    disabled={!canManage}
                    aria-invalid={Boolean(form.formState.errors.password)}
                    {...form.register("password")}
                  />
                  <FieldError errors={[form.formState.errors.password]} />
                </Field>
              </FieldGroup>

              {formError ? (
                <p role="alert" className="text-xs text-destructive">
                  {formError}
                </p>
              ) : null}

              <Button type="submit" disabled={!canManage || create.isPending}>
                <UserPlus data-icon="inline-start" />
                {create.isPending ? "Adicionando..." : "Adicionar à equipe"}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function MembersSkeleton() {
  return (
    <div className="flex flex-col gap-2">
      {[0, 1, 2].map((item) => (
        <div
          key={item}
          className="flex items-center justify-between gap-3 rounded-lg border border-border/60 px-3 py-2.5"
        >
          <div className="flex items-center gap-2.5">
            <Skeleton className="size-9 rounded-full" />
            <div className="space-y-1.5">
              <Skeleton className="h-3.5 w-32" />
              <Skeleton className="h-3 w-40" />
            </div>
          </div>
          <Skeleton className="h-5 w-20 rounded-full" />
        </div>
      ))}
    </div>
  );
}
