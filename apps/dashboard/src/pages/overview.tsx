import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import {
  ArrowUpRight,
  Inbox,
  MessageCircle,
  MessagesSquare,
  RefreshCw,
  Timer,
  Workflow,
} from "lucide-react";
import type { Conversation } from "@chatfree/shared";
import { useAuth } from "@/lib/auth";
import { api } from "@/lib/api";
import { initials, relativeTime, visitorLabel } from "@/lib/labels";
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
import { Skeleton } from "@/components/ui/skeleton";
import { PageHeader } from "@/components/page-header";
import { EmptyState } from "@/components/empty-state";
import { StatusBadge } from "@/components/status-badge";
import { ConversationsChart } from "@/components/conversations-chart";

const statMeta = {
  total: {
    label: "Conversas",
    hint: "Total histórico",
    icon: MessagesSquare,
    tone: "bg-primary/10 text-primary",
  },
  active: {
    label: "Em andamento",
    hint: "Bot ou humano",
    icon: MessageCircle,
    tone: "bg-success/10 text-success",
  },
  waiting: {
    label: "Aguardando",
    hint: "Fila de atendimento",
    icon: Timer,
    tone: "bg-warning/10 text-warning",
  },
  messagesToday: {
    label: "Mensagens hoje",
    hint: "Todas as conversas",
    icon: Inbox,
    tone: "bg-info/10 text-info",
  },
} as const;

type StatKey = keyof typeof statMeta;

export function OverviewPage() {
  const { session } = useAuth();

  const stats = useQuery({ queryKey: ["stats"], queryFn: api.stats });
  const conversations = useQuery({
    queryKey: ["conversations"],
    queryFn: () => api.listConversations(),
  });
  const flows = useQuery({ queryKey: ["flows"], queryFn: api.listFlows });
  const channels = useQuery({ queryKey: ["channels"], queryFn: api.listChannels });

  const data = stats.data;
  const channelName = (channelId: string) =>
    channels.data?.find((channel) => channel.id === channelId)?.name ?? "Canal";

  const metrics: { key: StatKey; value: number }[] = [
    { key: "total", value: data?.total ?? 0 },
    { key: "active", value: data?.active ?? 0 },
    { key: "waiting", value: data?.waiting ?? 0 },
    { key: "messagesToday", value: data?.messagesToday ?? 0 },
  ];

  return (
    <div className="mx-auto flex w-full max-w-6xl flex-col gap-6">
      <PageHeader
        eyebrow={session?.workspace.name}
        title="Visão geral"
        description="O pulso do atendimento: conversas, fila e automações em um só lugar."
        actions={
          <>
            <Button asChild variant="outline">
              <Link to="/flows">
                <Workflow data-icon="inline-start" />
                Editar fluxos
              </Link>
            </Button>
            <Button asChild>
              <Link to="/inbox">
                <Inbox data-icon="inline-start" />
                Abrir atendimento
              </Link>
            </Button>
          </>
        }
      />

      {stats.isError ? (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          <span>Não foi possível carregar as métricas. A API está rodando?</span>
          <Button variant="outline" size="sm" onClick={() => void stats.refetch()}>
            <RefreshCw data-icon="inline-start" />
            Tentar novamente
          </Button>
        </div>
      ) : null}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {metrics.map(({ key, value }) => (
          <StatCard key={key} metaKey={key} value={value} loading={stats.isLoading} />
        ))}
      </div>

      <div className="grid items-start gap-4 lg:grid-cols-[1.08fr_1fr]">
        <Card>
          <CardHeader className="border-b border-border/60 pb-4">
            <div className="flex items-center justify-between gap-3">
              <div>
                <CardTitle className="text-base">Conversas nos últimos 7 dias</CardTitle>
                <CardDescription>Novas sessões iniciadas pelo widget.</CardDescription>
              </div>
              <Badge variant="outline" className="shrink-0 text-[10px] font-medium text-muted-foreground">
                7 dias
              </Badge>
            </div>
          </CardHeader>
          <CardContent className="pt-4">
            {stats.isLoading ? (
              <Skeleton className="h-56 w-full" />
            ) : (
              <ConversationsChart data={data?.conversationsByDay ?? []} />
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="border-b border-border/60 pb-4">
            <div className="flex items-center justify-between gap-3">
              <div>
                <CardTitle className="text-base">Conversas recentes</CardTitle>
                <CardDescription>Últimas sessões atualizadas.</CardDescription>
              </div>
              <Button asChild variant="ghost" size="sm" className="text-xs">
                <Link to="/inbox">Ver todas</Link>
              </Button>
            </div>
          </CardHeader>
          <CardContent className="flex flex-col gap-1.5 pt-3">
            {conversations.isLoading ? (
              <RecentListSkeleton />
            ) : (conversations.data?.length ?? 0) === 0 ? (
              <EmptyState
                icon={MessageCircle}
                title="Nenhuma conversa ainda"
                description="Abra o playground do widget e inicie um atendimento para ver as conversas aqui."
                className="border-0 bg-transparent px-2 py-8"
              />
            ) : (
              conversations.data?.slice(0, 5).map((conversation) => (
                <RecentConversation
                  key={conversation.id}
                  conversation={conversation}
                  channelName={channelName(conversation.channelId)}
                />
              ))
            )}
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader className="border-b border-border/60 pb-4">
          <div className="flex items-center justify-between gap-3">
            <div>
              <CardTitle className="text-base">Fluxos publicados</CardTitle>
              <CardDescription>Lógicas de automação disponíveis.</CardDescription>
            </div>
            <Button asChild variant="ghost" size="sm" className="text-xs">
              <Link to="/flows">Gerenciar</Link>
            </Button>
          </div>
        </CardHeader>
        <CardContent className="pt-4">
          {flows.isLoading ? (
            <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
              {[0, 1, 2].map((item) => (
                <Skeleton key={item} className="h-[68px] w-full" />
              ))}
            </div>
          ) : (flows.data?.length ?? 0) === 0 ? (
            <EmptyState
              icon={Workflow}
              title="Nenhum fluxo ainda"
              description="Crie o primeiro fluxo para automatizar a triagem do atendimento."
              action={
                <Button asChild size="sm">
                  <Link to="/flows">Criar fluxo</Link>
                </Button>
              }
              className="border-0 bg-transparent"
            />
          ) : (
            <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
              {flows.data?.map((flow) => (
                <Link
                  key={flow.id}
                  to="/flows/$flowId"
                  params={{ flowId: flow.id }}
                  className="group flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-background/50 px-3 py-3 transition-colors duration-fast hover:border-primary/30 hover:bg-accent/40"
                >
                  <div className="flex min-w-0 items-center gap-2.5">
                    <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-accent text-accent-foreground">
                      <Workflow className="size-4" aria-hidden="true" />
                    </span>
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">{flow.name}</p>
                      <p className="text-xs text-muted-foreground">
                        v{flow.version} · {flow.nodes} {flow.nodes === 1 ? "node" : "nodes"}
                      </p>
                    </div>
                  </div>
                  <ArrowUpRight
                    className="size-4 shrink-0 text-muted-foreground transition-transform duration-fast group-hover:-translate-y-0.5 group-hover:translate-x-0.5"
                    aria-hidden="true"
                  />
                </Link>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function StatCard({
  metaKey,
  value,
  loading,
}: {
  metaKey: StatKey;
  value: number;
  loading: boolean;
}) {
  const meta = statMeta[metaKey];
  const Icon = meta.icon;

  return (
    <Card className="group relative overflow-hidden">
      <span
        aria-hidden="true"
        className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-primary/50 to-transparent opacity-0 transition-opacity duration-normal group-hover:opacity-100"
      />
      <CardContent className="p-5">
        <div className="flex items-center justify-between gap-2">
          <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
            {meta.label}
          </p>
          <span className={cn("flex size-8 items-center justify-center rounded-lg", meta.tone)}>
            <Icon className="size-4" aria-hidden="true" />
          </span>
        </div>
        {loading ? (
          <Skeleton className="mt-4 h-9 w-16" />
        ) : (
          <p className="mt-3 text-3xl font-semibold tracking-tight tabular-nums">{value}</p>
        )}
        <p className="mt-1 text-xs text-muted-foreground">{meta.hint}</p>
      </CardContent>
    </Card>
  );
}

function RecentConversation({
  conversation,
  channelName,
}: {
  conversation: Conversation;
  channelName: string;
}) {
  return (
    <Link
      to="/inbox"
      search={{ conversation: conversation.id }}
      className="group flex items-center gap-3 rounded-lg px-2 py-2.5 transition-colors duration-fast hover:bg-accent/50"
    >
      <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-accent text-[11px] font-semibold text-accent-foreground">
        {initials(visitorLabel(conversation.visitorId))}
      </span>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium">{visitorLabel(conversation.visitorId)}</p>
        <p className="truncate text-xs text-muted-foreground">
          {channelName}
          <span className="mx-1.5 text-muted-foreground/40">·</span>
          {relativeTime(conversation.updatedAt)}
        </p>
      </div>
      <StatusBadge status={conversation.status} />
    </Link>
  );
}

function RecentListSkeleton() {
  return (
    <div className="flex flex-col gap-2">
      {[0, 1, 2, 3, 4].map((item) => (
        <div key={item} className="flex items-center gap-3 px-2 py-2">
          <Skeleton className="size-9 rounded-full" />
          <div className="flex-1 space-y-1.5">
            <Skeleton className="h-3.5 w-32" />
            <Skeleton className="h-3 w-24" />
          </div>
          <Skeleton className="h-5 w-16 rounded-full" />
        </div>
      ))}
    </div>
  );
}
