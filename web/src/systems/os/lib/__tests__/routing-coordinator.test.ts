// Suite: routing coordinator
// Invariant: URL reconciliation mutates only the daemon-facing controller, route intent survives
// hydration, command fences, and in-flight authority races, and user causes produce exactly one
// history write.
// Owning layer: the sole URL ↔ window-manager bridge.
import { describe, expect, it, vi } from "vitest";

import type { OsHydration, OsOpenTarget, OsWindow, OsWindowRoute } from "../os-types";
import { RoutingCoordinator, type OsRouterPort } from "../routing-coordinator";
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
    pinned: false,
    desktopId: "desktop:main",
    placement: "floating",
    rect: { x: 40, y: 40, w: 720, h: 480 },
    layer: 1,
    minimized: false,
    zoomed: false,
    groupId: null,
    nodeId: null,
    stackId: null,
    stackActive: false,
    parentAxis: null,
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
    stackActive: {},
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
  const minimizeWindow = vi.fn(async (id: string) => {
    if (lifecycleResult !== null && !(await lifecycleResult)) return false;
    if (!windows[id]) return false;
    windows[id] = { ...windows[id], minimized: true };
    if (focusedId === id) {
      const successor =
        focusOrder.find(candidate => candidate !== id && !windows[candidate]?.minimized) ?? null;
      if (successor === null) focusedId = null;
      else focus(successor);
    }
    return true;
  });
  const zoomWindow = vi.fn((id: string) =>
    commandOutcome(() => {
      if (windows[id]) focus(id);
    })
  );
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
  const restoreWindow = vi.fn((id: string) =>
    commandOutcome(() => {
      if (windows[id]) {
        windows[id] = { ...windows[id], minimized: false };
        focus(id);
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
          stackActive: {},
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
    restoreWindow,
    closeWindow,
    minimizeWindow,
    zoomWindow,
    navigateWindow,
    retargetWindow,
    popWindowRoute,
  };

  return {
    getState: () => state,
    openOrFocus,
    focusWindow,
    restoreWindow,
    closeWindow,
    minimizeWindow,
    zoomWindow,
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
      restoreWindow,
      closeWindow,
      minimizeWindow,
      zoomWindow,
      navigateWindow,
      retargetWindow,
      popWindowRoute,
    },
  };
}

function createCoordinator(initialWindows: readonly OsWindow[] = []) {
  const store = createStore(initialWindows);
  const router: OsRouterPort = {
    navigate: vi.fn(),
    replace: vi.fn(),
  };
  return {
    coordinator: new RoutingCoordinator(store, router),
    router,
    store,
  };
}

describe("RoutingCoordinator", () => {
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

  it("Should push exactly once after a user opens an app", async () => {
    const { coordinator, router } = createCoordinator();
    coordinator.completeHydration();

    const id = await coordinator.userOpen({ app: "settings", route: route("/settings/layouts") });

    expect(id).toBe("w-settings");
    expect(router.navigate).toHaveBeenCalledOnce();
    expect(router.navigate).toHaveBeenCalledWith(route("/settings/layouts"));
  });

  it("Should hold user route intent until window commands become available", () => {
    const { coordinator, router, store } = createCoordinator();
    coordinator.completeHydration();
    store.setClientConnected(false);
    const settingsRoute = route("/settings/general");

    coordinator.userNavigate(settingsRoute);
    coordinator.reportRouteMatch(settingsRoute);

    expect(router.navigate).toHaveBeenCalledOnce();
    expect(router.navigate).toHaveBeenCalledWith(settingsRoute);
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

  it("Should activate a focused window when it is an inactive stack member", async () => {
    const agents = {
      ...windowFixture("agents", "/agents"),
      stackId: "stack:main",
      stackActive: true,
    };
    const tasks = {
      ...windowFixture("tasks", "/tasks"),
      stackId: "stack:main",
      stackActive: false,
    };
    const { coordinator, router, store } = createCoordinator([agents, tasks]);
    coordinator.completeHydration();
    vi.mocked(router.replace).mockClear();

    await expect(coordinator.userFocus(tasks.id)).resolves.toBe(true);

    expect(store.spies.focusWindow).toHaveBeenCalledOnce();
    expect(store.spies.focusWindow).toHaveBeenCalledWith(tasks.id);
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

  it("Should zoom an inactive window with one command and one history write", async () => {
    const tasks = windowFixture("tasks", "/tasks");
    const settings = windowFixture("settings", "/settings/general");
    const { coordinator, router, store } = createCoordinator([tasks, settings]);
    coordinator.completeHydration();

    await coordinator.userZoom(tasks.id);

    expect(store.spies.zoomWindow).toHaveBeenCalledOnce();
    expect(store.spies.zoomWindow).toHaveBeenCalledWith(tasks.id);
    expect(store.spies.focusWindow).not.toHaveBeenCalled();
    expect(router.navigate).toHaveBeenCalledOnce();
    expect(router.navigate).toHaveBeenCalledWith(tasks.route);
  });

  it("Should not write duplicate history when zooming the focused window", async () => {
    const tasks = windowFixture("tasks", "/tasks");
    const { coordinator, router, store } = createCoordinator([tasks]);
    coordinator.completeHydration();

    await coordinator.userZoom(tasks.id);

    expect(store.spies.zoomWindow).toHaveBeenCalledOnce();
    expect(router.navigate).not.toHaveBeenCalled();
  });

  it.each([
    {
      action: "close" as const,
      invoke: (coordinator: RoutingCoordinator, id: string) => coordinator.userClose(id),
    },
    {
      action: "minimize" as const,
      invoke: (coordinator: RoutingCoordinator, id: string) => coordinator.userMinimize(id),
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

  it("Should keep history unchanged when a zoom command is rejected", async () => {
    const tasks = windowFixture("tasks", "/tasks");
    const settings = windowFixture("settings", "/settings/general");
    const { coordinator, router, store } = createCoordinator([tasks, settings]);
    coordinator.completeHydration();
    vi.mocked(router.replace).mockClear();
    store.deferLifecycle();

    const pending = coordinator.userZoom(tasks.id);

    expect(router.navigate).not.toHaveBeenCalled();
    store.settleLifecycle(false);
    await expect(pending).resolves.toBe(false);
    expect(router.navigate).not.toHaveBeenCalled();
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
    expect(store.spies.focusWindow).toHaveBeenCalledWith(other.id);
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
});
