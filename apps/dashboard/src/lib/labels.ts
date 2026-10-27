import type { Availability, ConversationStatus, MemberRole, MessageRole } from "@chatfree/shared";

export const conversationStatusLabel: Record<ConversationStatus, string> = {
  bot: "Bot",
  waiting: "Aguardando",
  queued: "Na fila",
  human: "Humano",
  closed: "Encerrada",
};

export const conversationStatusTone: Record<ConversationStatus, string> = {
  bot: "border-info/25 bg-info/10 text-info",
  waiting: "border-warning/25 bg-warning/10 text-warning",
  queued: "border-warning/25 bg-warning/10 text-warning",
  human: "border-success/25 bg-success/10 text-success",
  closed: "border-border bg-muted text-muted-foreground",
};

export const conversationStatusDot: Record<ConversationStatus, string> = {
  bot: "bg-info",
  waiting: "bg-warning",
  queued: "bg-warning",
  human: "bg-success",
  closed: "bg-muted-foreground/60",
};

export const messageRoleLabel: Record<MessageRole, string> = {
  visitor: "Visitante",
  bot: "Bot",
  agent: "Agente",
  system: "Sistema",
  note: "Nota interna",
};

export const availabilityLabel: Record<Availability, string> = {
  online: "Disponível",
  busy: "Ocupado",
  offline: "Offline",
};

export const availabilityDot: Record<Availability, string> = {
  online: "bg-success",
  busy: "bg-warning",
  offline: "bg-muted-foreground/50",
};

export const memberRoleLabel: Record<MemberRole, string> = {
  owner: "Proprietário",
  admin: "Administrador",
  agent: "Agente",
};

export function initials(name?: string | null) {
  if (!name) return "CF";
  return name
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? "")
    .join("");
}

export function visitorLabel(visitorId: string) {
  if (!visitorId) return "Visitante";
  const normalized = visitorId.trim();
  if (normalized.includes("-") || normalized.length > 16) {
    return `Visitante ${normalized.slice(0, 8)}`;
  }
  return normalized;
}

export function shortId(id: string, length = 8) {
  return id.slice(0, length);
}

export function formatTime(iso: string) {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  return date.toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" });
}

export function formatDate(iso: string) {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  return date.toLocaleDateString("pt-BR", { day: "2-digit", month: "short", year: "numeric" });
}

export function formatDateTime(iso: string) {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  return `${formatDate(iso)} · ${formatTime(iso)}`;
}

export function relativeTime(iso: string) {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";

  const diff = date.getTime() - Date.now();
  const absolute = Math.abs(diff);

  if (absolute < 45_000) return "agora";

  const formatter = new Intl.RelativeTimeFormat("pt-BR", { numeric: "auto" });
  const units: { limit: number; amount: number; unit: Intl.RelativeTimeFormatUnit }[] = [
    { limit: 60 * 60 * 1000, amount: 60_000, unit: "minute" },
    { limit: 24 * 60 * 60 * 1000, amount: 60 * 60 * 1000, unit: "hour" },
    { limit: 7 * 24 * 60 * 60 * 1000, amount: 24 * 60 * 60 * 1000, unit: "day" },
    { limit: 30 * 24 * 60 * 60 * 1000, amount: 7 * 24 * 60 * 60 * 1000, unit: "week" },
  ];

  for (const band of units) {
    if (absolute < band.limit) {
      return formatter.format(Math.round(diff / band.amount), band.unit);
    }
  }

  return formatDate(iso);
}

export function dayLabel(isoDate: string) {
  const date = new Date(`${isoDate}T12:00:00`);
  if (Number.isNaN(date.getTime())) return isoDate;
  return date.toLocaleDateString("pt-BR", { day: "2-digit", month: "2-digit" });
}

export function weekdayLabel(isoDate: string) {
  const date = new Date(`${isoDate}T12:00:00`);
  if (Number.isNaN(date.getTime())) return isoDate;
  return date.toLocaleDateString("pt-BR", { weekday: "short" }).replace(".", "");
}

export function formatDuration(ms: number) {
  if (ms <= 0) return "—";
  const totalSeconds = Math.round(ms / 1000);
  if (totalSeconds < 60) return `${totalSeconds}s`;
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (minutes < 60) return `${minutes}min ${seconds}s`;
  const hours = Math.floor(minutes / 60);
  return `${hours}h ${minutes % 60}min`;
}

export function formatBytes(bytes: number) {
  if (bytes <= 0) return "0 KB";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
