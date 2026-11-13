// Suite: routing coordinator
// Invariant: URL reconciliation mutates only the daemon-facing controller, route intent survives
// hydration, command fences, and in-flight authority races, and user causes produce exactly one
// history write.
// Owning layer: the sole URL ↔ window-manager bridge.
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  defaultParseSearch,
  redirect,
} from "@tanstack/react-router";
import { describe, expect, it, vi } from "vitest";

import type { OsHydration, OsOpenTarget, OsWindow, OsWindowRoute } from "../os-types";
import { connectRouterEntries, RouterEntryBridge } from "../router-entry-bridge";
import { RoutingCoordinator, type OsRouterPort } from "../routing-coordinator";
import { RoutingEntryLedger } from "../routing-entry-ledger";
import { routeReplacesWindowContent } from "../window-navigation-guard";
import type { WindowManagerClientView } from "../window-manager-types";

function route(pathname: string, search: Record<string, unknown> = {}): OsWindowRoute {
  return { pathname, search };
}

/** Window IDs are opaque (ADR-010); the store mock keys them deterministically per fixture. */
function testWindowId(app: OsWindow["app"], instanceKey?: string | null): string {
  return instanceKey ? `w-${app}-${instanceKey}` : `w-${app}`;
}

function windowFixture(
  app: OsWindow["app"],
  pathname: string,
  instanceKey: string | null = null,
  id = testWindowId(app, instanceKey)
): OsWindow {
  return {
    id,
    app,
    instanceKey,
    route: route(pathname),
    navStack: [],
    desktopId: "desktop:main",
    tabId: `tab:${id}`,
    nodeId: null,
    pinned: false,
    inSplit: false,
    tabActive: false,
    shown: false,
  };
}

function createStore(initialWindows: readonly OsWindow[] = []) {
  const windows = Object.fromEntries(initialWindows.map(window => [window.id, window]));
  let focusedId = initialWindows.at(-1)?.id ?? null;
  let focusOrder = initialWindows.map(window => window.id).reverse();
  const focus = (id: string) => {
    focusedId = id;
    focusOrder = [id, ...focusOrder.filter(candidate => candidate !== id)];
  };
  let client: WindowManagerClientView | null = {
    workspaceId: "workspace:test",
    clientId: "client:web",
    activeDesktopId: "desktop:main",
    focusedWindowId: focusedId,
    focusOrder,
    activeTab: {},
    connectedAt: "2026-07-22T00:00:00Z",
    presentationRevision: 1,
  };
  let hydration: OsHydration = "live";
  let lifecycleResult: Promise<boolean> | null = null;
  let resolveLifecycle: ((accepted: boolean) => void) | null = null;

  const commandOutcome = (apply: () => void) => {
    const completion = lifecycleResult ?? Promise.resolve(true);
    if (lifecycleResult === null) apply();
    else void completion.then(accepted => accepted && apply());
    return { accepted: true, completion };
  };

  const openOrFocus = vi.fn((target: OsOpenTarget) => {
    const instanceKey = target.instanceKey ?? null;
    const matchesTarget = (window: OsWindow) =>
      window.app === target.app && (instanceKey === null || window.instanceKey === instanceKey);
    const existing =
      focusOrder
        .map(id => windows[id])
        .find((window): window is OsWindow => window !== undefined && matchesTarget(window)) ??
      Object.values(windows)
        .sort((left, right) => left.id.localeCompare(right.id))
        .find(matchesTarget);
    const id = existing?.id ?? testWindowId(target.app, target.instanceKey);
    return {
      windowId: id,
      ...commandOutcome(() => {
        const current = windows[id];
        windows[id] =
          current ??
          windowFixture(
            target.app,
            target.route?.pathname ?? (target.app === "dashboard" ? "/" : `/${target.app}`),
            target.instanceKey ?? null
          );
        if (target.route) windows[id] = { ...windows[id], route: target.route };
        focus(id);
      }),
    };
  });
  const focusWindow = vi.fn((id: string) =>
    commandOutcome(() => {
      if (windows[id]) focus(id);
    })
  );
  const closeWindow = vi.fn(async (id: string) => {
    if (lifecycleResult !== null && !(await lifecycleResult)) return false;
    delete windows[id];
    focusOrder = focusOrder.filter(candidate => candidate !== id);
    if (focusedId === id) {
      const successor = focusOrder[0] ?? null;
      if (successor === null) focusedId = null;
      else focus(successor);
    }
    return true;
  });
  const navigateWindow = vi.fn((id: string, nextRoute: OsWindowRoute) =>
    commandOutcome(() => {
      if (windows[id]) windows[id] = { ...windows[id], route: nextRoute };
    })
  );
  const popWindowRoute = vi.fn((id: string) =>
    commandOutcome(() => {
      const win = windows[id];
      const parent = win?.navStack.at(-1);
      if (win && parent) {
        windows[id] = { ...win, route: parent, navStack: win.navStack.slice(0, -1) };
      }
    })
  );
  const retargetWindow = vi.fn((id: string, instanceKey: string, nextRoute: OsWindowRoute) =>
    commandOutcome(() => {
      if (windows[id]) {
        windows[id] = { ...windows[id], instanceKey, route: nextRoute, navStack: [] };
      }
    })
  );
  const resetWorkspace = () => {
    for (const id of Object.keys(windows)) delete windows[id];
    focusedId = null;
    focusOrder = [];
  };
  const setAuthoritativeFocus = (id: string, nextRoute?: OsWindowRoute) => {
    if (nextRoute && windows[id]) windows[id] = { ...windows[id], route: nextRoute };
    focus(id);
  };
  const setClientConnected = (connected: boolean) => {
    client = connected
      ? {
          workspaceId: "workspace:test",
          clientId: "client:web",
          activeDesktopId: "desktop:main",
          focusedWindowId: focusedId,
          focusOrder,
          activeTab: {},
          connectedAt: "2026-07-22T00:00:00Z",
          presentationRevision: 2,
        }
      : null;
  };
  const state = {
    get windows() {
      return windows;
    },
    get focusedId() {
      return focusedId;
    },
    get client() {
      return client;
    },
    get hydration() {
      return hydration;
    },
    openOrFocus,
    focusWindow,
    closeWindow,
    navigateWindow,
    retargetWindow,
    popWindowRoute,
  };

  return {
    getState: () => state,
    openOrFocus,
    focusWindow,
    closeWindow,
    navigateWindow,
    retargetWindow,
    popWindowRoute,
    deferLifecycle: () => {
      lifecycleResult = new Promise<boolean>(resolve => {
        resolveLifecycle = resolve;
      });
    },
    settleLifecycle: (accepted: boolean) => {
      resolveLifecycle?.(accepted);
      resolveLifecycle = null;
    },
    resetWorkspace,
    setAuthoritativeFocus,
    setClientConnected,
    setHydration: (next: OsHydration) => {
      hydration = next;
    },
    resumeLifecycle: () => {
      lifecycleResult = null;
    },
    spies: {
      openOrFocus,
      focusWindow,
      closeWindow,
      navigateWindow,
      retargetWindow,
      popWindowRoute,
    },
  };
}

/** Entry keys are unique across every history in the suite, as the router's random keys are. */
let entryKeySeq = 0;

/**
 * The router's history as the bridge reports it to the ledger: every push and replace gets a fresh
 * key, Back restores the key of the entry it lands on.
 */
function createHistory(entries: RoutingEntryLedger) {
  const stack: { key: string; pathname: string }[] = [];
  let index = -1;
  const commit = (
    mode: "push" | "replace",
    kind: "write" | "push" | "replace",
    pathname: string,
    continuation = false
  ) => {
    entryKeySeq += 1;
    const key = `entry-${entryKeySeq}`;
    if (mode === "push" || index < 0) {
      stack.splice(index + 1, stack.length, { key, pathname });
      index += 1;
    } else {
      stack[index] = { key, pathname };
    }
    if (kind === "replace") entries.observe({ kind, key, pathname, continuation });
    else entries.observe(kind === "push" ? { kind, key } : { kind, key, pathname });
    return key;
  };
  const currentPathname = () => stack[index]?.pathname ?? "/";
  return {
    write: (mode: "push" | "replace", pathname: string) => commit(mode, "write", pathname),
    /** An operator link: a push the coordinator did not write. */
    link: (pathname = "/") => commit("push", "push", pathname),
    /**
     * The router's redirect or canonicalization of the current entry (it bypasses blockers, the
     * mark the bridge reads), which keeps its path by default.
     */
    redirect: (pathname = currentPathname()) => commit("replace", "replace", pathname, true),
    /** An app's or the operator's replace (a filter, a tab in the route): it answers blockers. */
    replace: (pathname = currentPathname()) => commit("replace", "replace", pathname),
    back: () => {
      index -= 1;
      const key = stack[index]?.key;
      if (key === undefined) throw new Error("history has no earlier entry");
      entries.observe({ kind: "traverse", key });
      return key;
    },
    resolve: (key: string) => entries.observe({ kind: "resolved", key }),
    /** The key of the entry history now shows; `null` before any entry was written. */
    current: (): string | null => stack[index]?.key ?? null,
  };
}

function createCoordinator(initialWindows: readonly OsWindow[] = []) {
  const store = createStore(initialWindows);
  const entries = new RoutingEntryLedger();
  const history = createHistory(entries);
  // The history reports each write synchronously, inside the router's push or replace.
  const router: OsRouterPort = {
    navigate: vi.fn((route: OsWindowRoute) => void history.write("push", route.pathname)),
    replace: vi.fn((route: OsWindowRoute) => void history.write("replace", route.pathname)),
  };
  return {
    coordinator: new RoutingCoordinator(store, router, entries),
    router,
    store,
    history,
  };
}

/**
 * The coordinator over the real router, history and bridge, with a navigation blocker registered
 * as a dirty agent editor's unsaved guard does. Session routes canonicalize with a same-path
 * loader redirect that adds the workspace binding, as the session permalink does.
 */
function createBridgedCoordinator(
  initialWindows: readonly OsWindow[],
  blockerFn: (args: {
    nextLocation: { pathname: string; search: string };
  }) => boolean | Promise<boolean>
) {
  const rootRoute = createRootRoute();
  const sessionRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/agents/general/sessions/$id",
    beforeLoad: ({ location }) => {
      if (!("workspace" in location.search)) {
        throw redirect({ href: `${location.pathname}?workspace=w`, replace: true });
      }
    },
  });
  // Tasks' preload can be held open, so a filter can change while the coordinator's write loads.
  const tasksLoad: { held: Promise<void> | null } = { held: null };
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      createRoute({
        getParentRoute: () => rootRoute,
        path: "/tasks",
        loader: () => tasksLoad.held ?? undefined,
      }),
      createRoute({ getParentRoute: () => rootRoute, path: "/settings" }),
      sessionRoute,
    ]),
    history: createMemoryHistory({ initialEntries: ["/tasks"] }),
  });
  const store = createStore(initialWindows);
  const entries = new RoutingEntryLedger();
  const bridge = new RouterEntryBridge(
    router.history,
    route => `${route.pathname}${router.options.stringifySearch(route.search)}`
  );
  // As the mounted Transitioner does: the router loads each history change.
  const stopLoading = router.history.subscribe(() => void router.load());
  const stopEntries = connectRouterEntries(router, bridge, entries);
  const releaseBlocker = router.history.block({ blockerFn, enableBeforeUnload: false });
  // The key of every entry a push committed, so a test names a write after its redirect landed.
  const pushed: string[] = [];
  const stopPushLog = router.history.subscribe(({ location, action }) => {
    const key = location.state.__TSR_key;
    if (action.type === "PUSH" && key !== undefined) pushed.push(key);
  });
  return {
    coordinator: new RoutingCoordinator(store, bridge.port, entries),
    router,
    store,
    currentKey: () => router.history.location.state.__TSR_key ?? null,
    lastPushKey: () => pushed.at(-1) ?? null,
    /** Holds every Tasks load until the returned release. */
    holdTasks: () => {
      let release: () => void = () => undefined;
      tasksLoad.held = new Promise<void>(resolve => {
        release = resolve;
      });
      return () => {
        tasksLoad.held = null;
        release();
      };
    },
    disconnect: () => {
      releaseBlocker();
      stopPushLog();
      stopEntries();
      stopLoading();
    },
  };
}

const registeredBlockers = [
  ["lets every navigation through", () => false],
  ["holds every navigation", () => new Promise<boolean>(() => undefined)],
] as const;

describe("RoutingCoordinator", () => {
  it.each(["/loops", "/loop-runs", "/loops/studio-onboarding-review"])(
    "Should honor the breadcrumb destination when the history parent is %s",
    async parent => {
      const loop = windowFixture("loops", "/loop-runs");
      loop.navStack = [route(parent)];
      const { coordinator, router, store } = createCoordinator([loop]);
      coordinator.reportRouteMatch(loop.route);
      coordinator.completeHydration();

      coordinator.noteNavigateMode("pop");
      coordinator.reportRouteMatch(route("/loops"));
      await Promise.resolve();

      const settled = store.getState().windows[loop.id];
      expect(settled.route).toEqual(route("/loops"));
      expect(settled.navStack).toEqual(parent === "/loops" ? [] : [route(parent)]);
      expect(router.navigate).not.toHaveBeenCalled();
      expect(router.replace).not.toHaveBeenCalled();
    }
  );

  it("Should hold a deep link until hydration then reconcile it without writing history", () => {
    const { coordinator, router, store } = createCoordinator();

    coordinator.reportRouteMatch(route("/tasks/task-42"));
    expect(store.spies.openOrFocus).not.toHaveBeenCalled();

    coordinator.completeHydration();

    expect(store.spies.openOrFocus).toHaveBeenCalledWith({
      app: "tasks",
      instanceKey: undefined,
      route: route("/tasks/task-42"),
    });
    expect(router.navigate).not.toHaveBeenCalled();
    expect(router.replace).not.toHaveBeenCalled();
  });

  it("Should preserve initial route intent through workspace binding and consume it once", () => {
    const { coordinator, router, store } = createCoordinator();
    const deepLink = route("/tasks/task-42", { panel: "activity" });

    coordinator.reportRouteMatch(deepLink);
    coordinator.beginWorkspaceSwitch();
    coordinator.completeHydration();

    expect(store.spies.openOrFocus).toHaveBeenCalledOnce();
    expect(store.spies.openOrFocus).toHaveBeenCalledWith({
      app: "tasks",
      instanceKey: undefined,
      route: deepLink,
    });

    store.resetWorkspace();
    coordinator.beginWorkspaceSwitch();
    coordinator.completeHydration();

    expect(store.spies.openOrFocus).toHaveBeenCalledOnce();
    expect(router.navigate).toHaveBeenCalledOnce();
    expect(router.navigate).toHaveBeenCalledWith(route("/"));
  });

  it("Should reconcile an existing route and focus through one semantic transition", () => {
    const tasks = windowFixture("tasks", "/tasks");
    const settings = windowFixture("settings", "/settings/general");
    const { coordinator, router, store } = createCoordinator([tasks, settings]);
    coordinator.completeHydration();
    vi.mocked(router.replace).mockClear();

    coordinator.reportRouteMatch(route("/tasks/task-42"));

    expect(store.spies.openOrFocus).toHaveBeenCalledOnce();
    expect(store.spies.openOrFocus).toHaveBeenCalledWith({
      app: "tasks",
      instanceKey: undefined,
      route: route("/tasks/task-42"),
    });
    expect(store.getState().focusedId).toBe(tasks.id);
    expect(store.getState().windows[tasks.id]?.route).toEqual(route("/tasks/task-42"));
    expect(store.spies.focusWindow).not.toHaveBeenCalled();
    expect(store.spies.navigateWindow).not.toHaveBeenCalled();
    expect(router.navigate).not.toHaveBeenCalled();
    expect(router.replace).not.toHaveBeenCalled();
  });

  it("Should carry a same-window link's search into the window location (Automations create link)", () => {
    // The Automations empty state links to `/automations?create=1&start=schedule`
    // from inside its own window; the editor seed is read from the WM location.
    const automations = windowFixture("automations", "/automations");
    const { coordinator, store } = createCoordinator([automations]);
    coordinator.completeHydration();

    coordinator.reportRouteMatch(route("/automations", { create: "1", start: "schedule" }));

    expect(store.getState().windows[automations.id]?.route).toEqual(
      route("/automations", { create: "1", start: "schedule" })
    );
  });

  it("[UT-042] Should reconcile a Tasks deep link through the most-recently-focused instance", () => {
    const older = windowFixture("tasks", "/tasks", null, "w-tasks-older");
    const mostRecent = windowFixture("tasks", "/tasks", null, "w-tasks-most-recent");
    const { coordinator, router, store } = createCoordinator([older, mostRecent]);
    coordinator.completeHydration();
    vi.mocked(router.replace).mockClear();
    const deepLink = route("/tasks/task-42", { view: "board" });

    coordinator.reportRouteMatch(deepLink);

    expect(store.spies.openOrFocus).toHaveBeenCalledWith({
      app: "tasks",
      instanceKey: undefined,
      route: deepLink,
    });
    expect(store.getState().focusedId).toBe(mostRecent.id);
    expect(store.getState().windows[mostRecent.id]?.route).toEqual(deepLink);
    expect(store.getState().windows[older.id]?.route).toEqual(older.route);
    expect(router.navigate).not.toHaveBeenCalled();
    expect(router.replace).not.toHaveBeenCalled();
  });

  it("Should keep an accepted route intent authoritative until reconciliation completes", async () => {
    const tasks = windowFixture("tasks", "/tasks");
    const settings = windowFixture("settings", "/settings/general");
    const { coordinator, router, store } = createCoordinator([tasks, settings]);
    coordinator.completeHydration();
    vi.mocked(router.replace).mockClear();
    store.deferLifecycle();

    coordinator.reportRouteMatch(route("/tasks/task-42"));
    coordinator.reportAuthoritativeState();
    coordinator.reportRouteMatch(route("/tasks/task-42"));

    expect(router.replace).not.toHaveBeenCalled();
    expect(store.spies.openOrFocus).toHaveBeenCalledOnce();

    store.settleLifecycle(true);
    await vi.waitFor(() => expect(store.getState().focusedId).toBe(tasks.id));

    expect(router.replace).not.toHaveBeenCalled();
    expect(router.navigate).not.toHaveBeenCalled();
  });

  it("Should preserve an unavailable route intent for authoritative recovery", () => {
    const tasks = windowFixture("tasks", "/tasks");
    const settings = windowFixture("settings", "/settings/general");
    const { coordinator, router, store } = createCoordinator([tasks, settings]);
    coordinator.completeHydration();
    vi.mocked(router.replace).mockClear();
    store.openOrFocus.mockReturnValueOnce({
      windowId: tasks.id,
      accepted: false,
      completion: Promise.resolve(false),
    });
    const deepLink = route("/tasks/task-42");

    coordinator.reportRouteMatch(deepLink);

    expect(router.replace).not.toHaveBeenCalled();

    coordinator.reportAuthoritativeState();

    expect(store.openOrFocus).toHaveBeenCalledTimes(2);
  });

  it("Should wait for live hydration before opening a session route", () => {
    const { coordinator, router, store } = createCoordinator();
    const deepLink = route("/agents/general/sessions/sess-created");
    coordinator.completeHydration();
    store.setHydration("pending");

    coordinator.reportRouteMatch(deepLink);

    expect(store.openOrFocus).not.toHaveBeenCalled();
    expect(router.replace).not.toHaveBeenCalled();

    store.setHydration("live");
    coordinator.reportAuthoritativeState();

    expect(store.openOrFocus).toHaveBeenCalledOnce();
    expect(store.openOrFocus).toHaveBeenCalledWith({
      app: "session",
      instanceKey: "sess-created",
      route: deepLink,
    });
  });

  it("Should retry a session route after an accepted open finishes unapplied", async () => {
    const { coordinator, router, store } = createCoordinator();
    const deepLink = route("/agents/general/sessions/sess-created");
    coordinator.completeHydration();
    store.deferLifecycle();

    coordinator.reportRouteMatch(deepLink);
    store.settleLifecycle(false);
    await vi.waitFor(() => expect(store.openOrFocus).toHaveBeenCalledOnce());

    expect(router.replace).not.toHaveBeenCalled();

    store.resumeLifecycle();
    coordinator.reportAuthoritativeState();

    const createdSession = Object.values(store.getState().windows).find(
      window => window.app === "session" && window.instanceKey === "sess-created"
    );
    expect(store.openOrFocus).toHaveBeenCalledTimes(2);
    expect(createdSession).toBeDefined();
    expect(store.getState().focusedId).toBe(createdSession?.id);
  });

  it("Should preserve an explicit route while its semantic focus is pending", async () => {
    const session = windowFixture(
      "session",
      "/agents/general/sessions/sess-route-intent",
      "sess-route-intent"
    );
    const marketplace = windowFixture("marketplace", "/marketplace/skills");
    const { coordinator, router, store } = createCoordinator([session, marketplace]);
    coordinator.completeHydration();
    vi.mocked(router.replace).mockClear();
    store.deferLifecycle();

    coordinator.reportRouteMatch(session.route);
    coordinator.reportAuthoritativeState();

    expect(store.spies.openOrFocus).toHaveBeenCalledOnce();
    expect(router.replace).not.toHaveBeenCalled();

    store.settleLifecycle(true);
    await vi.waitFor(() => expect(store.getState().focusedId).toBe(session.id));
    coordinator.reportAuthoritativeState();

    expect(router.replace).not.toHaveBeenCalled();
  });

  it("Should consume a desktop default-view intent and focus Home after hydration", async () => {
    const settings = windowFixture("settings", "/settings/general");
    const { coordinator, router, store } = createCoordinator([settings]);
    store.setHydration("pending");

    coordinator.reportRouteMatch(route("/", { _compozy_desktop_default: 1 }));
    coordinator.completeHydration();
    coordinator.reportRouteMatch(route("/"));
    coordinator.beginWorkspaceSwitch();
    coordinator.reportRouteMatch(route("/"));
    coordinator.completeHydration();

    expect(router.replace).toHaveBeenCalledTimes(2);
    expect(router.replace).toHaveBeenLastCalledWith(route("/"));
    expect(store.spies.openOrFocus).not.toHaveBeenCalled();

    store.setHydration("live");
    coordinator.reportAuthoritativeState();

    expect(store.spies.openOrFocus).toHaveBeenCalledWith({ app: "dashboard", route: route("/") });
    await vi.waitFor(() => expect(store.getState().focusedId).toBe("w-dashboard"));
  });

  it("Should leave an empty desktop alone when the desktop URL lands while Home lives on another desktop", () => {
    const elsewhere = { ...windowFixture("dashboard", "/"), desktopId: "desktop:other" };
    const focusedHere = windowFixture("tasks", "/tasks");
    const { coordinator, store } = createCoordinator([elsewhere, focusedHere]);
    coordinator.completeHydration();

    // Switching to an empty desktop reports `/`: focusing the other desktop's
    // Home would switch the client straight back.
    coordinator.reportRouteMatch(route("/"));

    expect(store.spies.openOrFocus).not.toHaveBeenCalled();
    expect(store.spies.focusWindow).not.toHaveBeenCalled();
  });

  it("Should still focus Home for the desktop URL when it lives on the active desktop", async () => {
    const home = windowFixture("dashboard", "/");
    const tasks = windowFixture("tasks", "/tasks");
    const { coordinator, store } = createCoordinator([home, tasks]);
    coordinator.completeHydration();

    coordinator.reportRouteMatch(route("/"));

    expect(store.spies.openOrFocus).toHaveBeenCalledWith({ app: "dashboard", route: route("/") });
    await vi.waitFor(() => expect(store.getState().focusedId).toBe(home.id));
  });

  it("Should push exactly once after a user opens an app", async () => {
    const { coordinator, router } = createCoordinator();
    coordinator.completeHydration();

    const id = await coordinator.userOpen({ app: "settings", route: route("/settings/layouts") });

    expect(id).toBe("w-settings");
    expect(router.navigate).toHaveBeenCalledOnce();
    expect(router.navigate).toHaveBeenCalledWith(route("/settings/layouts"));
  });

  it("Should write one entry when a frame shows an open-or-focus before it settles [#1655]", async () => {
    const files = windowFixture("files", "/files/project-a");
    const tasks = windowFixture("tasks", "/tasks");
    const { coordinator, router, store, history } = createCoordinator([files, tasks]);
    coordinator.completeHydration();
    coordinator.reportRouteMatch(tasks.route, history.current());
    const tasksEntry = history.current();
    vi.mocked(router.replace).mockClear();
    store.deferLifecycle();

    // Dock Files: open-or-focus of the project's Files window.
    const pending = coordinator.userOpen({ app: "files", route: files.route });
    // The stream shows the new focus before the command's answer.
    store.setAuthoritativeFocus(files.id);
    coordinator.reportAuthoritativeState();
    expect(router.replace).not.toHaveBeenCalled();
    store.settleLifecycle(true);
    await expect(pending).resolves.toBe(files.id);

    expect(router.navigate).toHaveBeenCalledOnce();
    expect(router.navigate).toHaveBeenCalledWith(files.route);
    expect(router.replace).not.toHaveBeenCalled();

    // One Back returns to Tasks.
    store.resumeLifecycle();
    coordinator.reportRouteMatch(files.route, history.current());
    expect(history.back()).toBe(tasksEntry);
    coordinator.reportRouteMatch(tasks.route, tasksEntry);
    await vi.waitFor(() => expect(store.getState().focusedId).toBe(tasks.id));
  });

  it("Should write nothing when an open-or-focus names the focused, shown window the URL names [#1655]", async () => {
    const files = { ...windowFixture("files", "/files/project-a"), tabActive: true, shown: true };
    const { coordinator, router, store } = createCoordinator([files]);
    coordinator.completeHydration();
    vi.mocked(router.replace).mockClear();

    await expect(coordinator.userOpen({ app: "files" })).resolves.toBe(files.id);

    expect(store.getState().focusedId).toBe(files.id);
    expect(router.navigate).not.toHaveBeenCalled();
    expect(router.replace).not.toHaveBeenCalled();
  });

  it("Should hand the keyboard only to focus changes this client asked for [#1655]", async () => {
    const tasks = windowFixture("tasks", "/tasks");
    const settings = windowFixture("settings", "/settings/general");
    const agents = windowFixture("agents", "/agents");
    const { coordinator, store, history } = createCoordinator([tasks, settings, agents]);
    coordinator.completeHydration();
    const request = vi.spyOn(coordinator.keyboardFocus, "request");

    // A pane's pointer or focus activation keeps the focus the click gave.
    await coordinator.userFocus(tasks.id);
    const tasksEntry = history.current();
    coordinator.reportRouteMatch(tasks.route, tasksEntry);
    // Another client or the CLI focuses Settings: the marker moves, the caret stays.
    store.setAuthoritativeFocus(settings.id);
    coordinator.reportAuthoritativeState();
    expect(request).not.toHaveBeenCalled();

    // A chord (tab next, "Focus last window"), the dock and the palette carry the keyboard.
    await coordinator.userFocus(agents.id, { keyboard: true });
    expect(request).toHaveBeenLastCalledWith(agents.id, expect.any(Number));
    await coordinator.userActivateWindow(settings.id);
    expect(request).toHaveBeenLastCalledWith(settings.id, expect.any(Number));
    await coordinator.userOpen({ app: "agents" });
    expect(request).toHaveBeenLastCalledWith(agents.id, expect.any(Number));
    coordinator.reportRouteMatch(agents.route, history.current());

    // Back to an earlier window's entry.
    request.mockClear();
    history.back();
    const backEntry = history.current();
    coordinator.reportRouteMatch(route("/settings/general"), backEntry);
    await vi.waitFor(() => expect(request).toHaveBeenCalledWith(settings.id, expect.any(Number)));
  });

  it("Should keep a navigation's first keyboard mark when the router reports it again [#1655]", async () => {
    const tasks = windowFixture("tasks", "/tasks");
    const settings = windowFixture("settings", "/settings/general");
    const { coordinator, store, history } = createCoordinator([tasks, settings]);
    coordinator.completeHydration();
    await coordinator.userFocus(tasks.id);
    coordinator.reportRouteMatch(tasks.route, history.current());
    const request = vi.spyOn(coordinator.keyboardFocus, "request");
    store.deferLifecycle();

    // Back to Settings: the router reports the entry as loading starts and again when it mounts,
    // and the operator clicks a field between the two reports.
    history.back();
    const entry = history.current();
    coordinator.reportRouteMatch(settings.route, entry);
    coordinator.keyboardFocus.cancel();
    coordinator.reportRouteMatch(settings.route, entry);
    store.settleLifecycle(true);

    await vi.waitFor(() => expect(store.getState().focusedId).toBe(settings.id));
    await vi.waitFor(() => expect(request).toHaveBeenCalled());
    const [, since] = request.mock.lastCall!;
    expect(since).not.toBe(coordinator.keyboardFocus.begin());
  });

  it("Should hold user route intent until window commands become available", () => {
    const { coordinator, router, store, history } = createCoordinator();
    coordinator.completeHydration();
    store.setClientConnected(false);
    const settingsRoute = route("/settings/general");

    coordinator.userNavigate(settingsRoute);
    coordinator.reportRouteMatch(settingsRoute, history.current());

    expect(router.navigate).toHaveBeenCalledOnce();
    // The operator's own navigation still answers navigation blockers.
    expect(router.navigate).toHaveBeenCalledWith(settingsRoute, "operator");
    expect(store.spies.openOrFocus).not.toHaveBeenCalled();

    store.setClientConnected(true);
    coordinator.reportAuthoritativeState();

    expect(store.spies.openOrFocus).toHaveBeenCalledOnce();
    expect(store.spies.openOrFocus).toHaveBeenCalledWith({
      app: "settings",
      instanceKey: undefined,
      route: settingsRoute,
    });
  });

  it("Should reconcile a user navigation whose location is already current", () => {
    const settings = windowFixture("settings", "/settings/general");
    const { coordinator, router, store } = createCoordinator([settings]);
    coordinator.completeHydration();
    vi.mocked(router.replace).mockClear();
    const profiles = route("/settings/profiles");

    // No route report follows: the router treats a same-location navigation as a no-op.
    coordinator.userNavigate(profiles);

    expect(router.navigate).toHaveBeenCalledOnce();
    expect(router.navigate).toHaveBeenCalledWith(profiles, "operator");
    expect(store.spies.openOrFocus).toHaveBeenCalledOnce();
    expect(store.spies.openOrFocus).toHaveBeenCalledWith({
      app: "settings",
      instanceKey: undefined,
      route: profiles,
    });
    expect(store.getState().windows[settings.id]?.route).toEqual(profiles);
  });

  it("Should preserve a consumed window route when its opening command completes", async () => {
    // Invariant: an accepted open reflects the current window route, including consumed intents.
    // Owner: the URL/window-manager bridge; canonical suite: RoutingCoordinator.
    const settings = windowFixture("settings", "/settings/profiles");
    const { coordinator, router, store } = createCoordinator([settings]);
    coordinator.completeHydration();
    const consumed = route("/settings/profiles");

    const pending = coordinator.userOpen({
      app: "settings",
      route: route("/settings/profiles", { flow: "create", name: "dispatch-notes" }),
    });
    store.setAuthoritativeFocus(settings.id, consumed);
    await expect(pending).resolves.toBe(settings.id);

    expect(router.navigate).toHaveBeenCalledOnce();
    expect(router.navigate).toHaveBeenCalledWith(consumed);
  });

  it("Should let a newer user navigation supersede a pending open's history write", async () => {
    const settings = windowFixture("settings", "/settings/profiles");
    const { coordinator, router, store } = createCoordinator([settings]);
    coordinator.completeHydration();
    vi.mocked(router.replace).mockClear();
    const general = route("/settings/general");
    const profiles = route("/settings/profiles");
    store.deferLifecycle();

    const pending = coordinator.userOpen({ app: "settings", route: general });
    // The runtime projects the navigate before the daemon answers (route intent).
    store.setAuthoritativeFocus(settings.id, general);
    expect(router.navigate).not.toHaveBeenCalled();

    // The operator picks Profiles while the daemon still holds the open.
    coordinator.userNavigate(profiles);
    expect(router.navigate).toHaveBeenCalledOnce();
    expect(router.navigate).toHaveBeenCalledWith(profiles, "operator");

    store.settleLifecycle(true);
    await expect(pending).resolves.toBe(settings.id);

    expect(router.navigate).toHaveBeenCalledOnce();
    expect(store.spies.openOrFocus).toHaveBeenLastCalledWith({
      app: "settings",
      instanceKey: undefined,
      route: profiles,
    });
    expect(store.getState().windows[settings.id]?.route).toEqual(profiles);
  });

  it("Should let a route the coordinator did not write supersede a pending open's history write", async () => {
    const settings = windowFixture("settings", "/settings/layouts");
    const { coordinator, router, store } = createCoordinator([settings]);
    coordinator.completeHydration();
    vi.mocked(router.replace).mockClear();
    const general = route("/settings/general");
    const profiles = route("/settings/profiles");
    store.deferLifecycle();

    const pending = coordinator.userOpen({ app: "settings", route: general });
    store.setAuthoritativeFocus(settings.id, general);

    // An in-window link changed the location; its own navigation wrote history.
    coordinator.reportRouteMatch(profiles);

    store.settleLifecycle(true);
    await expect(pending).resolves.toBe(settings.id);

    expect(router.navigate).not.toHaveBeenCalled();
    expect(store.spies.openOrFocus).toHaveBeenLastCalledWith({
      app: "settings",
      instanceKey: undefined,
      route: profiles,
    });
    expect(store.getState().windows[settings.id]?.route).toEqual(profiles);
  });

  it("Should let a link route own the single focus transition and history write", async () => {
    const tasks = windowFixture("tasks", "/tasks");
    const settings = windowFixture("settings", "/settings/general");
    const { coordinator, router, store } = createCoordinator([tasks, settings]);
    coordinator.completeHydration();
    vi.mocked(router.replace).mockClear();

    await coordinator.userFocus(tasks.id, { viaLink: true });

    expect(store.spies.focusWindow).not.toHaveBeenCalled();
    expect(store.getState().focusedId).toBe(settings.id);

    coordinator.reportRouteMatch(route("/tasks/task-42"));

    expect(store.spies.openOrFocus).toHaveBeenCalledOnce();
    expect(store.getState().focusedId).toBe(tasks.id);
    expect(store.getState().windows[tasks.id]?.route).toEqual(route("/tasks/task-42"));
    expect(router.navigate).not.toHaveBeenCalled();
    expect(router.replace).not.toHaveBeenCalled();
  });

  it("Should activate a focused window when it is hidden in an inactive tab", async () => {
    const agents = { ...windowFixture("agents", "/agents"), tabActive: true, shown: true };
    const tasks = { ...windowFixture("tasks", "/tasks"), tabActive: false, shown: false };
    const { coordinator, router, store } = createCoordinator([agents, tasks]);
    coordinator.completeHydration();
    vi.mocked(router.replace).mockClear();

    await expect(coordinator.userFocus(tasks.id)).resolves.toBe(true);

    expect(store.spies.focusWindow).toHaveBeenCalledOnce();
    expect(store.spies.focusWindow).toHaveBeenCalledWith(tasks.id, { replaceable: true });
    expect(router.navigate).toHaveBeenCalledOnce();
    expect(router.navigate).toHaveBeenCalledWith(tasks.route);
  });

  it("Should replace once when authoritative focus moves to a different route", () => {
    const tasks = windowFixture("tasks", "/tasks");
    const settings = windowFixture("settings", "/settings/general");
    const { coordinator, router, store } = createCoordinator([tasks, settings]);
    coordinator.reportRouteMatch(settings.route);
    coordinator.completeHydration();
    const authoritativeRoute = route("/tasks/task-42", {
      panel: "activity",
      filters: { owner: "me", state: "open" },
    });

    store.setAuthoritativeFocus("app:pending");
    coordinator.reportAuthoritativeState();

    expect(router.replace).not.toHaveBeenCalled();

    store.setAuthoritativeFocus(tasks.id, authoritativeRoute);
    store.setClientConnected(false);
    coordinator.reportAuthoritativeState();

    expect(router.replace).not.toHaveBeenCalled();

    store.setClientConnected(true);
    coordinator.reportAuthoritativeState();
    coordinator.reportAuthoritativeState();

    expect(router.replace).toHaveBeenCalledOnce();
    expect(router.replace).toHaveBeenCalledWith(authoritativeRoute);
    expect(router.navigate).not.toHaveBeenCalled();

    store.setAuthoritativeFocus(
      tasks.id,
      route("/tasks/task-42", {
        filters: { state: "open", owner: "me" },
        panel: "activity",
      })
    );
    coordinator.reportAuthoritativeState();

    expect(router.replace).toHaveBeenCalledOnce();
  });

  it("Should keep a held decision route through hydration and authoritative focus", () => {
    const tasks = windowFixture("tasks", "/tasks");
    const { coordinator, router, store } = createCoordinator([tasks]);
    const decision = route("/agents/general/sessions/sess-1", { workspaceSwitch: "confirm" });

    coordinator.holdRoute(decision);
    coordinator.completeHydration();
    store.setAuthoritativeFocus(tasks.id, tasks.route);
    coordinator.reportAuthoritativeState();

    expect(router.replace).not.toHaveBeenCalled();
    expect(router.navigate).not.toHaveBeenCalled();
    expect(store.spies.openOrFocus).not.toHaveBeenCalled();

    coordinator.releaseRouteHold();
    coordinator.reportAuthoritativeState();

    expect(router.replace).toHaveBeenCalledOnce();
    expect(router.replace).toHaveBeenCalledWith(tasks.route);
  });

  it.each([
    {
      action: "close" as const,
      invoke: (coordinator: RoutingCoordinator, id: string) => coordinator.userClose(id),
    },
  ])("Should navigate after the authoritative $action confirmation", async ({ invoke }) => {
    const tasks = windowFixture("tasks", "/tasks");
    const settings = windowFixture("settings", "/settings/general");
    const { coordinator, router, store } = createCoordinator([tasks, settings]);
    coordinator.completeHydration();
    store.deferLifecycle();

    const pending = invoke(coordinator, settings.id);

    expect(router.navigate).not.toHaveBeenCalled();

    store.settleLifecycle(true);
    await pending;

    expect(router.navigate).toHaveBeenCalledOnce();
    expect(router.navigate).toHaveBeenCalledWith(tasks.route);
  });

  it("Should keep history unchanged when a close command is rejected", async () => {
    const settings = windowFixture("settings", "/settings/general");
    const { coordinator, router, store } = createCoordinator([settings]);
    coordinator.completeHydration();
    store.deferLifecycle();

    const pending = coordinator.userClose(settings.id);
    store.settleLifecycle(false);
    await pending;

    expect(router.navigate).not.toHaveBeenCalled();
    expect(store.getState().windows[settings.id]).toBeDefined();
  });

  it("Should keep history unchanged when an open command is rejected", async () => {
    const { coordinator, router, store } = createCoordinator();
    coordinator.completeHydration();
    store.deferLifecycle();

    const pending = coordinator.userOpen({ app: "settings", route: route("/settings/layouts") });

    expect(router.navigate).not.toHaveBeenCalled();
    store.settleLifecycle(false);
    await expect(pending).resolves.toBeNull();
    expect(router.navigate).not.toHaveBeenCalled();
  });

  it("Should defer focus history until the semantic command completes", async () => {
    const tasks = windowFixture("tasks", "/tasks");
    const settings = windowFixture("settings", "/settings/general");
    const { coordinator, router, store } = createCoordinator([tasks, settings]);
    coordinator.completeHydration();
    vi.mocked(router.replace).mockClear();
    store.deferLifecycle();

    const pending = coordinator.userFocus(tasks.id);

    expect(router.navigate).not.toHaveBeenCalled();
    store.settleLifecycle(true);
    await expect(pending).resolves.toBe(true);
    expect(router.navigate).toHaveBeenCalledOnce();
    expect(router.navigate).toHaveBeenCalledWith(tasks.route);
  });

  /** Activations whose focus completes only when the test settles it, in dispatch order. */
  function deferFocusCompletions(store: ReturnType<typeof createCoordinator>["store"]) {
    const settles: Array<(applied: boolean) => void> = [];
    store.spies.focusWindow.mockImplementation(() => ({
      accepted: true,
      completion: new Promise<boolean>(resolve => {
        settles.push(resolve);
      }),
    }));
    return settles;
  }

  it.each([
    // A pane activates on pointer-down and again on focus: the first is replaced, the daemon
    // answers the repeat unapplied because the window is already focused.
    { burst: "A, A", ids: ["w-tasks", "w-tasks"], answers: [false, false] },
    { burst: "A, A with the first refused", ids: ["w-tasks", "w-tasks"], answers: [false, true] },
    // B is dropped before it is sent; the second A is a repeat at the daemon.
    { burst: "A, B, A", ids: ["w-tasks", "w-settings", "w-tasks"], answers: [false, false, false] },
    // B was sent and replaced; the second A moves the daemon back.
    {
      burst: "A, B, A with B sent",
      ids: ["w-tasks", "w-settings", "w-tasks"],
      answers: [false, false, true],
    },
  ])(
    "Should write one entry for a burst ($burst) that ends on its window [R9-01]",
    async ({ ids, answers }) => {
      const tasks = windowFixture("tasks", "/tasks");
      const settings = windowFixture("settings", "/settings/general");
      const agents = windowFixture("agents", "/agents");
      const { coordinator, router, store } = createCoordinator([tasks, settings, agents]);
      coordinator.completeHydration();
      vi.mocked(router.replace).mockClear();
      const settles = deferFocusCompletions(store);

      const activations = ids.map(id => coordinator.userFocus(id));
      // The stream reports the daemon focusing Tasks while the burst is still in flight.
      store.setAuthoritativeFocus(tasks.id);
      coordinator.reportAuthoritativeState();
      answers.forEach((applied, index) => settles[index]!(applied));
      await Promise.all(activations);

      expect(router.navigate).toHaveBeenCalledOnce();
      expect(router.navigate).toHaveBeenCalledWith(tasks.route);
      expect(router.replace).not.toHaveBeenCalled();
      await expect(activations.at(-1)!).resolves.toBe(true);
    }
  );

  it("Should write nothing for a burst that leaves focus where it was [R9-01]", async () => {
    const tasks = windowFixture("tasks", "/tasks");
    const agents = windowFixture("agents", "/agents");
    const { coordinator, router, store } = createCoordinator([tasks, agents]);
    coordinator.completeHydration();
    vi.mocked(router.replace).mockClear();
    const settles = deferFocusCompletions(store);

    const activations = [coordinator.userFocus(tasks.id), coordinator.userFocus(tasks.id)];
    for (const settle of settles) settle(false);

    await expect(Promise.all(activations)).resolves.toEqual([false, false]);
    expect(store.getState().focusedId).toBe(agents.id);
    expect(router.navigate).not.toHaveBeenCalled();
    expect(router.replace).not.toHaveBeenCalled();
  });

  it("Should write nothing when an activation re-activates the focused, shown window", async () => {
    const agents = windowFixture("agents", "/agents");
    const tasks = { ...windowFixture("tasks", "/tasks"), tabActive: true, shown: true };
    const { coordinator, router, store } = createCoordinator([agents, tasks]);
    coordinator.completeHydration();
    vi.mocked(router.replace).mockClear();
    const settles = deferFocusCompletions(store);

    // A session row or dock item for the window the operator is on.
    const activation = coordinator.userActivateWindow(tasks.id);
    // The daemon answers unapplied: the window is already focused.
    settles[0]!(false);

    await expect(activation).resolves.toBe(true);
    expect(store.getState().focusedId).toBe(tasks.id);
    expect(router.navigate).not.toHaveBeenCalled();
    expect(router.replace).not.toHaveBeenCalled();
  });

  it("Should leave the entry of a workspace switch to the switch when an activation from the workspace it left settles", async () => {
    const tasks = windowFixture("tasks", "/tasks");
    const agents = windowFixture("agents", "/agents");
    const settings = windowFixture("settings", "/settings/general");
    const { coordinator, router, store } = createCoordinator([tasks, agents, settings]);
    coordinator.completeHydration();
    vi.mocked(router.replace).mockClear();
    const settles = deferFocusCompletions(store);

    const activation = coordinator.userFocus(tasks.id);
    // The workspace switches while the focus is in flight; the target workspace focuses Agents.
    coordinator.beginWorkspaceSwitch();
    store.setAuthoritativeFocus(agents.id);
    // The daemon applied the focus sent from the workspace that was left.
    settles[0]!(true);
    await activation;
    expect(router.navigate).not.toHaveBeenCalled();
    coordinator.completeHydration();

    expect(router.navigate).toHaveBeenCalledOnce();
    expect(router.navigate).toHaveBeenCalledWith(agents.route);
    expect(router.replace).not.toHaveBeenCalled();
  });

  it("Should push where another source moved this client during an activation, keeping the entry before it [R9-02]", async () => {
    const tasks = windowFixture("tasks", "/tasks");
    const settings = windowFixture("settings", "/settings/general");
    const agents = windowFixture("agents", "/agents");
    const { coordinator, router, store } = createCoordinator([tasks, settings, agents]);
    coordinator.completeHydration();
    vi.mocked(router.replace).mockClear();
    const settles = deferFocusCompletions(store);

    const activation = coordinator.userFocus(tasks.id);
    // A held view adopted mid-flight: another source focused Settings, and this focus then fails.
    store.setAuthoritativeFocus(settings.id);
    coordinator.reportAuthoritativeState();
    expect(router.replace).not.toHaveBeenCalled();
    settles[0]!(false);

    await expect(activation).resolves.toBe(false);
    expect(router.navigate).toHaveBeenCalledOnce();
    expect(router.navigate).toHaveBeenCalledWith(settings.route);
    expect(router.replace).not.toHaveBeenCalled();
  });

  it("Should retarget the current window in place with one history write", async () => {
    const current = windowFixture("session", "/agents/coder/sessions/sess-a", "sess-a");
    const { coordinator, router, store } = createCoordinator([current]);
    coordinator.completeHydration();
    vi.mocked(router.replace).mockClear();

    const target = route("/agents/coder/sessions/sess-b");
    await expect(
      coordinator.userRetarget(current.id, {
        app: "session",
        instanceKey: "sess-b",
        route: target,
      })
    ).resolves.toBe(true);

    expect(store.spies.retargetWindow).toHaveBeenCalledWith(current.id, "sess-b", target);
    expect(store.spies.openOrFocus).not.toHaveBeenCalled();
    expect(router.navigate).toHaveBeenCalledOnce();
    expect(router.navigate).toHaveBeenCalledWith(target);
    expect(store.getState().windows[current.id]?.instanceKey).toBe("sess-b");
  });

  it("Should let the latest overlapping retarget own browser history", async () => {
    const current = windowFixture("terminal", "/terminals/term-a", "term-a");
    const { coordinator, router, store } = createCoordinator([current]);
    coordinator.completeHydration();
    vi.mocked(router.replace).mockClear();
    let settleFirst: ((accepted: boolean) => void) | undefined;
    let settleSecond: ((accepted: boolean) => void) | undefined;
    const firstCompletion = new Promise<boolean>(resolve => {
      settleFirst = resolve;
    });
    const secondCompletion = new Promise<boolean>(resolve => {
      settleSecond = resolve;
    });
    store.retargetWindow
      .mockReturnValueOnce({ accepted: true, completion: firstCompletion })
      .mockReturnValueOnce({ accepted: true, completion: secondCompletion });
    const firstRoute = route("/terminals/term-b");
    const latestRoute = route("/terminals/term-c");

    const first = coordinator.userRetarget(current.id, {
      app: "terminal",
      instanceKey: "term-b",
      route: firstRoute,
    });
    const latest = coordinator.userRetarget(current.id, {
      app: "terminal",
      instanceKey: "term-c",
      route: latestRoute,
    });
    settleSecond?.(true);
    await expect(latest).resolves.toBe(true);
    settleFirst?.(true);
    await expect(first).resolves.toBe(true);

    expect(router.navigate).toHaveBeenCalledOnce();
    expect(router.navigate).toHaveBeenCalledWith(latestRoute);
  });

  it("Should focus the target session's existing window instead of duplicating it", async () => {
    const current = windowFixture("session", "/agents/coder/sessions/sess-a", "sess-a");
    const other = windowFixture("session", "/agents/coder/sessions/sess-b", "sess-b");
    const { coordinator, router, store } = createCoordinator([current, other]);
    coordinator.completeHydration();
    vi.mocked(router.replace).mockClear();

    await expect(
      coordinator.userRetarget(current.id, {
        app: "session",
        instanceKey: "sess-b",
        route: route("/agents/coder/sessions/sess-b"),
      })
    ).resolves.toBe(true);

    expect(store.spies.retargetWindow).not.toHaveBeenCalled();
    expect(store.spies.focusWindow).toHaveBeenCalledWith(other.id, { replaceable: true });
    expect(router.navigate).toHaveBeenCalledOnce();
    expect(router.navigate).toHaveBeenCalledWith(other.route);
    expect(store.getState().windows[current.id]?.instanceKey).toBe("sess-a");
  });

  it("Should reconcile an existing target window to the requested route", async () => {
    const current = windowFixture("session", "/agents/coder/sessions/sess-a", "sess-a");
    const other = windowFixture("session", "/agents/coder/sessions/sess-b/child", "sess-b");
    const { coordinator, router, store } = createCoordinator([current, other]);
    coordinator.completeHydration();
    vi.mocked(router.replace).mockClear();
    const target = route("/agents/coder/sessions/sess-b");

    await expect(
      coordinator.userRetarget(current.id, {
        app: "session",
        instanceKey: "sess-b",
        route: target,
      })
    ).resolves.toBe(true);

    expect(store.getState().focusedId).toBe(other.id);
    expect(store.getState().windows[other.id]?.route).toEqual(target);
    expect(store.getState().windows[current.id]?.instanceKey).toBe("sess-a");
    expect(router.navigate).toHaveBeenCalledOnce();
    expect(router.navigate).toHaveBeenCalledWith(target);
  });

  it("Should retire a background session without changing the focused route", async () => {
    const focused = windowFixture("session", "/agents/coder/sessions/kept", "kept");
    const deleted = windowFixture("session", "/agents/coder/sessions/deleted", "deleted");
    const { coordinator, router, store } = createCoordinator([deleted, focused]);
    coordinator.completeHydration();
    await coordinator.userRetireSession(deleted.id);
    expect(store.getState().windows[deleted.id]?.instanceKey).toBe("");
    expect(store.getState().focusedId).toBe(focused.id);
    expect(router.navigate).not.toHaveBeenCalled();
  });

  it("Should keep the session a window moved on to when retiring the previous one", async () => {
    const current = windowFixture("session", "/agents/coder/sessions/next", "next");
    const { coordinator, store } = createCoordinator([current]);
    coordinator.completeHydration();

    await expect(coordinator.userRetireSession(current.id, "previous")).resolves.toBe(false);

    expect(store.spies.retargetWindow).not.toHaveBeenCalled();
    expect(store.getState().windows[current.id]?.instanceKey).toBe("next");
  });

  it("Should not push the empty route over a retarget that started while retiring", async () => {
    const current = windowFixture("session", "/agents/coder/sessions/sess-a", "sess-a");
    const { coordinator, router, store } = createCoordinator([current]);
    coordinator.completeHydration();
    let finishRetire = (_done: boolean) => {};
    const pending = new Promise<boolean>(resolve => (finishRetire = resolve));
    store.spies.retargetWindow.mockImplementationOnce(() => ({
      accepted: true,
      completion: pending,
    }));
    const next = route("/agents/coder/sessions/sess-b");

    const retiring = coordinator.userRetireSession(current.id, "sess-a");
    await coordinator.userRetarget(current.id, {
      app: "session",
      instanceKey: "sess-b",
      route: next,
    });
    vi.mocked(router.navigate).mockClear();
    finishRetire(true);

    await expect(retiring).resolves.toBe(true);
    expect(router.navigate).not.toHaveBeenCalled();
  });

  it("Should keep the history entry of a retarget pending while a background window retires", async () => {
    const focused = windowFixture("session", "/agents/coder/sessions/sess-a", "sess-a");
    const background = windowFixture("session", "/agents/coder/sessions/gone", "gone");
    const { coordinator, router, store } = createCoordinator([background, focused]);
    coordinator.completeHydration();
    let finishRetarget = (_done: boolean) => {};
    const pending = new Promise<boolean>(resolve => (finishRetarget = resolve));
    store.spies.retargetWindow.mockImplementationOnce(() => ({
      accepted: true,
      completion: pending,
    }));
    const next = route("/agents/coder/sessions/sess-b");

    const retargeting = coordinator.userRetarget(focused.id, {
      app: "session",
      instanceKey: "sess-b",
      route: next,
    });
    await coordinator.userRetireSession(background.id, "gone");
    finishRetarget(true);
    await retargeting;

    expect(router.navigate).toHaveBeenCalledWith(next);
  });

  it("Should retire a session tab onto the empty route without closing the window", async () => {
    const current = windowFixture("session", "/agents/coder/sessions/sess-a", "sess-a");
    const { coordinator, router, store } = createCoordinator([current]);
    coordinator.completeHydration();
    vi.mocked(router.replace).mockClear();
    const empty = route("/sessions");

    await expect(coordinator.userRetireSession(current.id)).resolves.toBe(true);

    expect(store.spies.retargetWindow).toHaveBeenCalledWith(current.id, "", empty);
    expect(store.spies.closeWindow).not.toHaveBeenCalled();
    expect(store.spies.openOrFocus).not.toHaveBeenCalled();
    expect(router.navigate).toHaveBeenCalledOnce();
    expect(router.navigate).toHaveBeenCalledWith(empty);
    expect(store.getState().windows[current.id]?.instanceKey).toBe("");
  });

  describe("new session landing (#825)", () => {
    const created = {
      app: "session" as const,
      instanceKey: "sess-new",
      route: route("/agents/coder/sessions/sess-new"),
    };
    const sessionWindow = (key: string | null, overrides: Partial<OsWindow> = {}): OsWindow => ({
      ...windowFixture("session", key ? `/agents/coder/sessions/${key}` : "/sessions", key),
      tabActive: true,
      shown: true,
      ...overrides,
    });

    it("Should select the new session in the focused Session window and keep it focused", async () => {
      const tasks = windowFixture("tasks", "/tasks");
      const focused = sessionWindow("sess-a");
      const { coordinator, router, store } = createCoordinator([tasks, focused]);
      coordinator.completeHydration();
      vi.mocked(router.replace).mockClear();

      await expect(coordinator.userOpenCreatedSession(created)).resolves.toBe(focused.id);

      expect(store.spies.retargetWindow).toHaveBeenCalledWith(
        focused.id,
        created.instanceKey,
        created.route
      );
      expect(store.spies.openOrFocus).not.toHaveBeenCalled();
      expect(store.spies.focusWindow).not.toHaveBeenCalled();
      expect(store.getState().focusedId).toBe(focused.id);
      expect(router.navigate).toHaveBeenCalledOnce();
      expect(router.navigate).toHaveBeenCalledWith(created.route);
    });

    it.each([
      ["visible", true],
      ["background", false],
    ] as const)(
      "Should bring a %s unfocused Session window forward with the new session",
      async (_label, shown) => {
        const session = sessionWindow("sess-a", { shown, tabActive: shown });
        const tasks = windowFixture("tasks", "/tasks");
        const { coordinator, router, store } = createCoordinator([session, tasks]);
        coordinator.completeHydration();
        vi.mocked(router.replace).mockClear();

        await expect(coordinator.userOpenCreatedSession(created)).resolves.toBe(session.id);

        expect(store.spies.focusWindow).toHaveBeenCalledWith(session.id);
        expect(store.spies.openOrFocus).not.toHaveBeenCalled();
        const landed = store.getState().windows[session.id];
        expect(landed?.instanceKey).toBe(created.instanceKey);
        expect(store.getState().focusedId).toBe(session.id);
        expect(router.navigate).toHaveBeenCalledOnce();
        expect(router.navigate).toHaveBeenCalledWith(created.route);
      }
    );

    it("Should prefer an empty Session window, then the most recently focused one", async () => {
      const older = sessionWindow("sess-a", { id: "w-session-older" });
      const empty = sessionWindow(null, { id: "w-session-empty" });
      const recent = sessionWindow("sess-b", { id: "w-session-recent" });
      const tasks = windowFixture("tasks", "/tasks");
      const withEmpty = createCoordinator([empty, older, recent, tasks]);
      withEmpty.coordinator.completeHydration();

      await expect(withEmpty.coordinator.userOpenCreatedSession(created)).resolves.toBe(empty.id);

      const withoutEmpty = createCoordinator([recent, older, tasks]);
      withoutEmpty.coordinator.completeHydration();

      await expect(withoutEmpty.coordinator.userOpenCreatedSession(created)).resolves.toBe(
        older.id
      );
      expect(withoutEmpty.store.getState().windows[recent.id]?.instanceKey).toBe("sess-b");
    });

    it.each([
      ["no Session window exists", [windowFixture("tasks", "/tasks")], {}],
      [
        "the only Session window is on another desktop",
        [sessionWindow("sess-a", { desktopId: "desktop:other" }), windowFixture("tasks", "/tasks")],
        {},
      ],
      ["the only Session window is pinned", [sessionWindow("sess-a", { pinned: true })], {}],
      [
        "the launcher asked for a split",
        [sessionWindow("sess-a")],
        { placement: "split" as const },
      ],
      [
        "the launcher asked for a new instance",
        [sessionWindow("sess-a")],
        { forceNewInstance: true },
      ],
    ])("Should open a new Session window when %s", async (_label, windows, options) => {
      const { coordinator, router, store } = createCoordinator(windows);
      coordinator.completeHydration();
      vi.mocked(router.replace).mockClear();
      const target = { ...created, ...options };

      await coordinator.userOpenCreatedSession(target);

      expect(store.spies.retargetWindow).not.toHaveBeenCalled();
      expect(store.spies.openOrFocus).toHaveBeenCalledWith(target);
      expect(router.navigate).toHaveBeenCalledOnce();
    });

    type Gesture =
      | "activate another window"
      | "focus the active window"
      | "close the focused window";
    const gestures: Record<
      Gesture,
      { run: (coordinator: RoutingCoordinator) => Promise<boolean>; focused: string }
    > = {
      "activate another window": {
        run: coordinator => coordinator.userActivateWindow("w-settings"),
        focused: "w-settings",
      },
      "focus the active window": {
        run: coordinator => coordinator.userFocus("w-tasks"),
        focused: "w-tasks",
      },
      "close the focused window": {
        run: coordinator => coordinator.userClose("w-tasks"),
        focused: "w-settings",
      },
    };

    it.each(
      (["direct", "route"] as const).flatMap(path =>
        (Object.keys(gestures) as Gesture[]).map(gesture => [path, gesture] as const)
      )
    )(
      "Should not pull focus back when a %s landing is pending and the operator chooses to %s",
      async (path, gesture) => {
        const session = sessionWindow("sess-a");
        const settings = windowFixture("settings", "/settings/general");
        const tasks = { ...windowFixture("tasks", "/tasks"), tabActive: true, shown: true };
        const { coordinator, router, store } = createCoordinator([session, settings, tasks]);
        coordinator.completeHydration();
        let settleRetarget: ((applied: boolean) => void) | undefined;
        store.retargetWindow.mockReturnValueOnce({
          accepted: true,
          completion: new Promise<boolean>(resolve => {
            settleRetarget = resolve;
          }),
        });

        let landing: Promise<string | null> | undefined;
        if (path === "direct") {
          landing = coordinator.userOpenCreatedSession(created);
        } else {
          coordinator.noteCreatedSession(created.instanceKey);
          coordinator.reportRouteMatch(created.route);
        }
        expect(store.spies.retargetWindow).toHaveBeenCalledWith(
          session.id,
          created.instanceKey,
          created.route
        );
        // The operator makes a newer choice while the retarget waits on the daemon.
        await gestures[gesture].run(coordinator);
        vi.mocked(router.navigate).mockClear();
        settleRetarget?.(true);
        await landing;
        await Promise.resolve();

        expect(store.spies.focusWindow).not.toHaveBeenCalledWith(session.id);
        expect(store.getState().focusedId).toBe(gestures[gesture].focused);
        expect(router.navigate).not.toHaveBeenCalled();
      }
    );

    it("Should choose again when another client closes the host before the landing", async () => {
      const session = sessionWindow("sess-a");
      const tasks = windowFixture("tasks", "/tasks");
      const { coordinator, router, store } = createCoordinator([session, tasks]);
      coordinator.completeHydration();
      vi.mocked(router.replace).mockClear();
      store.retargetWindow.mockImplementationOnce(() => ({
        accepted: true,
        // The daemon rejects the retarget: a peer closed the window first.
        completion: store.closeWindow(session.id).then(() => false),
      }));

      const landed = await coordinator.userOpenCreatedSession(created);

      expect(store.spies.openOrFocus).toHaveBeenCalledExactlyOnceWith(created);
      expect(landed).toBe(testWindowId("session", created.instanceKey));
      expect(store.getState().windows[landed ?? ""]?.instanceKey).toBe(created.instanceKey);
      expect(router.navigate).toHaveBeenCalledExactlyOnceWith(created.route);
    });

    it("Should open the session the ordinary way when a surviving host refuses the landing", async () => {
      const session = sessionWindow("sess-a");
      const tasks = windowFixture("tasks", "/tasks");
      const { coordinator, router, store } = createCoordinator([session, tasks]);
      coordinator.completeHydration();
      vi.mocked(router.replace).mockClear();
      store.retargetWindow.mockImplementationOnce(() => ({
        accepted: true,
        completion: Promise.resolve(false),
      }));

      const landed = await coordinator.userOpenCreatedSession(created);

      expect(store.getState().windows[session.id]).toBeDefined();
      expect(store.spies.openOrFocus).toHaveBeenCalledExactlyOnceWith(created);
      expect(landed).toBe(testWindowId("session", created.instanceKey));
      expect(router.navigate).toHaveBeenCalledExactlyOnceWith(created.route);
    });

    it("Should focus the window that already owns the new session", async () => {
      const owner = sessionWindow("sess-new");
      const focused = sessionWindow("sess-a");
      const { coordinator, store } = createCoordinator([owner, focused]);
      coordinator.completeHydration();

      await expect(coordinator.userOpenCreatedSession(created)).resolves.toBe(owner.id);

      expect(store.spies.retargetWindow).not.toHaveBeenCalled();
      expect(store.getState().windows[focused.id]?.instanceKey).toBe("sess-a");
      expect(store.getState().focusedId).toBe(owner.id);
    });

    it("Should land a dialog-created session's route in the Session window without writing history", async () => {
      const session = sessionWindow("sess-a");
      const tasks = windowFixture("tasks", "/tasks");
      const { coordinator, router, store } = createCoordinator([session, tasks]);
      coordinator.completeHydration();
      vi.mocked(router.replace).mockClear();

      coordinator.noteCreatedSession(created.instanceKey);
      coordinator.reportRouteMatch(created.route);
      await vi.waitFor(() => expect(store.getState().focusedId).toBe(session.id));

      expect(store.spies.retargetWindow).toHaveBeenCalledWith(
        session.id,
        created.instanceKey,
        created.route
      );
      expect(store.spies.openOrFocus).not.toHaveBeenCalled();
      expect(router.navigate).not.toHaveBeenCalled();
    });

    it("Should keep the created-session intent through a workspace switch", async () => {
      const { coordinator, store } = createCoordinator([windowFixture("tasks", "/tasks")]);
      coordinator.completeHydration();

      coordinator.noteCreatedSession(created.instanceKey);
      coordinator.beginWorkspaceSwitch();
      coordinator.reportRouteMatch(created.route);
      store.resetWorkspace();
      const target = sessionWindow(null, { id: "w-session-target" });
      store.getState().windows[target.id] = target;
      coordinator.completeHydration();
      await vi.waitFor(() => expect(store.getState().focusedId).toBe(target.id));

      expect(store.getState().windows[target.id]?.instanceKey).toBe(created.instanceKey);
      expect(store.spies.openOrFocus).not.toHaveBeenCalled();
    });

    it("Should keep the created-session intent from an older pending reconciliation", async () => {
      const session = sessionWindow("sess-a");
      const { coordinator, store } = createCoordinator([session]);
      coordinator.completeHydration();
      store.setHydration("pending");
      coordinator.reportRouteMatch(route("/tasks"));

      coordinator.noteCreatedSession(created.instanceKey);
      // The older route reconciles first, once commands become available.
      store.setHydration("live");
      coordinator.reportAuthoritativeState();
      await vi.waitFor(() => expect(store.getState().focusedId).toBe("w-tasks"));
      coordinator.reportRouteMatch(created.route);
      await vi.waitFor(() => expect(store.getState().focusedId).toBe(session.id));

      expect(store.getState().windows[session.id]?.instanceKey).toBe(created.instanceKey);
      expect(store.spies.openOrFocus).not.toHaveBeenCalledWith(
        expect.objectContaining({ instanceKey: created.instanceKey })
      );
    });

    it("Should keep opening a deep link to an unopened session in its own window", () => {
      const session = sessionWindow("sess-a");
      const { coordinator, store } = createCoordinator([session]);
      coordinator.completeHydration();

      coordinator.noteCreatedSession("sess-other");
      coordinator.reportRouteMatch(created.route);
      coordinator.reportRouteMatch(route("/agents/coder/sessions/sess-other"));

      expect(store.spies.retargetWindow).not.toHaveBeenCalled();
      expect(store.spies.openOrFocus).toHaveBeenCalledTimes(2);
      expect(store.getState().windows[session.id]?.instanceKey).toBe("sess-a");
    });
  });

  // Invariant: the router's late report of the coordinator's own latest write is an echo, never
  // intent — authoritative focus wins and the URL follows it; locations the coordinator did not
  // write stay intent.
  describe("Router echoes of the coordinator's own writes", () => {
    const sessionA = () => windowFixture("session", "/agents/general/sessions/sess-a", "sess-a");
    const sessionB = () => windowFixture("session", "/agents/general/sessions/sess-b", "sess-b");

    /** The coordinator wrote B's URL for authoritative focus B; the router has not reported it. */
    function afterWriteOfB() {
      const a = sessionA();
      const b = sessionB();
      const harness = createCoordinator([b, a]);
      const { coordinator, store } = harness;
      coordinator.completeHydration();
      coordinator.reportRouteMatch(a.route, harness.history.current());
      store.setAuthoritativeFocus(b.id);
      coordinator.reportAuthoritativeState();
      expect(harness.router.replace).toHaveBeenLastCalledWith(b.route);
      // The key of the entry B's write created; the router's report of that entry names it.
      const writeOfB = harness.history.current();
      vi.mocked(harness.router.replace).mockClear();
      vi.mocked(harness.router.navigate).mockClear();
      store.spies.openOrFocus.mockClear();
      store.spies.focusWindow.mockClear();
      return { ...harness, a, b, writeOfB };
    }

    it.each([
      ["F(A) S R(B)", "FSR"],
      ["F(A) R(B) S", "FRS"],
      ["S F(A) R(B)", "SFR"],
      ["S R(B) F(A)", "SRF"],
      ["R(B) F(A) S", "RFS"],
      ["R(B) S F(A)", "RSF"],
    ])(
      "Should keep focus A and land the URL on A when the events arrive as %s",
      async (_label, order) => {
        const { coordinator, router, store, a, b, writeOfB } = afterWriteOfB();
        const steps: Record<string, () => void> = {
          F: () => store.setAuthoritativeFocus(a.id),
          S: () => coordinator.reportAuthoritativeState(),
          R: () => coordinator.reportRouteMatch(b.route, writeOfB),
        };

        for (const step of order) steps[step]?.();
        // The stream reports every focus frame, so one more report follows the last event.
        coordinator.reportAuthoritativeState();
        await Promise.resolve();

        expect(store.getState().focusedId).toBe(a.id);
        expect(store.spies.openOrFocus).not.toHaveBeenCalled();
        expect(store.spies.focusWindow).not.toHaveBeenCalled();
        expect(router.navigate).not.toHaveBeenCalled();
        expect(router.replace).toHaveBeenLastCalledWith(a.route);
      }
    );

    it("Should treat Back to an earlier pushed location as intent", async () => {
      const tasks = windowFixture("tasks", "/tasks");
      const settings = windowFixture("settings", "/settings/general");
      const marketplace = windowFixture("marketplace", "/marketplace/skills");
      const { coordinator, router, store, history } = createCoordinator([
        tasks,
        settings,
        marketplace,
      ]);
      coordinator.completeHydration();
      coordinator.reportRouteMatch(marketplace.route, history.current());
      await coordinator.userFocus(tasks.id);
      const tasksEntry = history.current();
      coordinator.reportRouteMatch(tasks.route, tasksEntry);
      await coordinator.userFocus(settings.id);
      coordinator.reportRouteMatch(settings.route, history.current());
      store.spies.openOrFocus.mockClear();

      // Back pops to the Tasks entry the coordinator pushed; the traversal is the operator's.
      expect(history.back()).toBe(tasksEntry);
      coordinator.reportRouteMatch(tasks.route, tasksEntry);
      await vi.waitFor(() => expect(store.getState().focusedId).toBe(tasks.id));

      expect(store.spies.openOrFocus).toHaveBeenCalledWith(
        expect.objectContaining({ app: "tasks", route: tasks.route })
      );
      expect(router.navigate).toHaveBeenCalledTimes(2);
    });

    it("Should treat an entry a previous page load wrote as intent", async () => {
      const { coordinator, router, store, a, b, writeOfB } = afterWriteOfB();
      // Before a reload, the coordinator wrote B's URL into another entry of the same history;
      // this page's ledger starts empty and never saw that entry's key.
      const previousLoad = afterWriteOfB();
      expect(previousLoad.writeOfB).not.toBeNull();
      expect(previousLoad.writeOfB).not.toBe(writeOfB);

      store.setAuthoritativeFocus(a.id);
      coordinator.reportRouteMatch(b.route, previousLoad.writeOfB);
      await vi.waitFor(() => expect(store.getState().focusedId).toBe(b.id));
      coordinator.reportAuthoritativeState();

      expect(store.getState().focusedId).toBe(b.id);
      expect(router.replace).not.toHaveBeenCalled();
      expect(router.navigate).not.toHaveBeenCalled();
    });

    it("Should treat a deep link arriving after a write as intent", async () => {
      const { coordinator, router, store, a } = afterWriteOfB();
      const deepLink = route("/tasks/task-42");

      store.setAuthoritativeFocus(a.id);
      coordinator.reportRouteMatch(deepLink);
      await vi.waitFor(() => expect(store.getState().focusedId).toBe("w-tasks"));
      coordinator.reportAuthoritativeState();

      expect(store.spies.openOrFocus).toHaveBeenCalledWith(
        expect.objectContaining({ app: "tasks", route: deepLink })
      );
      expect(router.replace).not.toHaveBeenCalled();
      expect(router.navigate).not.toHaveBeenCalled();
    });

    /**
     * History holds an earlier Tasks entry and the URL shows Settings; authoritative focus then
     * writes Tasks and Sessions with `replace`, and the router has reported neither.
     */
    async function afterUnreportedReplaces() {
      const tasks = windowFixture("tasks", "/tasks");
      const sessions = sessionA();
      const settings = windowFixture("settings", "/settings/general");
      const harness = createCoordinator([tasks, sessions, settings]);
      const { coordinator, router, store, history } = harness;
      coordinator.completeHydration();
      coordinator.reportRouteMatch(settings.route, history.current());
      await coordinator.userFocus(tasks.id);
      const earlierTasks = history.current();
      coordinator.reportRouteMatch(tasks.route, earlierTasks);
      await coordinator.userFocus(settings.id);
      coordinator.reportRouteMatch(settings.route, history.current());
      store.setAuthoritativeFocus(tasks.id);
      coordinator.reportAuthoritativeState();
      store.setAuthoritativeFocus(sessions.id);
      coordinator.reportAuthoritativeState();
      expect(
        vi
          .mocked(router.replace)
          .mock.calls.slice(-2)
          .map(([written]) => written)
      ).toEqual([tasks.route, sessions.route]);
      vi.mocked(router.navigate).mockClear();
      vi.mocked(router.replace).mockClear();
      store.spies.openOrFocus.mockClear();
      return { ...harness, tasks, sessions, earlierTasks };
    }

    it("Should treat Back to an earlier entry with a pending write's URL as intent [AS-F-17]", async () => {
      const { coordinator, router, store, history, tasks, earlierTasks } =
        await afterUnreportedReplaces();

      // Back lands on the earlier Tasks entry before the router reports either replace.
      expect(history.back()).toBe(earlierTasks);
      coordinator.reportRouteMatch(tasks.route, earlierTasks);
      await vi.waitFor(() => expect(store.getState().focusedId).toBe(tasks.id));
      coordinator.reportAuthoritativeState();

      expect(store.spies.openOrFocus).toHaveBeenCalledWith(
        expect.objectContaining({ app: "tasks", route: tasks.route })
      );
      expect(router.navigate).not.toHaveBeenCalled();
      expect(router.replace).not.toHaveBeenCalled();
    });

    it("Should treat a link to a replaced pending write's URL as intent [AS-F-17]", async () => {
      const { coordinator, router, store, history, tasks } = await afterUnreportedReplaces();

      // A link creates its own entry, which the coordinator did not write.
      coordinator.reportRouteMatch(tasks.route, history.link());
      await vi.waitFor(() => expect(store.getState().focusedId).toBe(tasks.id));
      coordinator.reportAuthoritativeState();

      expect(store.spies.openOrFocus).toHaveBeenCalledWith(
        expect.objectContaining({ app: "tasks", route: tasks.route })
      );
      expect(router.navigate).not.toHaveBeenCalled();
      expect(router.replace).not.toHaveBeenCalled();
    });

    /** The coordinator's own activation of a session window its loader redirects to bind. */
    async function afterOwnActivationOfUnbound() {
      // Shown on its tab, as a window the dock can go to again.
      const unbound = {
        ...windowFixture("session", "/agents/general/sessions/sess-x", "sess-x"),
        shown: true,
      };
      const tasks = windowFixture("tasks", "/tasks");
      const harness = createCoordinator([unbound, tasks]);
      const { coordinator, router, store, history } = harness;
      coordinator.completeHydration();
      coordinator.reportRouteMatch(tasks.route, history.current());
      // All projects: the session window's stored route has no workspace binding yet.
      await coordinator.userActivateWindow(unbound.id);
      const written = history.current();
      const bound = route(unbound.route.pathname, { workspace: "workspace:a" });
      const redirected = history.redirect();
      store.spies.openOrFocus.mockClear();
      store.spies.focusWindow.mockClear();
      vi.mocked(router.navigate).mockClear();
      vi.mocked(router.replace).mockClear();
      return { ...harness, unbound, tasks, written, bound, redirected };
    }

    it("Should adopt the canonical route after a repeated activation of the same window settles [AS-F-19]", async () => {
      const { coordinator, router, store, unbound, written, bound, redirected } =
        await afterOwnActivationOfUnbound();

      // Dock "Go to tab" on the window already shown: both reports arrive while it settles.
      const repeat = coordinator.userActivateWindow(unbound.id);
      coordinator.reportRouteMatch(unbound.route, written);
      coordinator.reportRouteMatch(bound, redirected);
      expect(store.spies.navigateWindow).not.toHaveBeenCalled();
      await repeat;
      coordinator.reportAuthoritativeState();

      expect(store.spies.navigateWindow).toHaveBeenCalledExactlyOnceWith(unbound.id, bound);
      expect(store.getState().windows[unbound.id]?.route).toEqual(bound);
      expect(store.getState().focusedId).toBe(unbound.id);
      expect(store.spies.openOrFocus).not.toHaveBeenCalled();
      expect(router.navigate).not.toHaveBeenCalled();
      expect(router.replace).not.toHaveBeenCalled();
    });

    it("Should adopt the canonical route after a refused competing activation [AS-F-19]", async () => {
      const { coordinator, router, store, unbound, tasks, written, bound, redirected } =
        await afterOwnActivationOfUnbound();
      store.spies.focusWindow.mockReturnValueOnce({
        accepted: false,
        completion: Promise.resolve(false),
      });

      const competing = coordinator.userActivateWindow(tasks.id);
      coordinator.reportRouteMatch(unbound.route, written);
      coordinator.reportRouteMatch(bound, redirected);
      await competing;
      coordinator.reportAuthoritativeState();

      expect(store.getState().focusedId).toBe(unbound.id);
      expect(store.spies.navigateWindow).toHaveBeenCalledExactlyOnceWith(unbound.id, bound);
      expect(store.getState().windows[unbound.id]?.route).toEqual(bound);
      expect(router.navigate).not.toHaveBeenCalled();
      expect(router.replace).not.toHaveBeenCalled();
    });

    it("Should drop the canonical route when the activation that settles moved focus [AS-F-19]", async () => {
      const { coordinator, router, store, unbound, tasks, written, bound, redirected } =
        await afterOwnActivationOfUnbound();

      const competing = coordinator.userActivateWindow(tasks.id);
      coordinator.reportRouteMatch(unbound.route, written);
      coordinator.reportRouteMatch(bound, redirected);
      await competing;
      coordinator.reportAuthoritativeState();

      expect(store.getState().focusedId).toBe(tasks.id);
      expect(store.spies.navigateWindow).not.toHaveBeenCalled();
      expect(store.getState().windows[unbound.id]?.route).toEqual(unbound.route);
      expect(router.navigate).toHaveBeenCalledExactlyOnceWith(tasks.route);
      expect(router.replace).not.toHaveBeenCalled();
    });

    it("Should adopt the canonical route of its own write for the window still focused [R10-05]", async () => {
      const { coordinator, router, store, unbound, written, bound, redirected } =
        await afterOwnActivationOfUnbound();

      coordinator.reportRouteMatch(unbound.route, written);
      coordinator.reportRouteMatch(bound, redirected);
      coordinator.reportAuthoritativeState();
      await Promise.resolve();

      expect(store.spies.navigateWindow).toHaveBeenCalledExactlyOnceWith(unbound.id, bound);
      expect(store.getState().windows[unbound.id]?.route).toEqual(bound);
      expect(store.getState().focusedId).toBe(unbound.id);
      expect(store.spies.openOrFocus).not.toHaveBeenCalled();
      expect(store.spies.focusWindow).not.toHaveBeenCalled();
      expect(router.navigate).not.toHaveBeenCalled();
      expect(router.replace).not.toHaveBeenCalled();
    });

    it("Should keep the written route when the window refuses the canonical one [R10-05]", async () => {
      const { coordinator, router, store, unbound, written, bound, redirected } =
        await afterOwnActivationOfUnbound();
      store.spies.navigateWindow.mockReturnValueOnce({
        accepted: false,
        completion: Promise.resolve(false),
      });

      coordinator.reportRouteMatch(unbound.route, written);
      coordinator.reportRouteMatch(bound, redirected);
      coordinator.reportAuthoritativeState();
      await Promise.resolve();

      expect(store.spies.navigateWindow).toHaveBeenCalledOnce();
      expect(store.getState().windows[unbound.id]?.route).toEqual(unbound.route);
      // Replacing the URL back to the unbound route would redirect again, for every refusal.
      expect(router.navigate).not.toHaveBeenCalled();
      expect(router.replace).not.toHaveBeenCalled();
    });

    it("Should read the same entry reported twice as one echo when focus moves between reports [E2E-040]", async () => {
      const { coordinator, router, store, a, b, writeOfB } = afterWriteOfB();

      // The previous route's sync reports B's entry as its load starts; the session route reports
      // it again once its loader settles. Focus moves to A in between.
      coordinator.reportRouteMatch(b.route, writeOfB);
      store.setAuthoritativeFocus(a.id);
      coordinator.reportRouteMatch(b.route, writeOfB);
      coordinator.reportAuthoritativeState();
      await Promise.resolve();

      expect(store.getState().focusedId).toBe(a.id);
      expect(store.spies.openOrFocus).not.toHaveBeenCalled();
      expect(store.spies.focusWindow).not.toHaveBeenCalled();
      expect(router.navigate).not.toHaveBeenCalled();
      expect(router.replace).toHaveBeenLastCalledWith(a.route);
    });

    it("Should read both reports of a redirected write as echoes once focus moved on [AS-F-17, R10-05]", async () => {
      const { coordinator, router, store, history, a, b, writeOfB } = afterWriteOfB();
      // B's loader canonicalizes its URL with a replace while B's entry is still loading.
      const canonicalB = route(b.route.pathname, { workspace: "workspace:b" });
      const redirected = history.redirect();

      // Focus moves on to A before either report: B's canonical form is dropped.
      store.setAuthoritativeFocus(a.id);
      coordinator.reportRouteMatch(b.route, writeOfB);
      coordinator.reportRouteMatch(canonicalB, redirected);
      await Promise.resolve();

      expect(store.getState().focusedId).toBe(a.id);
      expect(store.spies.openOrFocus).not.toHaveBeenCalled();
      expect(store.spies.focusWindow).not.toHaveBeenCalled();
      expect(store.spies.navigateWindow).not.toHaveBeenCalled();
      expect(router.navigate).not.toHaveBeenCalled();
      expect(router.replace).toHaveBeenLastCalledWith(a.route);
    });

    it("Should read a replace after the written entry finished loading as intent", async () => {
      const { coordinator, store, history, a, b, writeOfB } = afterWriteOfB();
      if (writeOfB === null) throw new Error("B's write must have created an entry");
      history.resolve(writeOfB);
      // Not the router finishing B's navigation: a later replace is someone else's.
      const replaced = history.redirect();

      store.setAuthoritativeFocus(a.id);
      coordinator.reportRouteMatch(b.route, replaced);
      await vi.waitFor(() => expect(store.getState().focusedId).toBe(b.id));

      expect(store.spies.openOrFocus).toHaveBeenCalledWith(
        expect.objectContaining({ route: b.route })
      );
    });

    it.each(registeredBlockers)(
      "Should keep B when A's redirect reports after B was chosen and a blocker %s [AS-F-17]",
      async (_label, blockerFn) => {
        const a = sessionA();
        const tasks = windowFixture("tasks", "/tasks");
        const { coordinator, router, store, currentKey, lastPushKey, disconnect } =
          createBridgedCoordinator([a, tasks], blockerFn);
        await router.load();
        coordinator.completeHydration();
        coordinator.reportRouteMatch(tasks.route, currentKey());

        // All projects: A's route has no workspace binding; its loader will add one.
        await coordinator.userActivateWindow(a.id);
        const writeOfA = lastPushKey();
        expect(router.history.location.pathname).toBe(a.route.pathname);
        store.deferLifecycle();
        const toB = coordinator.userActivateWindow(tasks.id);
        await router.load();
        const redirectOfA = currentKey();
        expect(redirectOfA).not.toBe(writeOfA);
        coordinator.reportRouteMatch(a.route, writeOfA);
        coordinator.reportRouteMatch(route(a.route.pathname, { workspace: "w" }), redirectOfA);
        store.settleLifecycle(true);
        await toB;
        coordinator.reportAuthoritativeState();
        await Promise.resolve();

        expect(store.getState().focusedId).toBe(tasks.id);
        expect(store.spies.openOrFocus).not.toHaveBeenCalled();
        expect(store.spies.navigateWindow).not.toHaveBeenCalled();
        expect(router.history.location.pathname).toBe("/tasks");
        disconnect();
      }
    );

    it.each(registeredBlockers)(
      "Should read one written entry reported twice as an echo when a blocker %s [E2E-040]",
      async (_label, blockerFn) => {
        const a = windowFixture("tasks", "/tasks");
        const b = windowFixture("settings", "/settings");
        const { coordinator, router, store, currentKey, disconnect } = createBridgedCoordinator(
          [b, a],
          blockerFn
        );
        await router.load();
        coordinator.completeHydration();
        coordinator.reportRouteMatch(a.route, currentKey());
        store.setAuthoritativeFocus(b.id);
        coordinator.reportAuthoritativeState();
        const writeOfB = currentKey();
        expect(router.history.location.pathname).toBe("/settings");

        coordinator.reportRouteMatch(b.route, writeOfB);
        store.setAuthoritativeFocus(a.id);
        coordinator.reportRouteMatch(b.route, writeOfB);
        coordinator.reportAuthoritativeState();
        await Promise.resolve();

        expect(store.getState().focusedId).toBe(a.id);
        expect(store.spies.openOrFocus).not.toHaveBeenCalled();
        expect(store.spies.focusWindow).not.toHaveBeenCalled();
        expect(router.history.location.pathname).toBe("/tasks");
        disconnect();
      }
    );

    it.each(registeredBlockers)(
      "Should carry a Tasks filter made while Tasks still loads to its window unless a blocker holds it, when it %s [AS-F-17]",
      async (label, blockerFn) => {
        const settings = windowFixture("settings", "/settings");
        const tasks = windowFixture("tasks", "/tasks");
        const { coordinator, router, store, currentKey, holdTasks, disconnect } =
          createBridgedCoordinator([settings, tasks], blockerFn);
        await router.load();
        coordinator.completeHydration();
        coordinator.reportRouteMatch(tasks.route, currentKey());
        await coordinator.userActivateWindow(settings.id);
        await router.load();
        coordinator.reportRouteMatch(settings.route, currentKey());
        // The coordinator writes Tasks; its preload stays outstanding.
        const releaseTasks = holdTasks();
        await coordinator.userActivateWindow(tasks.id);
        expect(router.history.location.pathname).toBe("/tasks");
        coordinator.reportRouteMatch(tasks.route, currentKey());
        store.deferLifecycle();
        const toSettings = coordinator.userActivateWindow(settings.id);

        // The Tasks filter, as `tasks-catalog-location.tsx` changes it: an app's replace.
        const filtered = route("/tasks", { status: "ready" });
        void router.navigate({ to: "/tasks", search: { status: "ready" }, replace: true });
        const letsThrough = label.startsWith("lets");
        if (letsThrough) {
          await vi.waitFor(() => expect(router.history.location.search).toContain("status=ready"));
          coordinator.reportRouteMatch(filtered, currentKey());
        } else {
          await Promise.resolve();
          expect(router.history.location.search).not.toContain("status=ready");
        }
        releaseTasks();
        store.settleLifecycle(true);
        await toSettings;
        coordinator.reportAuthoritativeState();

        if (letsThrough) {
          await vi.waitFor(() =>
            expect(store.getState().windows[tasks.id]?.route).toEqual(filtered)
          );
        } else {
          await Promise.resolve();
          expect(store.getState().windows[tasks.id]?.route).toEqual(tasks.route);
          expect(store.getState().focusedId).toBe(settings.id);
        }
        disconnect();
      }
    );

    // Invariant (B-1): a dirty editor's guard holds only a navigation that lands in the editor's
    // own window with another route, resolved where the coordinator lands it; the URL following
    // focus to another window, that window's own navigation and a return to the editor's route
    // pass. Owning layer: the shell's answer to the guard over the real router and history.
    it("Should hold only a navigation that would replace a dirty editor in its own window [B-1]", async () => {
      const settings = windowFixture("settings", "/settings");
      const editor = windowFixture("tasks", "/tasks");
      const held: string[] = [];
      const bridged = createBridgedCoordinator([settings, editor], ({ nextLocation }) => {
        const replaces = routeReplacesWindowContent(bridged.store.getState(), editor.id, {
          pathname: nextLocation.pathname,
          search: defaultParseSearch(nextLocation.search),
        });
        if (replaces) held.push(`${nextLocation.pathname}${nextLocation.search}`);
        return replaces;
      });
      const { coordinator, router, currentKey, disconnect } = bridged;
      await router.load();
      coordinator.completeHydration();
      coordinator.reportRouteMatch(editor.route, currentKey());

      // Another tab: the URL follows focus to Settings.
      await coordinator.userActivateWindow(settings.id);
      expect(router.history.location.pathname).toBe("/settings");
      // Settings' own navigation, a section.
      await router.navigate({ href: "/settings?section=general" });
      expect(router.history.location.search).toContain("section=general");
      // A location that would land in the editor's window with another route.
      void router.navigate({ href: "/tasks?status=ready" });
      await Promise.resolve();
      expect(router.history.location.pathname).toBe("/settings");
      // Back to the editor's own route.
      await coordinator.userActivateWindow(editor.id);
      expect(router.history.location.pathname).toBe("/tasks");

      expect(held).toEqual(["/tasks?status=ready"]);
      disconnect();
    });

    // Invariant (#1653): the guard asks exactly when the edited resource would go: a route naming
    // the page the window shows (a search it does not read, a legacy path resolving to the page)
    // keeps it, another page or the catalog replaces it, a route outside the shell layout takes
    // every window, and the shell's legacy redirects land elsewhere.
    it.each([
      ["keep", "/agents/coder/what-it-does", { ordered: "1" }, "/_app/agents/$name/$page", false],
      ["keep", "/agents/coder/what-it-does", { panel: "tools" }, "/_app/agents/$name/$page", false],
      [
        "keep",
        "/agents/coder/settings",
        { section: "instructions" },
        "/_app/agents/$name/settings",
        false,
      ],
      ["keep", "/agents/coder", { tab: "instructions" }, "/_app/agents/$name/", false],
      ["replace", "/agents/coder/memory", {}, "/_app/agents/$name/$page", true],
      ["replace", "/agents", {}, "/_app/agents/", true],
      ["replace", "/design-system", {}, "/design-system", true],
      ["keep", "/jobs", {}, "/_app/jobs", false],
    ] as const)(
      "Should %s a dirty agent page for %s [#1653]",
      (_verb, pathname, search, routeId, replaces) => {
        const editor = windowFixture("agents", "/agents/coder/what-it-does");
        const settings = windowFixture("settings", "/settings");
        const state = createStore([editor, settings]).getState();
        expect(routeReplacesWindowContent(state, editor.id, { pathname, search, routeId })).toBe(
          replaces
        );
      }
    );

    it("Should carry a same-path replace made during an owned load to its window when a competing activation moves focus [AS-F-17]", async () => {
      const tasks = windowFixture("tasks", "/tasks");
      const settings = windowFixture("settings", "/settings");
      const { coordinator, store, history } = createCoordinator([tasks, settings]);
      coordinator.completeHydration();
      coordinator.reportRouteMatch(settings.route, history.current());
      // The coordinator's own write of Tasks; its preload has not finished.
      await coordinator.userActivateWindow(tasks.id);
      coordinator.reportRouteMatch(tasks.route, history.current());
      store.spies.openOrFocus.mockClear();
      store.deferLifecycle();
      const toSettings = coordinator.userActivateWindow(settings.id);

      // The operator changes a Tasks filter before the preload settled: an app's replace on the
      // same path, unmarked, so it is intent and not the loader's continuation.
      const filtered = route("/tasks", { status: "open" });
      coordinator.reportRouteMatch(filtered, history.replace());
      store.settleLifecycle(true);
      await toSettings;
      coordinator.reportAuthoritativeState();

      await vi.waitFor(() => expect(store.getState().windows[tasks.id]?.route).toEqual(filtered));
      expect(store.spies.openOrFocus).toHaveBeenCalledWith(
        expect.objectContaining({ app: "tasks", route: filtered })
      );
    });

    it("Should still own the router's marked continuation of an owned load at the same path [AS-F-17]", async () => {
      const tasks = windowFixture("tasks", "/tasks");
      const settings = windowFixture("settings", "/settings");
      const { coordinator, store, history } = createCoordinator([tasks, settings]);
      coordinator.completeHydration();
      coordinator.reportRouteMatch(settings.route, history.current());
      await coordinator.userActivateWindow(tasks.id);
      coordinator.reportRouteMatch(tasks.route, history.current());
      store.spies.openOrFocus.mockClear();
      store.deferLifecycle();
      const toSettings = coordinator.userActivateWindow(settings.id);

      // Tasks' loader canonicalizes its search while it is still loading.
      coordinator.reportRouteMatch(route("/tasks", { mode: "list" }), history.redirect());
      store.settleLifecycle(true);
      await toSettings;
      coordinator.reportAuthoritativeState();
      await Promise.resolve();

      expect(store.getState().focusedId).toBe(settings.id);
      expect(store.getState().windows[tasks.id]?.route).toEqual(tasks.route);
      expect(store.spies.openOrFocus).not.toHaveBeenCalled();
    });

    it("Should end on B when A's redirect reports after B was activated [AS-F-17]", async () => {
      const a = sessionA();
      const b = sessionB();
      const tasks = windowFixture("tasks", "/tasks");
      const { coordinator, router, store, history } = createCoordinator([a, b, tasks]);
      coordinator.completeHydration();
      coordinator.reportRouteMatch(tasks.route, history.current());

      // All projects: A's loader redirects to add the workspace binding before B is activated.
      await coordinator.userActivateWindow(a.id);
      const writeOfA = history.current();
      const redirectOfA = history.redirect();
      await coordinator.userActivateWindow(b.id);
      const writeOfB = history.current();
      store.spies.openOrFocus.mockClear();
      store.spies.focusWindow.mockClear();
      vi.mocked(router.navigate).mockClear();
      vi.mocked(router.replace).mockClear();

      coordinator.reportRouteMatch(a.route, writeOfA);
      coordinator.reportRouteMatch(route(a.route.pathname, { workspace: "ws-a" }), redirectOfA);
      coordinator.reportRouteMatch(b.route, writeOfB);
      coordinator.reportAuthoritativeState();
      await Promise.resolve();

      expect(store.getState().focusedId).toBe(b.id);
      expect(store.spies.openOrFocus).not.toHaveBeenCalled();
      expect(store.spies.focusWindow).not.toHaveBeenCalled();
      expect(router.navigate).not.toHaveBeenCalled();
      expect(router.replace).not.toHaveBeenCalled();
    });

    it("Should treat Back to a written entry as intent after a link and a replace followed it", async () => {
      const tasks = windowFixture("tasks", "/tasks");
      const settings = windowFixture("settings", "/settings/general");
      const sessions = sessionA();
      const { coordinator, router, store, history } = createCoordinator([
        tasks,
        settings,
        sessions,
      ]);
      coordinator.completeHydration();
      coordinator.reportRouteMatch(sessions.route, history.current());
      // Push A (Tasks), an operator link pushes B (Settings), then authority replaces B with C.
      await coordinator.userFocus(tasks.id);
      const entryOfA = history.current();
      history.link();
      store.setAuthoritativeFocus(sessions.id);
      coordinator.reportAuthoritativeState();
      expect(router.replace).toHaveBeenLastCalledWith(sessions.route);
      store.spies.openOrFocus.mockClear();
      vi.mocked(router.navigate).mockClear();
      vi.mocked(router.replace).mockClear();

      // Back to A before any of those entries reports.
      expect(history.back()).toBe(entryOfA);
      coordinator.reportRouteMatch(tasks.route, entryOfA);
      await vi.waitFor(() => expect(store.getState().focusedId).toBe(tasks.id));
      coordinator.reportAuthoritativeState();

      expect(store.spies.openOrFocus).toHaveBeenCalledWith(
        expect.objectContaining({ app: "tasks", route: tasks.route })
      );
      expect(router.navigate).not.toHaveBeenCalled();
      expect(router.replace).not.toHaveBeenCalled();
    });
  });
});
