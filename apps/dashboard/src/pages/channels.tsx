import { useEffect, useState } from "react";
import { Controller, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Check,
  Copy,
  KeyRound,
  Palette,
  Plus,
  RefreshCw,
  Save,
  Trash2,
} from "lucide-react";
import {
  DEFAULT_APPEARANCE,
  DISPLAY_MODES,
  WIDGET_POSITIONS,
  WIDGET_THEMES,
  channelSchema,
  type ChannelInput,
  type DisplayMode,
  type WidgetAppearance,
  type WidgetPosition,
  type WidgetTheme,
} from "@chatfree/shared";
import { useAuth } from "@/lib/auth";
import { useTheme } from "@/lib/theme";
import { api, apiUrl, type ChannelInputPayload } from "@/lib/api";
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
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { useToast } from "@/components/ui/toast";
import { PageHeader } from "@/components/page-header";
import { EmptyState } from "@/components/empty-state";

const modeLabels: Record<DisplayMode, string> = {
  bubble: "Bolha flutuante",
  inline: "Embutido na página",
  popup: "Popup central",
  fullscreen: "Tela cheia",
};

const positionLabels: Record<WidgetPosition, string> = {
  "bottom-right": "Inferior direita",
  "bottom-left": "Inferior esquerda",
  "top-right": "Superior direita",
  "top-left": "Superior esquerda",
};

const themeLabels: Record<WidgetTheme, string> = {
  light: "Claro",
  dark: "Escuro",
  auto: "Automático",
};

function parseOrigins(text: string) {
  return text
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);
}

export function ChannelsPage() {
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const { session } = useAuth();
  const canManage = session?.membership.role === "owner" || session?.membership.role === "admin";

  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [originsText, setOriginsText] = useState("");
  const [savedAt, setSavedAt] = useState<string | null>(null);
  const [copied, setCopied] = useState<"snippet" | "key" | null>(null);
  const [error, setError] = useState<string | null>(null);

  const channels = useQuery({ queryKey: ["channels"], queryFn: api.listChannels });
  const flows = useQuery({ queryKey: ["flows"], queryFn: api.listFlows });

  const selected = channels.data?.find((channel) => channel.id === selectedId) ?? null;

  const form = useForm<ChannelInput>({
    resolver: zodResolver(channelSchema),
    defaultValues: {
      name: "",
      flowId: "",
      appearance: DEFAULT_APPEARANCE,
    },
  });

  useEffect(() => {
    if (!channels.data) return;
    if (!selectedId || !channels.data.some((channel) => channel.id === selectedId)) {
      setSelectedId(channels.data[0]?.id ?? null);
    }
  }, [channels.data, selectedId]);

  useEffect(() => {
    if (!selected) return;
    form.reset({
      name: selected.name,
      flowId: selected.flowId,
      appearance: selected.appearance,
    });
    setOriginsText(selected.allowedOrigins.join("\n"));
    setSavedAt(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedId, form]);

  const create = useMutation({
    mutationFn: async () => {
      const flowId = flows.data?.[0]?.id;
      if (!flowId) {
        throw new Error("Crie um fluxo antes de adicionar um canal.");
      }
      return api.createChannel({
        name: "Novo canal",
        flowId,
        appearance: DEFAULT_APPEARANCE,
        allowedOrigins: [],
      });
    },
    onSuccess: (channel) => {
      setError(null);
      setSelectedId(channel.id);
      toast({ tone: "success", title: "Canal criado", description: channel.name });
      void queryClient.invalidateQueries({ queryKey: ["channels"] });
    },
    onError: (value) => {
      const message = value instanceof Error ? value.message : "Não foi possível criar o canal.";
      setError(message);
      toast({ tone: "error", title: "Falha ao criar canal", description: message });
    },
  });

  const save = useMutation({
    mutationFn: (values: ChannelInput) => {
      if (!selected) {
        throw new Error("Selecione um canal.");
      }
      const payload: ChannelInputPayload = {
        ...values,
        allowedOrigins: parseOrigins(originsText),
      };
      return api.updateChannel(selected.id, payload);
    },
    onSuccess: (channel) => {
      queryClient.setQueryData(["channels"], (current: typeof channels.data) =>
        current?.map((item) => (item.id === channel.id ? channel : item)),
      );
      setError(null);
      setSavedAt(new Date().toLocaleTimeString("pt-BR"));
      toast({ tone: "success", title: "Configuração salva" });
      void queryClient.invalidateQueries({ queryKey: ["channels"] });
    },
    onError: (value) => {
      const message = value instanceof Error ? value.message : "Não foi possível salvar o canal.";
      setError(message);
      toast({ tone: "error", title: "Falha ao salvar", description: message });
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => api.deleteChannel(id),
    onSuccess: () => {
      setSelectedId(null);
      setError(null);
      toast({ tone: "success", title: "Canal removido" });
      void queryClient.invalidateQueries({ queryKey: ["channels"] });
    },
    onError: (value) => {
      const message = value instanceof Error ? value.message : "Não foi possível remover o canal.";
      setError(message);
      toast({ tone: "error", title: "Falha ao remover canal", description: message });
    },
  });

  const appearance = form.watch("appearance");
  const name = form.watch("name");

  const apiBase =
    apiUrl("") || (import.meta.env.DEV ? "http://localhost:8090" : window.location.origin);
  const snippet = selected
    ? [
        `<script async src="https://cdn.chatfree.app/w.js"`,
        `  data-key="${selected.key}"`,
        `  data-api="${apiBase}"`,
        `  data-mode="${appearance.mode}"`,
        `  data-position="${appearance.position}"></script>`,
      ].join("\n")
    : "";

  const copy = async (value: string, kind: "snippet" | "key") => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(kind);
      window.setTimeout(() => setCopied(null), 2000);
      toast({
        tone: "success",
        title: kind === "key" ? "Chave copiada" : "Código copiado",
        description: kind === "key" ? selected?.key : "Cole o snippet antes de fechar o body.",
      });
    } catch {
      toast({
        tone: "error",
        title: "Não foi possível copiar",
        description: "Seu navegador bloqueou o acesso à área de transferência.",
      });
    }
  };

  return (
    <div className="mx-auto flex w-full max-w-6xl flex-col gap-6">
      <PageHeader
        eyebrow="Canais"
        title="Canais e widget"
        description="Crie um widget por site, escolha o fluxo e controle as origens permitidas."
        actions={
          canManage ? (
            <Button onClick={() => create.mutate()} disabled={create.isPending}>
              <Plus data-icon="inline-start" />
              {create.isPending ? "Criando..." : "Novo canal"}
            </Button>
          ) : undefined
        }
      />

      {error ? (
        <p role="alert" className="rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
          {error}
        </p>
      ) : null}

      {channels.isLoading ? (
        <ChannelsSkeleton />
      ) : channels.isError ? (
        <div className="flex flex-col items-center gap-3 rounded-xl border border-destructive/30 bg-destructive/5 px-6 py-10 text-center">
          <p className="text-sm font-medium text-destructive">Não foi possível carregar os canais.</p>
          <Button variant="outline" size="sm" onClick={() => void channels.refetch()}>
            <RefreshCw data-icon="inline-start" />
            Tentar novamente
          </Button>
        </div>
      ) : (channels.data?.length ?? 0) === 0 ? (
        <EmptyState
          icon={Palette}
          title="Nenhum canal ainda"
          description="Crie seu primeiro canal para gerar uma chave pública e incorporar o widget no site."
          action={
            canManage ? (
              <Button onClick={() => create.mutate()} disabled={create.isPending}>
                <Plus data-icon="inline-start" />
                Criar canal
              </Button>
            ) : undefined
          }
        />
      ) : (
        <>
          <div
            className="flex gap-2 overflow-x-auto pb-1"
            role="tablist"
            aria-label="Canais do workspace"
          >
            {channels.data?.map((channel) => (
              <button
                key={channel.id}
                type="button"
                role="tab"
                aria-selected={channel.id === selectedId}
                onClick={() => setSelectedId(channel.id)}
                className={cn(
                  "flex min-w-48 flex-col gap-1 rounded-xl border px-3 py-2.5 text-left transition-colors duration-fast",
                  channel.id === selectedId
                    ? "border-primary/50 bg-accent shadow-xs"
                    : "border-border/70 bg-card hover:bg-accent/40",
                )}
              >
                <span className="flex items-center justify-between gap-2">
                  <span className="truncate text-sm font-medium">{channel.name}</span>
                  <Badge variant="outline" className="shrink-0 text-[10px]">
                    {modeLabels[channel.appearance.mode]}
                  </Badge>
                </span>
                <span className="truncate font-mono text-[11px] text-muted-foreground">
                  {channel.key}
                </span>
              </button>
            ))}
          </div>

          {selected ? (
            <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_370px]">
              <Card>
                <CardHeader className="border-b border-border/60 pb-4">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <CardTitle className="text-base">Configuração do canal</CardTitle>
                      <CardDescription>
                        Essas opções podem ser sobrescritas por atributos no script do widget.
                      </CardDescription>
                    </div>
                    {canManage ? (
                      <ConfirmDialog
                        title={`Remover o canal "${selected.name}"?`}
                        description="O widget deixa de funcionar imediatamente nos sites que usam esta chave."
                        confirmLabel="Remover canal"
                        onConfirm={() => remove.mutate(selected.id)}
                        trigger={
                          <Button
                            variant="ghost"
                            size="icon"
                            className="text-muted-foreground hover:text-destructive"
                            aria-label="Remover canal"
                          >
                            <Trash2 />
                          </Button>
                        }
                      />
                    ) : null}
                  </div>
                </CardHeader>
                <CardContent className="pt-5">
                  <form
                    onSubmit={form.handleSubmit((values) => save.mutate(values))}
                    className="flex flex-col gap-6"
                  >
                    <section className="flex flex-col gap-4">
                      <SectionTitle
                        title="Identidade"
                        description="Como o canal aparece e qual automação ele executa."
                      />
                      <FieldGroup>
                        <Field data-invalid={Boolean(form.formState.errors.name)}>
                          <FieldLabel htmlFor="channel-name">Nome do canal</FieldLabel>
                          <Input
                            id="channel-name"
                            disabled={!canManage}
                            aria-invalid={Boolean(form.formState.errors.name)}
                            {...form.register("name")}
                          />
                          <FieldDescription>Aparece no topo da janela do chat.</FieldDescription>
                          <FieldError errors={[form.formState.errors.name]} />
                        </Field>

                        <Field data-invalid={Boolean(form.formState.errors.flowId)}>
                          <FieldLabel htmlFor="channel-flow">Fluxo de automação</FieldLabel>
                          <Controller
                            control={form.control}
                            name="flowId"
                            render={({ field }) => (
                              <Select value={field.value} onValueChange={field.onChange} disabled={!canManage}>
                                <SelectTrigger id="channel-flow" className="w-full">
                                  <SelectValue placeholder="Selecione um fluxo" />
                                </SelectTrigger>
                                <SelectContent>
                                  {flows.data?.map((flow) => (
                                    <SelectItem key={flow.id} value={flow.id}>
                                      {flow.name} · v{flow.version}
                                    </SelectItem>
                                  ))}
                                </SelectContent>
                              </Select>
                            )}
                          />
                          <FieldError errors={[form.formState.errors.flowId]} />
                        </Field>
                      </FieldGroup>
                    </section>

                    <SeparatorLine />

                    <section className="flex flex-col gap-4">
                      <SectionTitle
                        title="Exibição"
                        description="Onde e como a janela do widget aparece no site."
                      />
                      <div className="grid gap-4 sm:grid-cols-2">
                        <Field>
                          <FieldLabel htmlFor="channel-mode">Modo</FieldLabel>
                          <Controller
                            control={form.control}
                            name="appearance.mode"
                            render={({ field }) => (
                              <Select value={field.value} onValueChange={field.onChange} disabled={!canManage}>
                                <SelectTrigger id="channel-mode" className="w-full">
                                  <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                  <SelectGroup>
                                    {DISPLAY_MODES.map((mode) => (
                                      <SelectItem key={mode} value={mode}>
                                        {modeLabels[mode]}
                                      </SelectItem>
                                    ))}
                                  </SelectGroup>
                                </SelectContent>
                              </Select>
                            )}
                          />
                        </Field>

                        <Field>
                          <FieldLabel htmlFor="channel-position">Posição</FieldLabel>
                          <Controller
                            control={form.control}
                            name="appearance.position"
                            render={({ field }) => (
                              <Select value={field.value} onValueChange={field.onChange} disabled={!canManage}>
                                <SelectTrigger id="channel-position" className="w-full">
                                  <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                  <SelectGroup>
                                    {WIDGET_POSITIONS.map((position) => (
                                      <SelectItem key={position} value={position}>
                                        {positionLabels[position]}
                                      </SelectItem>
                                    ))}
                                  </SelectGroup>
                                </SelectContent>
                              </Select>
                            )}
                          />
                        </Field>

                        <Field>
                          <FieldLabel htmlFor="channel-theme">Tema do widget</FieldLabel>
                          <Controller
                            control={form.control}
                            name="appearance.theme"
                            render={({ field }) => (
                              <Select value={field.value} onValueChange={field.onChange} disabled={!canManage}>
                                <SelectTrigger id="channel-theme" className="w-full">
                                  <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                  <SelectGroup>
                                    {WIDGET_THEMES.map((theme) => (
                                      <SelectItem key={theme} value={theme}>
                                        {themeLabels[theme]}
                                      </SelectItem>
                                    ))}
                                  </SelectGroup>
                                </SelectContent>
                              </Select>
                            )}
                          />
                          <FieldDescription>
                            Automático acompanha a preferência do visitante.
                          </FieldDescription>
                        </Field>

                        <Field data-invalid={Boolean(form.formState.errors.appearance?.accent)}>
                          <FieldLabel htmlFor="channel-accent">Cor de destaque</FieldLabel>
                          <Controller
                            control={form.control}
                            name="appearance.accent"
                            render={({ field }) => (
                              <div className="flex items-center gap-2">
                                <Input
                                  type="color"
                                  disabled={!canManage}
                                  className="h-9 w-14 cursor-pointer p-1"
                                  aria-label="Selecionar cor de destaque"
                                  value={field.value}
                                  onChange={(event) => field.onChange(event.target.value)}
                                />
                                <Input
                                  id="channel-accent"
                                  disabled={!canManage}
                                  className="w-32 font-mono text-xs uppercase"
                                  aria-label="Valor hexadecimal da cor"
                                  aria-invalid={Boolean(form.formState.errors.appearance?.accent)}
                                  value={field.value}
                                  onChange={(event) => field.onChange(event.target.value)}
                                  onBlur={field.onBlur}
                                />
                              </div>
                            )}
                          />
                          <FieldError errors={[form.formState.errors.appearance?.accent]} />
                        </Field>
                      </div>
                    </section>

                    <SeparatorLine />

                    <section className="flex flex-col gap-4">
                      <SectionTitle
                        title="Conteúdo"
                        description="Textos exibidos na abertura e no rodapé do widget."
                      />
                      <FieldGroup>
                        <Field data-invalid={Boolean(form.formState.errors.appearance?.greeting)}>
                          <FieldLabel htmlFor="channel-greeting">Mensagem de saudação</FieldLabel>
                          <Textarea
                            id="channel-greeting"
                            rows={3}
                            disabled={!canManage}
                            aria-invalid={Boolean(form.formState.errors.appearance?.greeting)}
                            {...form.register("appearance.greeting")}
                          />
                          <FieldError errors={[form.formState.errors.appearance?.greeting]} />
                        </Field>

                        <Field data-invalid={Boolean(form.formState.errors.appearance?.launcherLabel)}>
                          <FieldLabel htmlFor="channel-launcher">Texto do botão</FieldLabel>
                          <Input
                            id="channel-launcher"
                            disabled={!canManage}
                            aria-invalid={Boolean(form.formState.errors.appearance?.launcherLabel)}
                            {...form.register("appearance.launcherLabel")}
                          />
                          <FieldDescription>Usado como rótulo acessível do botão flutuante.</FieldDescription>
                          <FieldError errors={[form.formState.errors.appearance?.launcherLabel]} />
                        </Field>

                        <Field orientation="horizontal">
                          <Controller
                            control={form.control}
                            name="appearance.showBranding"
                            render={({ field }) => (
                              <Switch
                                checked={field.value}
                                onCheckedChange={field.onChange}
                                disabled={!canManage}
                                aria-label="Mostrar assinatura ChatFree"
                              />
                            )}
                          />
                          <div className="flex flex-col gap-1">
                            <FieldLabel>Assinatura ChatFree</FieldLabel>
                            <FieldDescription>Exibe "Atendimento por ChatFree" no rodapé.</FieldDescription>
                          </div>
                        </Field>
                      </FieldGroup>
                    </section>

                    <SeparatorLine />

                    <section className="flex flex-col gap-4">
                      <SectionTitle
                        title="Segurança"
                        description="Restrinja em quais domínios o widget pode ser carregado."
                      />
                      <Field>
                        <FieldLabel htmlFor="channel-origins">Origens permitidas</FieldLabel>
                        <Textarea
                          id="channel-origins"
                          rows={3}
                          disabled={!canManage}
                          value={originsText}
                          onChange={(event) => setOriginsText(event.target.value)}
                          placeholder={"https://meusite.com\nhttps://www.meusite.com"}
                          className="font-mono text-xs"
                        />
                        <FieldDescription>
                          Uma origem por linha. Vazio libera qualquer site — use em produção com cuidado.
                        </FieldDescription>
                      </Field>
                    </section>

                    <div className="flex flex-wrap items-center gap-3 border-t border-border/60 pt-5">
                      <Button type="submit" disabled={!canManage || save.isPending}>
                        <Save data-icon="inline-start" />
                        {save.isPending ? "Salvando..." : "Salvar configuração"}
                      </Button>
                      {savedAt ? (
                        <span className="flex items-center gap-1 text-xs text-success">
                          <Check className="size-3.5" aria-hidden="true" />
                          salvo às {savedAt}
                        </span>
                      ) : null}
                    </div>
                  </form>
                </CardContent>
              </Card>

              <div className="flex flex-col gap-6 xl:sticky xl:top-20">
                <Card>
                  <CardHeader className="border-b border-border/60 pb-4">
                    <div className="flex items-center justify-between gap-2">
                      <div>
                        <CardTitle className="text-base">Pré-visualização</CardTitle>
                        <CardDescription>{modeLabels[appearance.mode]}</CardDescription>
                      </div>
                      <Badge variant="outline" className="gap-1.5 text-[10px]">
                        <span className="size-1.5 animate-pulse rounded-full bg-success" aria-hidden="true" />
                        ao vivo
                      </Badge>
                    </div>
                  </CardHeader>
                  <CardContent className="pt-4">
                    <WidgetPreview name={name} appearance={appearance} />
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader className="border-b border-border/60 pb-4">
                    <CardTitle className="text-base">Chave pública</CardTitle>
                    <CardDescription>Identifica o canal no snippet.</CardDescription>
                  </CardHeader>
                  <CardContent className="pt-4">
                    <div className="flex items-center gap-2 rounded-lg border border-border/70 bg-muted/30 px-3 py-2">
                      <KeyRound className="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
                      <code className="min-w-0 flex-1 truncate font-mono text-xs">{selected.key}</code>
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon-sm"
                        onClick={() => void copy(selected.key, "key")}
                        aria-label="Copiar chave pública"
                      >
                        {copied === "key" ? <Check className="text-success" /> : <Copy />}
                      </Button>
                    </div>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader className="border-b border-border/60 pb-4">
                    <CardTitle className="text-base">Código de incorporação</CardTitle>
                    <CardDescription>Cole antes de fechar o body no site do cliente.</CardDescription>
                  </CardHeader>
                  <CardContent className="flex flex-col gap-3 pt-4">
                    <pre className="scrollbar-thin overflow-x-auto rounded-lg border border-border/60 bg-muted/40 p-3 text-[11px] leading-relaxed">
                      <code>{snippet}</code>
                    </pre>
                    <Button type="button" variant="outline" onClick={() => void copy(snippet, "snippet")}>
                      {copied === "snippet" ? (
                        <Check data-icon="inline-start" />
                      ) : (
                        <Copy data-icon="inline-start" />
                      )}
                      {copied === "snippet" ? "Copiado" : "Copiar código"}
                    </Button>
                  </CardContent>
                </Card>
              </div>
            </div>
          ) : null}
        </>
      )}
    </div>
  );
}

function SectionTitle({ title, description }: { title: string; description: string }) {
  return (
    <div>
      <h3 className="text-sm font-semibold tracking-tight">{title}</h3>
      <p className="mt-0.5 text-xs text-muted-foreground">{description}</p>
    </div>
  );
}

function SeparatorLine() {
  return <div className="h-px w-full bg-border/60" aria-hidden="true" />;
}

function WidgetPreview({ name, appearance }: { name: string; appearance: WidgetAppearance }) {
  const { resolvedTheme } = useTheme();
  const isDark =
    appearance.theme === "dark" || (appearance.theme === "auto" && resolvedTheme === "dark");
  const accent = appearance.accent || "#4f46e5";

  const isBubble = appearance.mode === "bubble";
  const isFullscreen = appearance.mode === "fullscreen";
  const panelPosition = isBubble
    ? {
        "bottom-right": "bottom-12 right-3",
        "bottom-left": "bottom-12 left-3",
        "top-right": "top-12 right-3",
        "top-left": "top-12 left-3",
      }[appearance.position]
    : isFullscreen
      ? "inset-2"
      : "left-1/2 top-1/2 h-52 w-44 -translate-x-1/2 -translate-y-1/2";

  const launcherPosition = {
    "bottom-right": "bottom-3 right-3",
    "bottom-left": "bottom-3 left-3",
    "top-right": "top-3 right-3",
    "top-left": "top-3 left-3",
  }[appearance.position];

  const palette = isDark
    ? {
        panel: "border-white/10 bg-[#161923] text-slate-100",
        muted: "text-slate-400",
        bubble: "bg-white/8 text-slate-100",
        input: "border-white/10 bg-white/5",
      }
    : {
        panel: "border-slate-200 bg-white text-slate-900",
        muted: "text-slate-500",
        bubble: "bg-slate-100 text-slate-800",
        input: "border-slate-200 bg-slate-50",
      };

  return (
    <div className="relative h-[300px] overflow-hidden rounded-xl border border-border/70 bg-muted/30">
      <div className="absolute inset-0 bg-dots opacity-40" aria-hidden="true" />
      <div className="absolute inset-x-4 top-4 flex items-center gap-1.5 opacity-40" aria-hidden="true">
        <span className="size-1.5 rounded-full bg-muted-foreground/50" />
        <span className="size-1.5 rounded-full bg-muted-foreground/30" />
        <span className="size-1.5 rounded-full bg-muted-foreground/20" />
        <span className="ml-2 h-2 w-24 rounded-full bg-muted-foreground/15" />
      </div>
      <div className="absolute inset-x-4 top-10 space-y-2 opacity-30" aria-hidden="true">
        <div className="h-2 w-2/3 rounded-full bg-muted-foreground/20" />
        <div className="h-2 w-1/2 rounded-full bg-muted-foreground/15" />
        <div className="h-2 w-3/5 rounded-full bg-muted-foreground/10" />
      </div>

      <div
        className={cn(
          "absolute flex flex-col overflow-hidden rounded-xl border shadow-lg",
          panelPosition,
          isBubble ? "h-52 w-44" : !isFullscreen ? "h-52 w-44" : "",
          palette.panel,
        )}
      >
        <div
          className="flex items-center gap-2 px-2.5 py-2 text-white"
          style={{ backgroundColor: accent }}
        >
          <span className="flex size-6 items-center justify-center rounded-full bg-white/20 text-[10px] font-semibold">
            {(name || "A").slice(0, 1).toUpperCase()}
          </span>
          <div className="flex min-w-0 flex-col">
            <span className="truncate text-[11px] font-semibold">{name || "Atendimento"}</span>
            <span className="text-[9px] text-white/75">Online agora</span>
          </div>
        </div>
        <div className="flex flex-1 flex-col gap-1.5 p-2.5">
          <span className={cn("max-w-[88%] rounded-lg rounded-bl-sm px-2 py-1.5 text-[10px] leading-snug", palette.bubble)}>
            {appearance.greeting || "Olá! Como podemos ajudar?"}
          </span>
          <span className={cn("max-w-[70%] self-end rounded-lg rounded-br-sm px-2 py-1.5 text-[10px]", palette.bubble)}>
            Quero saber mais
          </span>
        </div>
        <div className={cn("flex items-center gap-1.5 border-t p-2", palette.input)}>
          <span className={cn("h-6 flex-1 rounded-full border px-2 text-[9px] leading-6", palette.input, palette.muted)}>
            Escreva...
          </span>
          <span
            className="flex size-6 items-center justify-center rounded-full text-white"
            style={{ backgroundColor: accent }}
          >
            <span className="size-1.5 rotate-45 rounded-sm bg-current" aria-hidden="true" />
          </span>
        </div>
        {appearance.showBranding ? (
          <p className={cn("pb-1 text-center text-[8px]", palette.muted)}>Atendimento por ChatFree</p>
        ) : null}
      </div>

      {isBubble ? (
        <button
          type="button"
          tabIndex={-1}
          aria-hidden="true"
          className={cn(
            "absolute flex size-9 items-center justify-center rounded-full text-white shadow-md",
            launcherPosition,
          )}
          style={{ backgroundColor: accent }}
        >
          <span className="size-3.5 rounded-md bg-current" />
        </button>
      ) : null}
    </div>
  );
}

function ChannelsSkeleton() {
  return (
    <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_370px]">
      <div className="space-y-4 rounded-xl border bg-card p-6">
        <Skeleton className="h-5 w-40" />
        <Skeleton className="h-9 w-full" />
        <Skeleton className="h-9 w-full" />
        <Skeleton className="h-9 w-full" />
        <Skeleton className="h-24 w-full" />
      </div>
      <div className="space-y-4">
        <Skeleton className="h-64 w-full rounded-xl" />
        <Skeleton className="h-24 w-full rounded-xl" />
      </div>
    </div>
  );
}
