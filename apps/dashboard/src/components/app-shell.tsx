import { useState, type ReactNode } from "react";
import { Link, useRouterState } from "@tanstack/react-router";
import {
  BarChart3,
  BookOpen,
  ChevronRight,
  Inbox,
  LayoutDashboard,
  LogOut,
  MessagesSquare,
  Palette,
  Users,
  Workflow,
} from "lucide-react";
import type { Availability } from "@chatfree/shared";
import { useAuth } from "@/lib/auth";
import { availabilityDot, availabilityLabel, initials } from "@/lib/labels";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ThemeToggle } from "@/components/theme-toggle";

const nav = [
  { to: "/", label: "Visão geral", icon: LayoutDashboard },
  { to: "/inbox", label: "Atendimento", icon: Inbox },
  { to: "/reports", label: "Relatórios", icon: BarChart3 },
  { to: "/replies", label: "Respostas prontas", icon: BookOpen },
  { to: "/flows", label: "Fluxos", icon: Workflow },
  { to: "/channels", label: "Canal e widget", icon: Palette },
  { to: "/team", label: "Equipe", icon: Users },
] as const;

function isNavActive(pathname: string, to: string) {
  return to === "/" ? pathname === "/" : pathname.startsWith(to);
}

function AvailabilitySelect({
  value,
  onChange,
  disabled,
  className,
  compact,
}: {
  value: Availability;
  onChange: (value: string) => void;
  disabled: boolean;
  className?: string;
  compact?: boolean;
}) {
  return (
    <Select value={value} onValueChange={onChange} disabled={disabled}>
      <SelectTrigger
        className={cn("text-xs", compact ? "h-8 w-32" : "h-8 w-full", className)}
        aria-label="Disponibilidade"
      >
        <SelectValue placeholder="Disponibilidade" />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="online">Disponível</SelectItem>
        <SelectItem value="busy">Ocupado</SelectItem>
        <SelectItem value="offline">Offline</SelectItem>
      </SelectContent>
    </Select>
  );
}

export function AppShell({ children }: { children: ReactNode }) {
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const { session, logout, setAvailability } = useAuth();
  const [updating, setUpdating] = useState(false);

  const availability = session?.membership.availability ?? "offline";
  const userInitials = initials(session?.user.name);
  const currentPage = nav.find((item) => isNavActive(pathname, item.to));

  const changeAvailability = async (value: string) => {
    setUpdating(true);
    try {
      await setAvailability(value as Availability);
    } finally {
      setUpdating(false);
    }
  };

  return (
    <div className="flex min-h-svh bg-background">
      <a
        href="#conteudo"
        className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-50 focus:rounded-lg focus:bg-primary focus:px-4 focus:py-2 focus:text-sm focus:font-medium focus:text-primary-foreground"
      >
        Pular para o conteúdo
      </a>

      <aside className="sticky top-0 hidden h-svh w-64 shrink-0 flex-col border-r border-border/70 bg-sidebar lg:flex">
        <div className="flex min-h-0 flex-1 flex-col px-3 py-4">
          <Link
            to="/"
            className="flex items-center gap-3 rounded-xl px-2 py-1.5 transition-colors duration-fast hover:bg-accent/60"
            aria-label="Ir para a visão geral"
          >
            <span className="flex size-9 items-center justify-center rounded-xl bg-gradient-to-br from-primary to-primary/75 text-primary-foreground shadow-sm">
              <MessagesSquare className="size-4.5" />
            </span>
            <span className="flex min-w-0 flex-col">
              <span className="truncate text-sm font-semibold tracking-tight">ChatFree</span>
              <span className="max-w-36 truncate text-xs text-muted-foreground">
                {session?.workspace.name ?? "Workspace"}
              </span>
            </span>
          </Link>

          <nav className="mt-7 flex flex-col gap-1" aria-label="Navegação principal">
            <p className="px-3 pb-1 text-[10px] font-semibold uppercase tracking-[0.14em] text-muted-foreground/80">
              Operação
            </p>
            {nav.map((item) => {
              const isActive = isNavActive(pathname, item.to);
              return (
                <Link
                  key={item.to}
                  to={item.to}
                  className={cn(
                    "relative flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors duration-fast",
                    isActive
                      ? "bg-accent text-accent-foreground shadow-xs"
                      : "text-muted-foreground hover:bg-accent/50 hover:text-foreground",
                  )}
                  aria-current={isActive ? "page" : undefined}
                >
                  {isActive ? (
                    <span
                      aria-hidden="true"
                      className="absolute left-0 top-1/2 h-4 w-0.5 -translate-y-1/2 rounded-full bg-primary"
                    />
                  ) : null}
                  <item.icon className="size-4 shrink-0" aria-hidden="true" />
                  {item.label}
                </Link>
              );
            })}
          </nav>

          <div className="mt-auto rounded-xl border border-border/70 bg-card p-3 shadow-xs">
            <div className="flex items-center gap-3">
              <span className="relative flex size-9 shrink-0 items-center justify-center rounded-full bg-primary/10 text-xs font-semibold text-primary">
                {userInitials}
                <span
                  className={cn(
                    "absolute -bottom-0.5 -right-0.5 size-2.5 rounded-full border-2 border-card",
                    availabilityDot[availability],
                  )}
                  aria-hidden="true"
                />
              </span>
              <div className="min-w-0 flex-1">
                <p className="truncate text-xs font-medium">{session?.user.name}</p>
                <p className="truncate text-[11px] text-muted-foreground">{session?.user.email}</p>
              </div>
            </div>

            <div className="mt-3 flex items-center gap-2">
              <AvailabilitySelect
                value={availability}
                onChange={changeAvailability}
                disabled={updating}
                className="flex-1"
              />
              <Button
                variant="ghost"
                size="icon"
                className="size-8 shrink-0 rounded-lg transition-colors hover:bg-destructive/10 hover:text-destructive"
                onClick={logout}
                aria-label="Sair"
              >
                <LogOut className="size-4" />
              </Button>
            </div>
          </div>
        </div>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-30 flex h-14 items-center justify-between gap-3 border-b border-border/70 bg-background/85 px-4 backdrop-blur-md lg:px-8">
          <div className="flex min-w-0 items-center gap-2 lg:hidden">
            <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-gradient-to-br from-primary to-primary/75 text-primary-foreground">
              <MessagesSquare className="size-4" />
            </span>
            <span className="truncate text-sm font-semibold">{currentPage?.label ?? "ChatFree"}</span>
          </div>

          <nav
            aria-label="Trilha de navegação"
            className="hidden min-w-0 items-center gap-1.5 text-sm lg:flex"
          >
            <Link
              to="/"
              className="truncate text-muted-foreground transition-colors hover:text-foreground"
            >
              {session?.workspace.name ?? "Workspace"}
            </Link>
            <ChevronRight className="size-3.5 shrink-0 text-muted-foreground/60" aria-hidden="true" />
            <span className="truncate font-medium text-foreground">
              {currentPage?.label ?? "Visão geral"}
            </span>
          </nav>

          <div className="flex shrink-0 items-center gap-1.5 sm:gap-2">
            <ThemeToggle />
            <AvailabilitySelect
              value={availability}
              onChange={changeAvailability}
              disabled={updating}
              compact
              className="lg:hidden"
            />
            <Badge
              variant="outline"
              className="hidden gap-1.5 border-border/80 px-2 text-muted-foreground sm:inline-flex"
            >
              <span className={cn("size-1.5 rounded-full", availabilityDot[availability])} aria-hidden="true" />
              {availabilityLabel[availability]}
            </Badge>
            <Badge variant="secondary" className="hidden gap-1.5 px-2 capitalize md:inline-flex">
              {session?.membership.role}
            </Badge>
            <Button
              variant="ghost"
              size="icon"
              className="size-8 rounded-lg transition-colors hover:bg-destructive/10 hover:text-destructive lg:hidden"
              onClick={logout}
              aria-label="Sair"
            >
              <LogOut className="size-4" />
            </Button>
          </div>
        </header>

        <nav
          className="no-scrollbar flex gap-1 overflow-x-auto border-b border-border/70 bg-background/70 px-3 py-2 lg:hidden"
          aria-label="Navegação móvel"
        >
          {nav.map((item) => {
            const isActive = isNavActive(pathname, item.to);
            return (
              <Link
                key={item.to}
                to={item.to}
                className={cn(
                  "flex shrink-0 items-center gap-2 rounded-lg px-3 py-2 text-xs font-medium transition-colors duration-fast",
                  isActive
                    ? "bg-accent text-accent-foreground"
                    : "text-muted-foreground hover:bg-accent/50 hover:text-foreground",
                )}
                aria-current={isActive ? "page" : undefined}
              >
                <item.icon className="size-3.5 shrink-0" aria-hidden="true" />
                {item.label}
              </Link>
            );
          })}
        </nav>

        <main id="conteudo" className="flex-1 px-4 py-6 lg:px-8">
          {children}
        </main>
      </div>
    </div>
  );
}
