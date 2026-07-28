// Suite: cross-workspace session deep-link router integration
// Invariant: session navigation remains scoped to the active workspace; a foreign session id
// resolves as not found without changing selection or reading a cache owned by another workspace.
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
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { sessionKeys, type SessionPayload } from "@/systems/session";
import {
  activeWorkspaceStore,
  setActiveWorkspaceId,
  workspaceKeys,
  type WorkspacePayload,
} from "@/systems/workspace";
import { routeBeforeLoad, routeLoader } from "@/test/route-options";
import type { AgentSessionRouteLoaderData } from "../-agent-session-route-loader";
import { prefetchAgentSessionRoute } from "../-agent-session-route-loader";
import { Route as ProductionSessionPermalinkRoute } from "../session.$id";
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
    queryClient.setQueryData(sessionKeys.detail(session.workspace_id, session.id), session);
    queryClient.setQueryData(sessionKeys.transcript(session.workspace_id, session.id), {
      pages: [],
      pageParams: [],
    });
  }
}

function buildSessionDeepLinkRouter({
  initialEntry = `/agents/general/sessions/${BENCH_SESSION_ID}`,
  seedQueries = true,
}: {
  initialEntry?: string;
  seedQueries?: boolean;
} = {}) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  if (seedQueries) {
    seedSessionRouteQueries(queryClient);
  }

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
  const redirectPermalinkRoute = routeBeforeLoad<{
    context: TestRouterContext;
    params: { id: string };
  }>(ProductionSessionPermalinkRoute);
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
  const permalinkRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "session/$id",
    beforeLoad: redirectPermalinkRoute,
    component: PermalinkRouteHarness,
  });
  const routeTree = rootRoute.addChildren([agentRoute.addChildren([sessionRoute]), permalinkRoute]);
  const router = createRouter({
    routeTree,
    context: { queryClient },
    history: createMemoryHistory({
      initialEntries: [initialEntry],
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

  function PermalinkRouteHarness() {
    const { permalinkError } = permalinkRoute.useRouteContext() as { permalinkError?: string };
    return <p>Permalink error: {permalinkError ?? "none"}</p>;
  }
}

describe("cross-workspace session deep-link router integration", () => {
  beforeEach(() => {
    localStorage.clear();
    setActiveWorkspaceId(BENCH_WORKSPACE_ID);
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(null, { status: 404 })));
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("Should reject a foreign session link without changing the active workspace", async () => {
    const { router } = buildSessionDeepLinkRouter();
    render(<RouterProvider router={router} />);

    await screen.findByText(`Loaded session: ${BENCH_SESSION_ID}`);
    fireEvent.click(screen.getByRole("link", { name: "Open primary session" }));

    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/agents/general");
    });
    expect(activeWorkspaceStore.getSnapshot().context.selectedWorkspaceId).toBe(BENCH_WORKSPACE_ID);
    expect(localStorage.getItem("compozy:active-workspace:v2")).toContain(BENCH_WORKSPACE_ID);
    expect(vi.mocked(globalThis.fetch)).toHaveBeenCalledTimes(1);
    const request = vi.mocked(globalThis.fetch).mock.calls[0]?.[0];
    expect(new URL(request instanceof Request ? request.url : String(request)).pathname).toBe(
      `/api/workspaces/${BENCH_WORKSPACE_ID}/sessions/${PRIMARY_SESSION_ID}`
    );
  });

  it("Should keep an external foreign session permalink inside the active workspace", async () => {
    const foreignSession = makeSession(PRIMARY_SESSION_ID, PRIMARY_WORKSPACE_ID, "primary");
    vi.mocked(globalThis.fetch).mockImplementation(request => {
      const url = new URL(request instanceof Request ? request.url : String(request));
      if (url.pathname === `/api/sessions/${PRIMARY_SESSION_ID}`) {
        return Promise.resolve(Response.json({ session: foreignSession }));
      }
      return Promise.resolve(new Response(null, { status: 404 }));
    });
    const { router } = buildSessionDeepLinkRouter({
      initialEntry: `/session/${PRIMARY_SESSION_ID}`,
    });
    render(<RouterProvider router={router} />);

    await screen.findByText("Permalink error: Session not found");

    expect(router.state.location.pathname).toBe(`/session/${PRIMARY_SESSION_ID}`);
    expect(activeWorkspaceStore.getSnapshot().context.selectedWorkspaceId).toBe(BENCH_WORKSPACE_ID);
    expect(vi.mocked(globalThis.fetch)).toHaveBeenCalledTimes(1);
    const request = vi.mocked(globalThis.fetch).mock.calls[0]?.[0];
    expect(new URL(request instanceof Request ? request.url : String(request)).pathname).toBe(
      `/api/workspaces/${BENCH_WORKSPACE_ID}/sessions/${PRIMARY_SESSION_ID}`
    );
  });

  it("Should surface workspace loading failure on an external session permalink", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));
    const { router } = buildSessionDeepLinkRouter({
      initialEntry: `/session/${BENCH_SESSION_ID}`,
      seedQueries: false,
    });
    render(<RouterProvider router={router} />);

    const error = await screen.findByText(/Permalink error:/);
    expect(error).toHaveTextContent(/Failed to fetch workspaces/);
    expect(error).not.toHaveTextContent("Session not found");
    expect(router.state.location.pathname).toBe(`/session/${BENCH_SESSION_ID}`);
    expect(vi.mocked(globalThis.fetch)).toHaveBeenCalledTimes(1);
    const request = vi.mocked(globalThis.fetch).mock.calls[0]?.[0];
    expect(new URL(request instanceof Request ? request.url : String(request)).pathname).toBe(
      "/api/workspaces"
    );
  });

  it("Should load a session owned by the active workspace", async () => {
    const { router } = buildSessionDeepLinkRouter();
    render(<RouterProvider router={router} />);

    await screen.findByText(`Loaded session: ${BENCH_SESSION_ID}`);
    expect(router.state.location.pathname).toBe(`/agents/general/sessions/${BENCH_SESSION_ID}`);
    expect(activeWorkspaceStore.getSnapshot().context.selectedWorkspaceId).toBe(BENCH_WORKSPACE_ID);
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });

  it("Should keep foreign preloads scoped without changing the selection", async () => {
    const { queryClient } = buildSessionDeepLinkRouter();

    const data = await prefetchAgentSessionRoute({
      queryClient,
      sessionId: PRIMARY_SESSION_ID,
    });

    expect(data.workspaceId).toBeNull();
    expect(activeWorkspaceStore.getSnapshot().context.selectedWorkspaceId).toBe(BENCH_WORKSPACE_ID);
    expect(vi.mocked(globalThis.fetch)).toHaveBeenCalledTimes(1);
  });
});
