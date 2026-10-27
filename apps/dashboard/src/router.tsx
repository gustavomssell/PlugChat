import {
  createRootRoute,
  createRoute,
  createRouter,
  lazyRouteComponent,
  Navigate,
  Outlet,
} from "@tanstack/react-router";
import { Loader2 } from "lucide-react";
import { AuthProvider, useAuth } from "@/lib/auth";
import { AppShell } from "@/components/app-shell";
import { LoginPage } from "@/pages/login";
import { OverviewPage } from "@/pages/overview";

const rootRoute = createRootRoute({
  component: () => (
    <AuthProvider>
      <Outlet />
    </AuthProvider>
  ),
});

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/login",
  component: LoginPage,
});

function AppLayout() {
  const { loading, session } = useAuth();

  if (loading) {
    return (
      <div className="flex min-h-svh items-center justify-center bg-background">
        <Loader2 className="size-5 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!session) {
    return <Navigate to="/login" replace />;
  }

  return (
    <AppShell>
      <Outlet />
    </AppShell>
  );
}

const appRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: "_app",
  component: AppLayout,
});

const overviewRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/",
  component: OverviewPage,
});

const inboxRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/inbox",
  validateSearch: (search: Record<string, unknown>): { conversation?: string } => {
    const value = search.conversation;
    return typeof value === "string" && value.length > 0 ? { conversation: value } : {};
  },
  component: lazyRouteComponent(() => import("@/pages/inbox"), "InboxPage"),
});

const flowsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/flows",
  component: lazyRouteComponent(() => import("@/pages/flows"), "FlowsPage"),
});

const channelsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/channels",
  component: lazyRouteComponent(() => import("@/pages/channels"), "ChannelsPage"),
});

const reportsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/reports",
  component: lazyRouteComponent(() => import("@/pages/reports"), "ReportsPage"),
});

const repliesRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/replies",
  component: lazyRouteComponent(() => import("@/pages/replies"), "RepliesPage"),
});

const teamRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/team",
  component: lazyRouteComponent(() => import("@/pages/team"), "TeamPage"),
});

const flowEditorRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/flows/$flowId",
  component: lazyRouteComponent(
    () => import("@/pages/flow-editor"),
    "FlowEditorPage",
  ),
});

const routeTree = rootRoute.addChildren([
  loginRoute,
  appRoute.addChildren([
    overviewRoute,
    inboxRoute,
    flowsRoute,
    flowEditorRoute,
    channelsRoute,
    teamRoute,
    reportsRoute,
    repliesRoute,
  ]),
]);

function RoutePending() {
  return (
    <div className="flex min-h-[40svh] items-center justify-center">
      <Loader2 className="size-5 animate-spin text-muted-foreground" aria-label="Carregando" />
    </div>
  );
}

export const router = createRouter({
  routeTree,
  defaultPendingComponent: RoutePending,
  defaultPendingMs: 150,
  defaultPendingMinMs: 200,
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
