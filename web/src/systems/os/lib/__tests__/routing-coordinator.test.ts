// Suite: routing coordinator
// Invariant: URL reconciliation mutates only the daemon-facing controller, initial route intent
// survives workspace binding exactly once, and user causes produce exactly one history write.
// Owning layer: the sole URL ↔ window-manager bridge.
import { describe, expect, it, vi } from "vitest";

import type { OsOpenTarget, OsWindow, OsWindowRoute } from "../os-types";
import { osWindowId } from "../os-types";
import { RoutingCoordinator, type OsRouterPort } from "../routing-coordinator";
import type { WindowManagerClientView } from "../window-manager-types";

function route(pathname: string, search: Record<string, unknown> = {}): OsWindowRoute {
  return { pathname, search };
}

function windowFixture(
  app: OsWindow["app"],
  pathname: string,
  instanceKey: string | null = null
): OsWindow {
  return {
    id: osWindowId(app, instanceKey),
    app,
    instanceKey,
    route: route(pathname),
    desktopId: "desktop:main",
    placement: "floating",
    rect: { x: 40, y: 40, w: 720, h: 480 },
    layer: 1,
    minimized: false,
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
  let client: WindowManagerClientView | null = {
    workspaceId: "workspace:test",
    clientId: "client:web",
    activeDesktopId: "desktop:main",
    focusedWindowId: focusedId,
    focusOrder: focusedId === null ? [] : [focusedId],
    connectedAt: "2026-07-22T00:00:00Z",
    presentationRevision: 1,
  };
  let lifecycleResult: Promise<boolean> | null = null;
  let resolveLifecycle: ((accepted: boolean) => void) | null = null;

  const commandOutcome = (apply: () => void) => {
    const completion = lifecycleResult ?? Promise.resolve(true);
    if (lifecycleResult === null) apply();
    else void completion.then(accepted => accepted && apply());
    return { accepted: true, completion };
  };

  const openOrFocus = vi.fn((target: OsOpenTarget) => {
    const id = osWindowId(target.app, target.instanceKey);
    return {
      windowId: id,
      ...commandOutcome(() => {
        const existing = windows[id];
        windows[id] =
          existing ??
          windowFixture(
            target.app,
            target.route?.pathname ?? (target.app === "dashboard" ? "/" : `/${target.app}`),
            target.instanceKey ?? null
          );
        if (target.route) windows[id] = { ...windows[id], route: target.route };
        focusedId = id;
      }),
    };
  });
  const focusWindow = vi.fn((id: string) =>
    commandOutcome(() => {
      if (windows[id]) focusedId = id;
    })
  );
  const closeWindow = vi.fn(async (id: string) => {
    if (lifecycleResult !== null && !(await lifecycleResult)) return false;
    delete windows[id];
    if (focusedId === id) focusedId = Object.keys(windows).at(-1) ?? null;
    return true;
  });
  const minimizeWindow = vi.fn(async (id: string) => {
    if (lifecycleResult !== null && !(await lifecycleResult)) return false;
    if (!windows[id]) return false;
    windows[id] = { ...windows[id], minimized: true };
    if (focusedId === id) focusedId = Object.keys(windows).find(key => key !== id) ?? null;
    return true;
  });
  const zoomWindow = vi.fn((id: string) =>
    commandOutcome(() => {
      if (windows[id]) focusedId = id;
    })
  );
  const navigateWindow = vi.fn((id: string, nextRoute: OsWindowRoute) =>
    commandOutcome(() => {
      if (windows[id]) windows[id] = { ...windows[id], route: nextRoute };
    })
  );
  const resetWorkspace = () => {
    for (const id of Object.keys(windows)) delete windows[id];
    focusedId = null;
  };
  const setAuthoritativeFocus = (id: string, nextRoute?: OsWindowRoute) => {
    if (nextRoute && windows[id]) windows[id] = { ...windows[id], route: nextRoute };
    focusedId = id;
  };
  const setClientConnected = (connected: boolean) => {
    client = connected
      ? {
          workspaceId: "workspace:test",
          clientId: "client:web",
          activeDesktopId: "desktop:main",
          focusedWindowId: focusedId,
          focusOrder: focusedId === null ? [] : [focusedId],
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
    openOrFocus,
    focusWindow,
    closeWindow,
    minimizeWindow,
    zoomWindow,
    navigateWindow,
  };

  return {
    getState: () => state,
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
    spies: { openOrFocus, focusWindow, closeWindow, minimizeWindow, zoomWindow, navigateWindow },
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

  it("Should push exactly once after a user opens an app", async () => {
    const { coordinator, router } = createCoordinator();
    coordinator.completeHydration();

    const id = await coordinator.userOpen({ app: "settings", route: route("/settings/layouts") });

    expect(id).toBe("app:settings");
    expect(router.navigate).toHaveBeenCalledOnce();
    expect(router.navigate).toHaveBeenCalledWith(route("/settings/layouts"));
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
});
