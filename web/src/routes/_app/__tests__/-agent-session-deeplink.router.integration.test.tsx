// Suite: cross-workspace session deep-link router integration
// Invariant: a session navigation always selects the session's owning workspace (Routing rule 8,
// US-016.EC-2 — nothing renders cross-workspace in place); preloads never change the selection.
// Boundary IN: TanStack Router, Link, route beforeLoad/loader, Query cache, and active-workspace store.
// Boundary OUT: HTTP adapters and rendered session transcript, owned by their system suites.

import { QueryClient } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import {
  Link,
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRouteWithContext,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { beforeEach, describe, expect, it } from "vitest";

import { sessionKeys, type SessionPayload } from "@/systems/session";
import { useActiveWorkspaceStore, workspaceKeys, type WorkspacePayload } from "@/systems/workspace";
import { routeBeforeLoad, routeLoader } from "@/test/route-options";
import type { AgentSessionRouteLoaderData } from "../-agent-session-route-loader";
import { prefetchAgentSessionRoute } from "../-agent-session-route-loader";
import { Route as ProductionSessionRoute } from "../agents.$name.sessions.$id";

const BENCH_SESSION_ID = "sess-40e90687024bfb24";
const BENCH_WORKSPACE_ID = "ws_74a58ac2bf973937";
const PRIMARY_SESSION_ID = "sess-5ec18f5f2a13fe16";
const PRIMARY_WORKSPACE_ID = "ws_06366aad69887872";

interface TestRouterContext {
  queryClient: QueryClient;
}

function makeWorkspace(id: string, name: string): WorkspacePayload {
  return {
    id,
    name,
    root_dir: `/workspace/${name}`,
    add_dirs: [],
    created_at: "2026-07-13T00:00:00Z",
    updated_at: "2026-07-13T00:00:00Z",
  };
}

type OwnedSession = SessionPayload & { workspace_id: string };

function makeSession(id: string, workspaceId: string, name: string): OwnedSession {
  return {
    id,
    name,
    agent_name: "general",
    provider: "codex",
    workspace_id: workspaceId,
    workspace_path: `/workspace/${name}`,
    state: "active",
    badge: "running",
    attachable: true,
    available_commands: [],
    created_at: "2026-07-13T00:00:00Z",
    updated_at: "2026-07-13T00:00:00Z",
  };
}

function seedSessionRouteQueries(queryClient: QueryClient): void {
  const sessions = [
    makeSession(BENCH_SESSION_ID, BENCH_WORKSPACE_ID, "bench-ops"),
    makeSession(PRIMARY_SESSION_ID, PRIMARY_WORKSPACE_ID, "primary"),
  ];
  queryClient.setQueryData(workspaceKeys.list(), [
    makeWorkspace(BENCH_WORKSPACE_ID, "bench-ops"),
    makeWorkspace(PRIMARY_WORKSPACE_ID, "primary"),
  ]);
  for (const session of sessions) {
    queryClient.setQueryData(sessionKeys.byId(session.id), session);
    queryClient.setQueryData(sessionKeys.detail(session.workspace_id, session.id), session);
    queryClient.setQueryData(sessionKeys.transcript(session.workspace_id, session.id), {
      pages: [],
      pageParams: [],
    });
  }
}

function buildSessionDeepLinkRouter() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  seedSessionRouteQueries(queryClient);

  const rootRoute = createRootRouteWithContext<TestRouterContext>()({
    component: () => <Outlet />,
  });
  const beforeLoad = routeBeforeLoad<{
    params: { id: string; name: string };
  }>(ProductionSessionRoute);
  const loadSessionRoute = routeLoader<{
    context: TestRouterContext;
    params: { id: string };
    preload: boolean;
  }>(ProductionSessionRoute);
  const agentRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "agents/$name",
    component: () => <Outlet />,
  });
  const sessionRoute = createRoute({
    getParentRoute: () => agentRoute,
    path: "sessions/$id",
    beforeLoad,
    loader: args => loadSessionRoute(args) as Promise<AgentSessionRouteLoaderData>,
    component: SessionRouteHarness,
  });
  const routeTree = rootRoute.addChildren([agentRoute.addChildren([sessionRoute])]);
  const router = createRouter({
    routeTree,
    context: { queryClient },
    history: createMemoryHistory({
      initialEntries: [`/agents/general/sessions/${BENCH_SESSION_ID}`],
    }),
    defaultPreloadStaleTime: 0,
  });

  return { queryClient, router };

  function SessionRouteHarness() {
    const { id } = sessionRoute.useParams();
    return (
      <main>
        <p>Loaded session: {id}</p>
        {id === BENCH_SESSION_ID ? (
          <Link
            to="/agents/$name/sessions/$id"
            params={{ name: "general", id: PRIMARY_SESSION_ID }}
          >
            Open primary session
          </Link>
        ) : (
          <Link to="/agents/$name/sessions/$id" params={{ name: "general", id: BENCH_SESSION_ID }}>
            Open bench permalink
          </Link>
        )}
      </main>
    );
  }
}

describe("cross-workspace session deep-link router integration", () => {
  beforeEach(() => {
    localStorage.clear();
    useActiveWorkspaceStore.setState({ selectedWorkspaceId: BENCH_WORKSPACE_ID });
  });

  it("Should select the owning workspace when a session link crosses workspaces", async () => {
    const { router } = buildSessionDeepLinkRouter();
    render(<RouterProvider router={router} />);

    await screen.findByText(`Loaded session: ${BENCH_SESSION_ID}`);
    fireEvent.click(screen.getByRole("link", { name: "Open primary session" }));

    await screen.findByText(`Loaded session: ${PRIMARY_SESSION_ID}`);
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(`/agents/general/sessions/${PRIMARY_SESSION_ID}`);
    });
    expect(useActiveWorkspaceStore.getState().selectedWorkspaceId).toBe(PRIMARY_WORKSPACE_ID);
    expect(localStorage.getItem("agh:active-workspace")).toContain(PRIMARY_WORKSPACE_ID);
  });

  it("Should adopt the owner on every cross-workspace hop, both directions", async () => {
    const { router } = buildSessionDeepLinkRouter();
    render(<RouterProvider router={router} />);

    await screen.findByText(`Loaded session: ${BENCH_SESSION_ID}`);
    fireEvent.click(screen.getByRole("link", { name: "Open primary session" }));
    await screen.findByText(`Loaded session: ${PRIMARY_SESSION_ID}`);
    await waitFor(() => {
      expect(useActiveWorkspaceStore.getState().selectedWorkspaceId).toBe(PRIMARY_WORKSPACE_ID);
    });
    fireEvent.click(screen.getByRole("link", { name: "Open bench permalink" }));

    await screen.findByText(`Loaded session: ${BENCH_SESSION_ID}`);
    await waitFor(() => {
      expect(useActiveWorkspaceStore.getState().selectedWorkspaceId).toBe(BENCH_WORKSPACE_ID);
    });
  });

  it("Should never change the selection from a preload", async () => {
    const { queryClient } = buildSessionDeepLinkRouter();

    const data = await prefetchAgentSessionRoute({
      queryClient,
      sessionId: PRIMARY_SESSION_ID,
      preload: true,
    });

    expect(data.workspaceId).toBe(PRIMARY_WORKSPACE_ID);
    expect(useActiveWorkspaceStore.getState().selectedWorkspaceId).toBe(BENCH_WORKSPACE_ID);
  });
});
