import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { BarChart3, CheckCircle2, Inbox, Timer, Users } from "lucide-react";
import { api } from "@/lib/api";
import { formatDuration } from "@/lib/labels";
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
import { ConversationsChart } from "@/components/conversations-chart";

const periods = [
  { days: 7, label: "7 dias" },
  { days: 30, label: "30 dias" },
  { days: 90, label: "90 dias" },
] as const;

export function ReportsPage() {
  const [days, setDays] = useState<number>(7);

  const metrics = useQuery({
    queryKey: ["metrics", days],
    queryFn: () => api.metrics(days),
  });

  const data = metrics.data;

  const kpis = [
    {
      label: "Conversas no período",
      value: data?.conversations ?? 0,
      hint: "Iniciadas pelo widget",
      icon: Inbox,
      tone: "bg-primary/10 text-primary",
    },
    {
      label: "Encerradas",
      value: data?.closed ?? 0,
      hint: "Atendimentos concluídos",
      icon: CheckCircle2,
      tone: "bg-success/10 text-success",
    },
    {
      label: "1ª resposta (média)",
      value: formatDuration(data?.firstResponseMs ?? 0),
      hint: "Bot ou atendente",
      icon: Timer,
      tone: "bg-info/10 text-info",
    },
    {
      label: "Mensagens",
      value: data?.messages ?? 0,
      hint: "Todos os lados",
      icon: BarChart3,
      tone: "bg-warning/10 text-warning",
    },
  ];

  return (
    <div className="mx-auto flex w-full max-w-6xl flex-col gap-6">
      <PageHeader
        title="Relatórios"
        description="Volume, tempo de primeira resposta e carga de trabalho por canal e por atendente."
        actions={
          <div className="flex items-center gap-1 rounded-lg border border-border/70 bg-muted/40 p-0.5">
            {periods.map((period) => (
              <button
                key={period.days}
                type="button"
                onClick={() => setDays(period.days)}
                aria-pressed={days === period.days}
                className={cn(
                  "rounded-md px-2.5 py-1 text-xs font-medium transition-colors duration-fast",
                  days === period.days
                    ? "bg-background text-foreground shadow-xs"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                {period.label}
              </button>
            ))}
          </div>
        }
      />

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {kpis.map((kpi) => (
          <Card key={kpi.label}>
            <CardContent className="p-5">
              <div className="flex items-center justify-between gap-2">
                <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                  {kpi.label}
                </p>
                <span className={cn("flex size-8 items-center justify-center rounded-lg", kpi.tone)}>
                  <kpi.icon className="size-4" aria-hidden="true" />
                </span>
              </div>
              {metrics.isLoading ? (
                <Skeleton className="mt-4 h-9 w-20" />
              ) : (
                <p className="mt-3 text-3xl font-semibold tracking-tight tabular-nums">{kpi.value}</p>
              )}
              <p className="mt-1 text-xs text-muted-foreground">{kpi.hint}</p>
            </CardContent>
          </Card>
        ))}
      </div>

      <Card>
        <CardHeader className="border-b border-border/60 pb-4">
          <div className="flex items-center justify-between gap-3">
            <div>
              <CardTitle className="text-base">Conversas por dia</CardTitle>
              <CardDescription>Novas sessões nos últimos {days} dias.</CardDescription>
            </div>
            <Badge variant="outline" className="shrink-0 text-[10px] font-medium text-muted-foreground">
              {days} dias
            </Badge>
          </div>
        </CardHeader>
        <CardContent className="pt-4">
          {metrics.isLoading ? (
            <Skeleton className="h-56 w-full" />
          ) : (
            <ConversationsChart data={data?.conversationsByDay ?? []} />
          )}
        </CardContent>
      </Card>

      <div className="grid items-start gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader className="border-b border-border/60 pb-4">
            <CardTitle className="text-base">Por canal</CardTitle>
            <CardDescription>Conversas e tempo médio de primeira resposta.</CardDescription>
          </CardHeader>
          <CardContent className="pt-2">
            {metrics.isLoading ? (
              <TableSkeleton rows={2} />
            ) : (data?.byChannel.length ?? 0) === 0 ? (
              <EmptyState
                icon={BarChart3}
                title="Nenhum canal com movimento"
                description="Assim que o widget iniciar conversas, o detalhe por canal aparece aqui."
                className="border-0 bg-transparent py-8"
              />
            ) : (
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-left text-[11px] uppercase tracking-wider text-muted-foreground">
                    <th scope="col" className="py-2 font-medium">
                      Canal
                    </th>
                    <th scope="col" className="py-2 text-right font-medium">
                      Conversas
                    </th>
                    <th scope="col" className="py-2 text-right font-medium">
                      Encerradas
                    </th>
                    <th scope="col" className="py-2 text-right font-medium">
                      1ª resposta
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {data?.byChannel.map((channel) => (
                    <tr key={channel.channelId} className="border-t border-border/60">
                      <td className="max-w-40 truncate py-2.5 font-medium">{channel.name}</td>
                      <td className="py-2.5 text-right tabular-nums">{channel.conversations}</td>
                      <td className="py-2.5 text-right tabular-nums text-muted-foreground">
                        {channel.closed}
                      </td>
                      <td className="py-2.5 text-right tabular-nums text-muted-foreground">
                        {formatDuration(channel.firstResponseMs)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="border-b border-border/60 pb-4">
            <CardTitle className="text-base">Por atendente</CardTitle>
            <CardDescription>Conversas atribuídas no período.</CardDescription>
          </CardHeader>
          <CardContent className="pt-2">
            {metrics.isLoading ? (
              <TableSkeleton rows={3} />
            ) : (data?.byAgent.length ?? 0) === 0 ? (
              <EmptyState
                icon={Users}
                title="Nenhuma conversa atribuída"
                description="Atribua conversas na inbox para acompanhar a carga da equipe."
                className="border-0 bg-transparent py-8"
              />
            ) : (
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-left text-[11px] uppercase tracking-wider text-muted-foreground">
                    <th scope="col" className="py-2 font-medium">
                      Atendente
                    </th>
                    <th scope="col" className="py-2 text-right font-medium">
                      Conversas
                    </th>
                    <th scope="col" className="py-2 text-right font-medium">
                      Encerradas
                    </th>
                    <th scope="col" className="py-2 text-right font-medium">
                      1ª resposta
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {data?.byAgent.map((agent) => (
                    <tr key={agent.userId} className="border-t border-border/60">
                      <td className="max-w-40 truncate py-2.5 font-medium">{agent.name}</td>
                      <td className="py-2.5 text-right tabular-nums">{agent.conversations}</td>
                      <td className="py-2.5 text-right tabular-nums text-muted-foreground">
                        {agent.closed}
                      </td>
                      <td className="py-2.5 text-right tabular-nums text-muted-foreground">
                        {formatDuration(agent.firstResponseMs)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </CardContent>
        </Card>
      </div>

      {metrics.isError ? (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          <span>Não foi possível carregar os relatórios. A API está rodando?</span>
          <Button variant="outline" size="sm" onClick={() => void metrics.refetch()}>
            Tentar novamente
          </Button>
        </div>
      ) : null}
    </div>
  );
}

function TableSkeleton({ rows }: { rows: number }) {
  return (
    <div className="flex flex-col gap-2 py-2">
      {Array.from({ length: rows }, (_, index) => (
        <div key={index} className="flex items-center gap-3">
          <Skeleton className="h-4 flex-1" />
          <Skeleton className="h-4 w-12" />
          <Skeleton className="h-4 w-12" />
          <Skeleton className="h-4 w-14" />
        </div>
      ))}
    </div>
  );
}
