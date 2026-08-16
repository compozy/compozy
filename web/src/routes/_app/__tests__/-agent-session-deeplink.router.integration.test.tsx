// Suite: cross-workspace session deep-link router integration
// Invariant: a deep link that misses in the active workspace resolves only the minimal owner
// projection and asks before switching — confirm delegates the switch to the active-workspace
// store and re-enters the deep link, while cancel and an unknown session keep today's not-found
// outcome with the selection untouched and no foreign session payload in a workspace-scoped cache.
// The confirmation search belongs exclusively to the session leaf: it reaches that leaf unchanged
// because the agent layout owns no search and agent detail validates its own from an index leaf.
// Boundary IN: TanStack Router, Link, route beforeLoad/loader, validated search, Query cache,
// active-workspace store, and the shipped confirm dialog.
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

import { UIProvider } from "@compozy/ui";

import { validateAgentDetailSearch, validateAgentsFleetSearch } from "@/systems/agent";
import {
  SessionWorkspaceSwitchDialog,
  sessionKeys,
  sessionOwnerKeys,
  validateSessionDeepLinkSearch,
  type SessionDeepLinkSearch,
  type SessionOwnerDialogState,
  type SessionPayload,
} from "@/systems/session";
import { statusKeys, type StatusPayload } from "@/systems/status";
import {
  ACTIVE_WORKSPACE_PERSIST_KEY,
  activeWorkspaceStore,
  setActiveWorkspaceId,
  workspaceKeys,
  type WorkspacePayload,
} from "@/systems/workspace";
import { routeBeforeLoad, routeComponent, routeLoader } from "@/test/route-options";
import type { AgentSessionRouteLoaderData } from "../-agent-session-route-loader";
import { prefetchAgentSessionRoute } from "../-agent-session-route-loader";
import type { SessionPermalinkRouteContext } from "../-session-permalink-route";
import { confirmSessionWorkspaceSwitch } from "../-session-workspace-switch-action";
import { Route as ProductionSessionPermalinkRoute } from "../session.$id";
import { Route as ProductionAgentsRoute } from "../agents";
import { Route as ProductionAgentRoute } from "../agents.$name";
import { Route as ProductionSessionRoute } from "../agents.$name.sessions.$id";

const BENCH_SESSION_ID = "sess-40e90687024bfb24";
const BENCH_WORKSPACE_ID = "ws_74a58ac2bf973937";
const PRIMARY_SESSION_ID = "sess-5ec18f5f2a13fe16";
const PRIMARY_WORKSPACE_ID = "ws_06366aad69887872";
const PRIMARY_WORKSPACE_NAME = "primary";
const UNKNOWN_SESSION_ID = "sess-000000000000dead";

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
    runtime: {
      status: "ready",
      transition: "initial_bind",
      effective: { provider: "codex" },
      selection_revision: 0,
    },
    workspace_id: workspaceId,
    workspace_path: `/workspace/${name}`,
    state: "active",
    badge: "running",
    attachable: true,
    archived_at: null,
    available_commands: [],
    pending_interactions: [],
    created_at: "2026-07-13T00:00:00Z",
    updated_at: "2026-07-13T00:00:00Z",
  };
}

const foreignSession = makeSession(
  PRIMARY_SESSION_ID,
  PRIMARY_WORKSPACE_ID,
  PRIMARY_WORKSPACE_NAME
);

/**
 * Seeds only what the active workspace legitimately owns. The foreign session stays out of every
 * cache so the pre-confirm isolation assertions cannot pass on seeded data.
 */
function seedSessionRouteQueries(queryClient: QueryClient): void {
  // Scope resolution requires `$HOME` to tell the home row from projects.
  queryClient.setQueryData(statusKeys.current(), {
    daemon: { user_home_dir: "/Users/operator" },
  } as StatusPayload);
  queryClient.setQueryData(workspaceKeys.list(), [
    makeWorkspace(BENCH_WORKSPACE_ID, "bench-ops"),
    makeWorkspace(PRIMARY_WORKSPACE_ID, PRIMARY_WORKSPACE_NAME),
  ]);
  const bench = makeSession(BENCH_SESSION_ID, BENCH_WORKSPACE_ID, "bench-ops");
  queryClient.setQueryData(sessionKeys.detail(bench.workspace_id, bench.id), bench);
  queryClient.setQueryData(sessionKeys.transcript(bench.workspace_id, bench.id), {
    pages: [],
    pageParams: [],
  });
}

function requestPathname(request: unknown): string {
  return new URL(request instanceof Request ? request.url : String(request)).pathname;
}

function fetchedPathnames(): string[] {
  return vi.mocked(globalThis.fetch).mock.calls.map(call => requestPathname(call[0]));
}

/** Serves the foreign-owner journey: active-workspace miss, owner projection, owning-workspace read. */
function stubForeignSessionBackend(): void {
  vi.mocked(globalThis.fetch).mockImplementation(request => {
    const pathname = requestPathname(request);
    if (pathname === `/api/sessions/${PRIMARY_SESSION_ID}/owner`) {
      return Promise.resolve(
        Response.json({
          session_id: PRIMARY_SESSION_ID,
          workspace_id: PRIMARY_WORKSPACE_ID,
          workspace_name: PRIMARY_WORKSPACE_NAME,
        })
      );
    }
    if (
      pathname === `/api/workspaces/${PRIMARY_WORKSPACE_ID}/sessions/${PRIMARY_SESSION_ID}` ||
      pathname === `/api/sessions/${PRIMARY_SESSION_ID}`
    ) {
      return Promise.resolve(Response.json({ session: foreignSession }));
    }
    return Promise.resolve(new Response(null, { status: 404 }));
  });
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
    params: { id: string; name: string };
    deps: { workspaceSwitch?: "confirm" | "declined" };
    preload: boolean;
  }>(ProductionSessionRoute);
  const redirectPermalinkRoute = routeBeforeLoad<{
    context: TestRouterContext;
    params: { id: string };
    search: { workspaceSwitch?: "confirm" | "declined" };
  }>(ProductionSessionPermalinkRoute);
  // Mirrors the production topology (ADR-008): both agent routes are search-neutral layouts, the
  // fleet and agent detail are index leaves owning their own search, and the session leaf is a
  // sibling of agent detail. Each layout takes production's own search ownership and component:
  // reinstating detail validation strips the session-owned key, while replacing the fleet outlet
  // with OS route sync prevents this deliberately shell-free router from mounting descendants.
  const agentsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "agents",
    validateSearch: ProductionAgentsRoute.options.validateSearch,
    beforeLoad: routeBeforeLoad<unknown>(ProductionAgentsRoute),
    component: routeComponent(ProductionAgentsRoute),
  });
  const agentsFleetRoute = createRoute({
    getParentRoute: () => agentsRoute,
    path: "/",
    validateSearch: validateAgentsFleetSearch,
    component: AgentsFleetRouteHarness,
  });
  const agentRoute = createRoute({
    getParentRoute: () => agentsRoute,
    path: "$name",
    validateSearch: ProductionAgentRoute.options.validateSearch,
    beforeLoad: routeBeforeLoad<{ params: { name: string } }>(ProductionAgentRoute),
    component: routeComponent(ProductionAgentRoute),
  });
  const agentDetailRoute = createRoute({
    getParentRoute: () => agentRoute,
    path: "/",
    validateSearch: validateAgentDetailSearch,
    component: AgentDetailRouteHarness,
  });
  const sessionRoute = createRoute({
    getParentRoute: () => agentRoute,
    path: "sessions/$id",
    validateSearch: validateSessionDeepLinkSearch,
    loaderDeps: ({ search }) => ({ workspaceSwitch: search.workspaceSwitch }),
    beforeLoad,
    loader: args => loadSessionRoute(args) as Promise<AgentSessionRouteLoaderData>,
    component: SessionRouteHarness,
  });
  const permalinkRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "session/$id",
    validateSearch: validateSessionDeepLinkSearch,
    beforeLoad: redirectPermalinkRoute,
    component: PermalinkRouteHarness,
  });
  const routeTree = rootRoute.addChildren([
    agentsRoute.addChildren([
      agentsFleetRoute,
      agentRoute.addChildren([agentDetailRoute, sessionRoute]),
    ]),
    permalinkRoute,
  ]);
  const router = createRouter({
    routeTree,
    context: { queryClient },
    history: createMemoryHistory({
      initialEntries: [initialEntry],
    }),
    defaultPreloadStaleTime: 0,
  });

  function AgentsFleetRouteHarness() {
    return <main>Agents fleet</main>;
  }

  function AgentDetailRouteHarness() {
    const params = agentDetailRoute.useParams() as { name: string };
    return <main>Agent detail: {params.name}</main>;
  }

  function SessionRouteHarness() {
    const data = sessionRoute.useLoaderData() as AgentSessionRouteLoaderData;
    const { workspaceSwitch } = sessionRoute.useSearch() as SessionDeepLinkSearch;
    const params = sessionRoute.useParams() as { name: string; id: string };
    const navigate = sessionRoute.useNavigate();

    if (data.status === "foreign") {
      return (
        <SessionWorkspaceSwitchDialog
          open={workspaceSwitch === "confirm"}
          workspaceName={data.owner.workspaceName}
          onConfirm={() =>
            confirmSessionWorkspaceSwitch(data.owner, { isGlobal: false }, () => {
              void navigate({
                to: "/agents/$name/sessions/$id",
                params,
                search: {},
                replace: true,
              });
            })
          }
          onCancel={() => {
            void navigate({ search: { workspaceSwitch: "declined" }, replace: true });
          }}
        />
      );
    }

    return (
      <main>
        <p>Loaded session: {params.id}</p>
        {params.id === BENCH_SESSION_ID ? (
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
    const context = permalinkRoute.useRouteContext() as SessionPermalinkRouteContext;
    const { workspaceSwitch } = permalinkRoute.useSearch();
    const params = permalinkRoute.useParams();
    const navigate = permalinkRoute.useNavigate();
    const owner: SessionOwnerDialogState | undefined = context.sessionOwner;

    if (!owner) {
      return <p>Permalink error: {context.permalinkError ?? "none"}</p>;
    }

    return (
      <SessionWorkspaceSwitchDialog
        open={workspaceSwitch === "confirm"}
        workspaceName={owner.workspaceName}
        onConfirm={() =>
          confirmSessionWorkspaceSwitch(owner, { isGlobal: false }, () => {
            void navigate({ to: "/session/$id", params, search: {}, replace: true });
          })
        }
        onCancel={() => {
          void navigate({ search: { workspaceSwitch: "declined" }, replace: true });
        }}
      />
    );
  }

  return {
    queryClient,
    router,
    loadSessionRoute,
    agentsFleetRoute,
    agentDetailRoute,
    sessionRoute,
  };
}

function renderRouter(router: ReturnType<typeof buildSessionDeepLinkRouter>["router"]) {
  return render(
    <UIProvider reducedMotion="always">
      <RouterProvider router={router} />
    </UIProvider>
  );
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

  it("Should resolve a foreign session link through the owner projection alone", async () => {
    stubForeignSessionBackend();
    const { router } = buildSessionDeepLinkRouter();
    renderRouter(router);

    await screen.findByText(`Loaded session: ${BENCH_SESSION_ID}`);
    fireEvent.click(screen.getByRole("link", { name: "Open primary session" }));

    await screen.findByTestId("session-workspace-switch-dialog");
    expect(
      screen.getByText(new RegExp(`belongs to ${PRIMARY_WORKSPACE_NAME}`))
    ).toBeInTheDocument();
    expect(router.state.location.pathname).toBe(`/agents/general/sessions/${PRIMARY_SESSION_ID}`);
    expect(router.state.location.search).toEqual({ workspaceSwitch: "confirm" });
    expect(fetchedPathnames()).toEqual([
      `/api/workspaces/${BENCH_WORKSPACE_ID}/sessions/${PRIMARY_SESSION_ID}`,
      `/api/sessions/${PRIMARY_SESSION_ID}/owner`,
    ]);
    expect(activeWorkspaceStore.getSnapshot().context.selectedWorkspaceId).toBe(BENCH_WORKSPACE_ID);
  });

  it("Should cache the owner projection outside every workspace-scoped session key", async () => {
    stubForeignSessionBackend();
    const { queryClient, router } = buildSessionDeepLinkRouter({
      initialEntry: `/agents/general/sessions/${PRIMARY_SESSION_ID}`,
    });
    renderRouter(router);

    await screen.findByTestId("session-workspace-switch-dialog");

    expect(queryClient.getQueryData(sessionOwnerKeys.detail(PRIMARY_SESSION_ID))).toEqual({
      session_id: PRIMARY_SESSION_ID,
      workspace_id: PRIMARY_WORKSPACE_ID,
      workspace_name: PRIMARY_WORKSPACE_NAME,
    });
    const foreignScopedEntries = queryClient
      .getQueryCache()
      .findAll({ queryKey: sessionKeys.workspace(PRIMARY_WORKSPACE_ID) })
      .filter(query => query.state.data !== undefined);
    expect(foreignScopedEntries).toHaveLength(0);
    expect(
      queryClient.getQueryData(sessionKeys.detail(PRIMARY_WORKSPACE_ID, PRIMARY_SESSION_ID))
    ).toBeUndefined();
    expect(
      queryClient.getQueryData(sessionKeys.list({ workspace: PRIMARY_WORKSPACE_ID }))
    ).toBeUndefined();
  });

  it("Should switch the active workspace on confirm and open the session", async () => {
    stubForeignSessionBackend();
    const { queryClient, router } = buildSessionDeepLinkRouter({
      initialEntry: `/agents/general/sessions/${PRIMARY_SESSION_ID}`,
    });
    renderRouter(router);

    fireEvent.click(await screen.findByTestId("session-workspace-switch-confirm"));

    await screen.findByText(`Loaded session: ${PRIMARY_SESSION_ID}`);
    expect(activeWorkspaceStore.getSnapshot().context.selectedWorkspaceId).toBe(
      PRIMARY_WORKSPACE_ID
    );
    expect(localStorage.getItem(ACTIVE_WORKSPACE_PERSIST_KEY)).toContain(PRIMARY_WORKSPACE_ID);
    expect(router.state.location.pathname).toBe(`/agents/general/sessions/${PRIMARY_SESSION_ID}`);
    expect(router.state.location.search).toEqual({});
    expect(
      queryClient.getQueryData(sessionKeys.detail(PRIMARY_WORKSPACE_ID, PRIMARY_SESSION_ID))
    ).toEqual(foreignSession);
  });

  it("Should keep the active workspace and fall back to not found on cancel", async () => {
    stubForeignSessionBackend();
    const { queryClient, router } = buildSessionDeepLinkRouter({
      initialEntry: `/agents/general/sessions/${PRIMARY_SESSION_ID}`,
    });
    renderRouter(router);

    fireEvent.click(await screen.findByTestId("session-workspace-switch-cancel"));

    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/agents/general");
    });
    expect(screen.queryByTestId("session-workspace-switch-dialog")).not.toBeInTheDocument();
    expect(activeWorkspaceStore.getSnapshot().context.selectedWorkspaceId).toBe(BENCH_WORKSPACE_ID);
    expect(localStorage.getItem(ACTIVE_WORKSPACE_PERSIST_KEY)).toContain(BENCH_WORKSPACE_ID);
    expect(
      queryClient.getQueryData(sessionKeys.detail(PRIMARY_WORKSPACE_ID, PRIMARY_SESSION_ID))
    ).toBeUndefined();
  });

  it("Should keep an unknown session on the not-found path without a dialog", async () => {
    const { router } = buildSessionDeepLinkRouter({
      initialEntry: `/agents/general/sessions/${UNKNOWN_SESSION_ID}`,
    });
    renderRouter(router);

    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/agents/general");
    });
    expect(screen.queryByTestId("session-workspace-switch-dialog")).not.toBeInTheDocument();
    expect(activeWorkspaceStore.getSnapshot().context.selectedWorkspaceId).toBe(BENCH_WORKSPACE_ID);
    expect(fetchedPathnames()).toEqual([
      `/api/workspaces/${BENCH_WORKSPACE_ID}/sessions/${UNKNOWN_SESSION_ID}`,
      `/api/sessions/${UNKNOWN_SESSION_ID}/owner`,
    ]);
  });

  it("Should offer the same confirmation on an external session permalink", async () => {
    stubForeignSessionBackend();
    const { router } = buildSessionDeepLinkRouter({
      initialEntry: `/session/${PRIMARY_SESSION_ID}`,
    });
    renderRouter(router);

    await screen.findByTestId("session-workspace-switch-dialog");
    expect(router.state.location.pathname).toBe(`/session/${PRIMARY_SESSION_ID}`);
    expect(router.state.location.search).toEqual({ workspaceSwitch: "confirm" });
    expect(fetchedPathnames()).toEqual([
      `/api/workspaces/${BENCH_WORKSPACE_ID}/sessions/${PRIMARY_SESSION_ID}`,
      `/api/sessions/${PRIMARY_SESSION_ID}/owner`,
    ]);

    fireEvent.click(screen.getByTestId("session-workspace-switch-confirm"));

    await waitFor(() => {
      expect(router.state.location.pathname).toBe(`/agents/general/sessions/${PRIMARY_SESSION_ID}`);
    });
    expect(activeWorkspaceStore.getSnapshot().context.selectedWorkspaceId).toBe(
      PRIMARY_WORKSPACE_ID
    );
  });

  it("Should keep an external permalink on today's not-found rendering when declined", async () => {
    stubForeignSessionBackend();
    const { router } = buildSessionDeepLinkRouter({
      initialEntry: `/session/${PRIMARY_SESSION_ID}`,
    });
    renderRouter(router);

    fireEvent.click(await screen.findByTestId("session-workspace-switch-cancel"));

    await screen.findByText("Permalink error: Session not found");
    expect(router.state.location.pathname).toBe(`/session/${PRIMARY_SESSION_ID}`);
    expect(activeWorkspaceStore.getSnapshot().context.selectedWorkspaceId).toBe(BENCH_WORKSPACE_ID);
  });

  it("Should surface workspace loading failure on an external session permalink", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));
    const { router } = buildSessionDeepLinkRouter({
      initialEntry: `/session/${BENCH_SESSION_ID}`,
      seedQueries: false,
    });
    renderRouter(router);

    const error = await screen.findByText(/Permalink error:/, undefined, { timeout: 5000 });
    expect(error).toHaveTextContent(/Failed to fetch workspaces/);
    expect(error).not.toHaveTextContent("Session not found");
    expect(router.state.location.pathname).toBe(`/session/${BENCH_SESSION_ID}`);
    // Status retries once on failure; only the two resolution endpoints may be hit.
    expect(fetchedPathnames()[0]).toBe("/api/workspaces");
    expect(new Set(fetchedPathnames())).toEqual(new Set(["/api/workspaces", "/api/status"]));
  });

  it("Should load a session owned by the active workspace", async () => {
    const { router } = buildSessionDeepLinkRouter();
    renderRouter(router);

    await screen.findByText(`Loaded session: ${BENCH_SESSION_ID}`);
    expect(router.state.location.pathname).toBe(`/agents/general/sessions/${BENCH_SESSION_ID}`);
    expect(router.state.location.search).toEqual({});
    expect(activeWorkspaceStore.getSnapshot().context.selectedWorkspaceId).toBe(BENCH_WORKSPACE_ID);
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });

  it("Should deliver the confirmation search to the session leaf unchanged by its ancestors", async () => {
    stubForeignSessionBackend();
    const { router, agentDetailRoute, sessionRoute } = buildSessionDeepLinkRouter({
      initialEntry: `/agents/general/sessions/${PRIMARY_SESSION_ID}?workspaceSwitch=confirm`,
    });
    renderRouter(router);

    await screen.findByTestId("session-workspace-switch-dialog");
    // The router normalizes the URL from every validator in the matched chain. Keeping agent-detail
    // validation on its sibling index leaf prevents it from rewriting this deep link with defaults
    // the session leaf does not own. Only the session leaf's own key survives here.
    expect(router.state.location.search).toEqual({ workspaceSwitch: "confirm" });
    expect(router.state.matches.map(match => match.routeId)).not.toContain(agentDetailRoute.id);
    expect(router.state.matches.at(-1)?.routeId).toBe(sessionRoute.id);
    expect(router.state.matches.at(-1)?.search).toEqual({ workspaceSwitch: "confirm" });
  });

  it("Should resolve direct agent detail through the index leaf that owns its search", async () => {
    const { router, agentDetailRoute } = buildSessionDeepLinkRouter({
      initialEntry: "/agents/general?tab=sessions&filter=nope",
    });
    renderRouter(router);

    await screen.findByText("Agent detail: general");
    expect(router.state.location.pathname).toBe("/agents/general");
    expect(router.state.matches.at(-1)?.routeId).toBe(agentDetailRoute.id);
    expect(router.state.matches.at(-1)?.search).toEqual({
      tab: "sessions",
      file: "agent",
      filter: "all",
    });
  });

  it("Should resolve the agents fleet through its own index leaf below a layout that mounts descendants", async () => {
    const { router, agentsFleetRoute } = buildSessionDeepLinkRouter({
      initialEntry: "/agents?q=%20fraud%20&status=nope&view=rows",
    });
    renderRouter(router);

    // Rendering at all proves the outer layout hands its outlet to descendants — the defect that
    // kept every agent route below it, including the session confirmation, from ever mounting.
    await screen.findByText("Agents fleet");
    expect(router.state.location.pathname).toBe("/agents");
    expect(router.state.matches.at(-1)?.routeId).toBe(agentsFleetRoute.id);
    expect(router.state.matches.at(-1)?.search).toEqual({ q: "fraud", view: "rows" });
  });

  it("Should return the minimal dialog state from a foreign preload without navigating", async () => {
    stubForeignSessionBackend();
    const { queryClient, loadSessionRoute } = buildSessionDeepLinkRouter();

    const data = await prefetchAgentSessionRoute({
      queryClient,
      sessionId: PRIMARY_SESSION_ID,
    });

    expect(data).toEqual({
      status: "foreign",
      owner: {
        sessionId: PRIMARY_SESSION_ID,
        workspaceId: PRIMARY_WORKSPACE_ID,
        workspaceName: PRIMARY_WORKSPACE_NAME,
      },
    });
    await expect(
      loadSessionRoute({
        context: { queryClient },
        params: { name: "general", id: PRIMARY_SESSION_ID },
        deps: {},
        preload: true,
      })
    ).resolves.toEqual(data);
    expect(activeWorkspaceStore.getSnapshot().context.selectedWorkspaceId).toBe(BENCH_WORKSPACE_ID);
  });
});
