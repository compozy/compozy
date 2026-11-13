// Suite: window-manager runtime projection and command admission
// Invariant: cache data changes drive projections while observer lifecycle stays render-pure;
// unchanged window subscribers skip renders, and accepted presentation intents persist until
// the serialized daemon command settles. Tab, layout, and desktop moves become one v6 command
// each (`window.tab.*`, `window.move` with `move_group`, `layout.arrange` with no frame fields),
// and directional focus only reaches windows on screen in the active tab. A view-only close takes a window off the desk without
// the operator close guard and never races a close that guard is confirming (#1194).
// Owning layer: the Query cache → runtime projection and runtime → command boundaries.
import { QueryClient, QueryObserver } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import { useAtom } from "@xstate/store-react";
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  executeWindowManagerCommand,
  fetchWindowManagerSnapshot,
  WindowManagerApiError,
} from "../../adapters/window-manager-api";
import type { WindowCloseGuard } from "../../lib/window-close-targets";
import type { OsWindowRoute } from "../../lib/os-types";
import { windowManagerKeys } from "../../lib/window-manager-query";
import type { WindowManagerSettingsSection } from "../../lib/window-manager-settings-section";
import type {
  LayoutDesktop,
  WindowManagerChangeSet,
  WindowManagerConfig,
  WindowManagerSnapshot,
  WindowManagerWindow,
} from "../../lib/window-manager-types";
import {
  selectDesktopTransitionIntent,
  windowManagerStore,
} from "../../stores/window-manager-store";
import { beginWindowManagerCommand } from "../../stores/window-manager-store-commands";
import { WindowManagerRuntime } from "../../runtime/window-manager-runtime";
import {
  WINDOW_MANAGER_COMMAND_DEADLINE_MS,
  WINDOW_MANAGER_DIAGNOSTIC_TTL_MS,
} from "../../runtime/window-manager-runtime-helpers";

vi.mock("../../adapters/window-manager-api", async importOriginal => {
  const actual = await importOriginal<typeof import("../../adapters/window-manager-api")>();
  return {
    ...actual,
    executeWindowManagerCommand: vi.fn(),
    fetchWindowManagerSnapshot: vi.fn(),
  };
});

/** Settles one microtask so the runtime's command serializer starts the queued runCommand. */
async function flushCommandQueue(): Promise<void> {
  await Promise.resolve();
}

const CONFIG: WindowManagerConfig = {
  newWindowPolicy: "tab",
  smallViewportPolicy: "stack",
  focusPolicy: "click_directional",
  focusWrap: true,
  focusFollowsPointer: false,
  swapModifier: "shift",
  historyLimit: 50,
  navStackLimit: 50,
  closedEntryLimit: 20,
  desktopTransition: "slide",
  gaps: { inner: 8, top: 0, right: 0, bottom: 0, left: 0 },
  shortcuts: {},
  globalShortcuts: {},
  shortcutDefaults: {},
  effectiveShortcuts: {},
};

const TEST_CONFIG_KEY = windowManagerKeys.config("workspace:test", "client:web", "marketing");
const SETTINGS_SECTION: WindowManagerSettingsSection = {
  scope: "workspace",
  availableScopes: ["user", "workspace"],
  workspaceId: "workspace:test",
  config: CONFIG,
  commands: [],
  aliases: {},
  extensionDefaults: [],
  diagnostics: [],
  globalShortcuts: [],
};

const SNAPSHOT: WindowManagerSnapshot = {
  version: 6,
  workspaceId: "workspace:test",
  revision: 7,
  desktops: [
    { id: "desktop:one", name: "One", order: 0, tabs: [], activeTabId: "" },
    { id: "desktop:two", name: "Two", order: 1, tabs: [], activeTabId: "" },
  ],
  windows: {},
  closedEntryCount: 0,
  overrides: {},
  migration: null,
  updatedAt: "2026-07-22T00:00:00Z",
};

const CLIENT_VIEW_DEFAULTS = { activeTab: {} } as const;
const EMPTY_CHANGES: WindowManagerChangeSet = {
  desktopIds: [],
  windowIds: [],
  tabIds: [],
  nodeIds: [],
  clientIds: [],
  tabSplit: [],
  tabUnsplit: [],
};

/** Test windows each sit alone in the tab named after them unless a test splits them. */
function tabIdFor(windowId: string): string {
  return `tab:${windowId}`;
}

function testWindow(
  id: string,
  app: string,
  route: OsWindowRoute = { pathname: `/${app}`, search: {} },
  extra: Partial<WindowManagerWindow> = {}
): WindowManagerWindow {
  return {
    id,
    app,
    instanceKey: null,
    route,
    navStack: [],
    desktopId: "desktop:one",
    tabId: tabIdFor(id),
    ...extra,
  };
}

/** desktop:one holds one single-window tab per window ID, in order; the last is the default. */
function tabsOnDesktopOne(
  desktops: readonly LayoutDesktop[],
  windowIds: readonly string[],
  pinned: readonly string[] = []
): LayoutDesktop[] {
  return desktops.map(desktop =>
    desktop.id === "desktop:one"
      ? {
          ...desktop,
          tabs: windowIds.map(windowId => ({
            id: tabIdFor(windowId),
            pinned: pinned.includes(windowId),
            root: { id: `leaf:${windowId}`, kind: "leaf" as const, windowId },
          })),
          activeTabId: tabIdFor(windowIds[windowIds.length - 1] ?? ""),
        }
      : desktop
  );
}

function withWindows(
  windows: readonly WindowManagerWindow[],
  base: WindowManagerSnapshot = SNAPSHOT
): WindowManagerSnapshot {
  return {
    ...base,
    desktops: tabsOnDesktopOne(
      base.desktops,
      windows.map(window => window.id)
    ),
    windows: Object.fromEntries(windows.map(window => [window.id, window])),
  };
}

/** Two single-window tabs on desktop:one: Tasks, then the active Agents tab. */
function snapshotWithTabPair(pinned: readonly string[] = []): WindowManagerSnapshot {
  const snapshot = withWindows([
    testWindow("app:tasks", "tasks", { pathname: "/tasks", search: {} }),
    testWindow("app:agents", "agents"),
  ]);
  return {
    ...snapshot,
    desktops: tabsOnDesktopOne(snapshot.desktops, ["app:tasks", "app:agents"], pinned),
  };
}

/** One split tab on desktop:one holding Tasks beside Agents. */
function snapshotWithSplitTab(): WindowManagerSnapshot {
  const tabId = "tab:split";
  return {
    ...SNAPSHOT,
    desktops: SNAPSHOT.desktops.map(desktop =>
      desktop.id === "desktop:one"
        ? {
            ...desktop,
            activeTabId: tabId,
            tabs: [
              {
                id: tabId,
                pinned: false,
                root: {
                  id: "split:one",
                  kind: "split",
                  axis: "horizontal",
                  weights: [0.7, 0.3],
                  children: [
                    { id: "leaf:tasks", kind: "leaf", windowId: "app:tasks" },
                    { id: "leaf:agents", kind: "leaf", windowId: "app:agents" },
                  ],
                },
              },
            ],
          }
        : desktop
    ),
    windows: {
      "app:tasks": testWindow("app:tasks", "tasks", undefined, { tabId }),
      "app:agents": testWindow("app:agents", "agents", undefined, { tabId }),
    },
  };
}

function snapshotWithAgentsRoute(
  route: OsWindowRoute,
  revision = SNAPSHOT.revision
): WindowManagerSnapshot {
  return { ...withWindows([testWindow("app:agents", "agents", route)]), revision };
}

afterEach(() => {
  windowManagerStore.trigger.bindingUnbound();
  windowManagerStore.trigger.workAreaMeasured({ workArea: null });
  vi.clearAllMocks();
});

describe("WindowManagerRuntime", () => {
  it("Should update a moved window without rendering an unchanged window subscriber", () => {
    const queryClient = new QueryClient();
    const queryKey = windowManagerKeys.snapshot("workspace:test", "marketing");
    const snapshot = withWindows([
      testWindow("app:agents", "agents"),
      testWindow("app:tasks", "tasks"),
    ]);
    queryClient.setQueryData(queryKey, snapshot);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.start();
    const renderAgents = vi.fn();
    const renderTasks = vi.fn();
    const agents = renderHook(() => {
      const window = useAtom(runtime.projectionAtom, state => state.windows["app:agents"]);
      renderAgents();
      return window;
    });
    const tasks = renderHook(() => {
      const window = useAtom(runtime.projectionAtom, state => state.windows["app:tasks"]);
      renderTasks();
      return window;
    });
    renderAgents.mockClear();
    renderTasks.mockClear();
    try {
      act(() => {
        queryClient.setQueryData(queryKey, {
          ...snapshot,
          revision: snapshot.revision + 1,
          windows: {
            ...snapshot.windows,
            "app:agents": testWindow("app:agents", "agents", {
              pathname: "/agents/release",
              search: {},
            }),
          },
        });
      });

      expect(agents.result.current?.route.pathname).toBe("/agents/release");
      expect(tasks.result.current?.route.pathname).toBe("/tasks");
      expect(renderAgents).toHaveBeenCalled();
      expect(renderTasks).not.toHaveBeenCalled();
    } finally {
      agents.unmount();
      tasks.unmount();
      runtime.stop();
    }
  });

  it("Should resume authoritative projections after the runtime lifecycle restarts", () => {
    const runtime = new WindowManagerRuntime(new QueryClient());
    runtime.start();

    runtime.stop();
    runtime.start();
    windowManagerStore.trigger.connectionStatusChanged({ status: "connected" });

    expect(runtime.getState().connectionStatus).toBe("connected");
    runtime.stop();
  });

  it("Should project the Query-owned config without a React effect mirror", () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), SNAPSHOT);
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.start();
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });

    expect(runtime.getState().hydration).toBe("pending");

    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);

    expect(runtime.getState().hydration).toBe("live");
    expect(runtime.getState().windowManagerConfig?.historyLimit).toBe(50);
    runtime.stop();
  });

  it("Should publish only when Query cache data changes", () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.start();
    const onProjection = vi.fn();
    const unsubscribe = runtime.subscribe(onProjection);

    new QueryObserver(queryClient, {
      queryKey: TEST_CONFIG_KEY,
      queryFn: () => Promise.resolve(SETTINGS_SECTION),
    });

    expect(onProjection).not.toHaveBeenCalled();

    queryClient.removeQueries({ queryKey: TEST_CONFIG_KEY, exact: true });

    expect(onProjection).toHaveBeenCalledOnce();
    expect(runtime.getState().windowManagerConfig).toBeNull();
    unsubscribe();
    runtime.stop();
  });

  it("Should not republish an unchanged load error", () => {
    const runtime = new WindowManagerRuntime(new QueryClient());
    runtime.start();
    const onProjection = vi.fn();
    const unsubscribe = runtime.subscribe(onProjection);

    runtime.setLoadError(null);
    expect(onProjection).not.toHaveBeenCalled();

    const loadError = new Error("window manager unavailable");
    runtime.setLoadError(loadError);
    runtime.setLoadError(loadError);
    expect(onProjection).toHaveBeenCalledOnce();

    runtime.setLoadError(null);
    runtime.setLoadError(null);
    expect(onProjection).toHaveBeenCalledTimes(2);
    unsubscribe();
    runtime.stop();
  });

  it("Should discard the optimistic transition when a queued switch cannot begin", async () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), SNAPSHOT);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: null,
      focusOrder: [],
      connectedAt: "2026-07-22T00:00:00Z",
    });
    expect(
      beginWindowManagerCommand(windowManagerStore, {
        id: "command:pending",
        kind: "window.move",
        expectedRevision: 7,
      })
    ).toBe(true);

    runtime.switchDesktop("desktop:two");
    await flushCommandQueue();

    expect(selectDesktopTransitionIntent(windowManagerStore.getSnapshot().context)).toBeNull();
    expect(executeWindowManagerCommand).not.toHaveBeenCalled();
    runtime.stop();
  });

  it("Should run queued commands in order instead of dropping rapid interactions", async () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), SNAPSHOT);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    vi.mocked(executeWindowManagerCommand).mockResolvedValue({
      snapshot: SNAPSHOT,
      applied: true,
      changes: EMPTY_CHANGES,
      diagnostics: [],
      client: null,
      deprecations: [],
      rebasedFrom: null,
    });
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: null,
      focusOrder: [],
      connectedAt: "2026-07-22T00:00:00Z",
    });

    const first = runtime.focusWindow("app:first");
    const second = runtime.focusWindow("app:second");
    expect(first.accepted).toBe(true);
    expect(second.accepted).toBe(true);
    await Promise.all([first.completion, second.completion]);

    expect(executeWindowManagerCommand).toHaveBeenCalledTimes(2);
    const windowIds = vi
      .mocked(executeWindowManagerCommand)
      .mock.calls.map(call => (call[4].payload as { window_id: string }).window_id);
    expect(windowIds).toEqual(["app:first", "app:second"]);
    runtime.stop();
  });

  it("Should drop clicks still queued past their deadline instead of replaying them", async () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), SNAPSHOT);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    const deadlines: AbortController[] = [];
    const timeout = vi.spyOn(AbortSignal, "timeout").mockImplementation(() => {
      const controller = new AbortController();
      deadlines.push(controller);
      return controller.signal;
    });
    // A request stalled in the browser's connection pool settles only when its signal aborts.
    vi.mocked(executeWindowManagerCommand).mockImplementation(
      (_workspace, _profile, _client, _revision, _command, signal) =>
        new Promise((_resolve, reject) => {
          signal?.addEventListener("abort", () => reject(signal.reason), { once: true });
        })
    );
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: null,
      focusOrder: [],
      connectedAt: "2026-07-22T00:00:00Z",
    });

    const first = runtime.focusWindow("app:first");
    const second = runtime.focusWindow("app:second");
    expect(deadlines).toHaveLength(2);
    expect(WINDOW_MANAGER_COMMAND_DEADLINE_MS).toBe(8_000);
    expect(timeout.mock.calls).toEqual([
      [WINDOW_MANAGER_COMMAND_DEADLINE_MS],
      [WINDOW_MANAGER_COMMAND_DEADLINE_MS],
    ]);
    await vi.waitFor(() => expect(executeWindowManagerCommand).toHaveBeenCalledOnce());
    for (const deadline of deadlines) deadline.abort(new DOMException("timed out", "TimeoutError"));

    await expect(first.completion).resolves.toBe(false);
    await expect(second.completion).resolves.toBe(false);
    expect(executeWindowManagerCommand).toHaveBeenCalledOnce();
    expect(vi.mocked(executeWindowManagerCommand).mock.calls[0]?.[5]).toBe(deadlines[0]?.signal);
    timeout.mockRestore();
    runtime.stop();
  });

  it("Should open a forced-new terminal at the accepted revision of its queued adoption", async () => {
    const queryClient = new QueryClient();
    const sourceId = "app:agents";
    const tabs = snapshotWithTabPair();
    const initial: WindowManagerSnapshot = {
      ...tabs,
      windows: {
        ...tabs.windows,
        [sourceId]: {
          ...tabs.windows[sourceId]!,
          app: "terminal",
          route: { pathname: "/terminal", search: {} },
        },
      },
    };
    const route = { pathname: "/terminal/term-adopted", search: {} };
    const adopted: WindowManagerSnapshot = {
      ...initial,
      revision: initial.revision + 1,
      windows: {
        ...initial.windows,
        [sourceId]: { ...initial.windows[sourceId]!, instanceKey: "term-adopted", route },
      },
    };
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), initial);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    let completeAdoption!: (
      result: Awaited<ReturnType<typeof executeWindowManagerCommand>>
    ) => void;
    vi.mocked(executeWindowManagerCommand)
      .mockImplementationOnce(
        () =>
          new Promise(resolve => {
            completeAdoption = resolve;
          })
      )
      .mockResolvedValueOnce({
        snapshot: { ...adopted, revision: adopted.revision + 1 },
        applied: true,
        changes: EMPTY_CHANGES,
        diagnostics: [],
        client: null,
        deprecations: [],
        rebasedFrom: null,
      });
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: sourceId,
      focusOrder: [sourceId],
      connectedAt: "2026-07-22T00:00:00Z",
    });

    const adoption = runtime.retargetWindow(sourceId, "term-adopted", route);
    await flushCommandQueue();
    const fresh = runtime.openOrFocus({
      app: "terminal",
      forceNewInstance: true,
      afterWindowId: sourceId,
      route: { pathname: "/terminal", search: { new: "1" } },
    });
    expect(executeWindowManagerCommand).toHaveBeenCalledOnce();
    completeAdoption({
      snapshot: adopted,
      applied: true,
      changes: EMPTY_CHANGES,
      diagnostics: [],
      client: null,
      deprecations: [],
      rebasedFrom: null,
    });
    await expect(Promise.all([adoption.completion, fresh.completion])).resolves.toEqual([
      true,
      true,
    ]);
    expect(executeWindowManagerCommand).toHaveBeenNthCalledWith(
      1,
      "workspace:test",
      "marketing",
      "client:web",
      initial.revision,
      expect.objectContaining({ commandId: "window.navigate" }),
      expect.any(AbortSignal)
    );
    expect(executeWindowManagerCommand).toHaveBeenNthCalledWith(
      2,
      "workspace:test",
      "marketing",
      "client:web",
      adopted.revision,
      expect.objectContaining({
        commandId: "window.open",
        payload: expect.objectContaining({
          window: expect.objectContaining({
            app: "terminal",
            stack_target_window_id: sourceId,
            route: { pathname: "/terminal", search: { new: "1" } },
          }),
        }),
      }),
      expect.any(AbortSignal)
    );
    runtime.stop();
  });

  it("Should share one in-flight semantic open instead of creating a duplicate window", async () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), SNAPSHOT);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    vi.mocked(executeWindowManagerCommand).mockResolvedValue({
      snapshot: SNAPSHOT,
      applied: true,
      changes: EMPTY_CHANGES,
      diagnostics: [],
      client: null,
      deprecations: [],
      rebasedFrom: null,
    });
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: null,
      focusOrder: [],
      connectedAt: "2026-07-22T00:00:00Z",
    });

    const deepLink = runtime.openOrFocus({
      app: "marketplace",
      route: { pathname: "/marketplace", search: {} },
    });
    const dock = runtime.openOrFocus({ app: "marketplace" });

    expect(dock.windowId).toBe(deepLink.windowId);
    await expect(Promise.all([deepLink.completion, dock.completion])).resolves.toEqual([
      true,
      true,
    ]);
    expect(executeWindowManagerCommand).toHaveBeenCalledOnce();
    expect(executeWindowManagerCommand).toHaveBeenCalledWith(
      "workspace:test",
      "marketing",
      "client:web",
      SNAPSHOT.revision,
      expect.objectContaining({
        commandId: "window.open",
        payload: expect.objectContaining({
          window: expect.objectContaining({ app: "marketplace" }),
        }),
      }),
      expect.any(AbortSignal)
    );
    runtime.stop();
  });

  it("Should reject a semantic-open continuation after the runtime binding changes", async () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), SNAPSHOT);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    let resolveOpen!: (result: Awaited<ReturnType<typeof executeWindowManagerCommand>>) => void;
    vi.mocked(executeWindowManagerCommand).mockReturnValueOnce(
      new Promise(resolve => {
        resolveOpen = resolve;
      })
    );
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: null,
      focusOrder: [],
      connectedAt: "2026-07-22T00:00:00Z",
    });

    const first = runtime.openOrFocus({
      app: "session",
      instanceKey: "session-pending",
      route: { pathname: "/sessions/session-pending", search: { tab: "messages" } },
    });
    const continuation = runtime.openOrFocus({
      app: "session",
      instanceKey: "session-pending",
      route: { pathname: "/sessions/session-pending", search: { tab: "events" } },
    });
    await flushCommandQueue();

    const nextSnapshot = { ...SNAPSHOT, workspaceId: "workspace:next" };
    queryClient.setQueryData(
      windowManagerKeys.snapshot("workspace:next", "marketing"),
      nextSnapshot
    );
    queryClient.setQueryData(
      windowManagerKeys.config("workspace:next", "client:next", "marketing"),
      {
        ...SETTINGS_SECTION,
        workspaceId: "workspace:next",
      }
    );
    runtime.bind({
      workspaceId: "workspace:next",
      profileId: "marketing",
      clientId: "client:next",
    });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:next",
      clientId: "client:next",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: null,
      focusOrder: [],
      connectedAt: "2026-07-22T00:00:00Z",
    });
    resolveOpen({
      snapshot: SNAPSHOT,
      applied: true,
      changes: EMPTY_CHANGES,
      diagnostics: [],
      client: null,
      deprecations: [],
      rebasedFrom: null,
    });

    await expect(first.completion).resolves.toBe(true);
    await expect(continuation.completion).resolves.toBe(false);
    expect(executeWindowManagerCommand).toHaveBeenCalledOnce();
    runtime.stop();
  });

  it("Should keep the latest route visible while older navigation commands settle", async () => {
    const initialRoute = { pathname: "/agents", search: {} };
    const releaseRoute = { pathname: "/agents", search: { q: "release" } };
    const opsRoute = {
      pathname: "/agents",
      search: { q: "ops", category: "Engineering / Release" },
    };
    const queryClient = new QueryClient();
    queryClient.setQueryData(
      windowManagerKeys.snapshot("workspace:test", "marketing"),
      snapshotWithAgentsRoute(initialRoute)
    );
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    let resolveFirst!: (result: Awaited<ReturnType<typeof executeWindowManagerCommand>>) => void;
    let resolveSecond!: (result: Awaited<ReturnType<typeof executeWindowManagerCommand>>) => void;
    vi.mocked(executeWindowManagerCommand)
      .mockReturnValueOnce(new Promise(resolve => (resolveFirst = resolve)))
      .mockReturnValueOnce(new Promise(resolve => (resolveSecond = resolve)));
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: "app:agents",
      focusOrder: ["app:agents"],
      connectedAt: "2026-07-22T00:00:00Z",
    });

    const first = runtime.navigateWindow("app:agents", releaseRoute);
    const second = runtime.navigateWindow("app:agents", opsRoute);

    const routeWhileQueued = runtime.getState().windows["app:agents"]?.route;
    expect(runtime.getState().windows["app:agents"]?.instanceKey).toBeNull();
    await flushCommandQueue();
    resolveFirst({
      snapshot: snapshotWithAgentsRoute(releaseRoute, 8),
      applied: true,
      changes: {
        ...EMPTY_CHANGES,
        windowIds: ["app:agents"],
      },
      diagnostics: [],
      client: null,
      deprecations: [],
      rebasedFrom: null,
    });
    await expect(first.completion).resolves.toBe(true);

    const routeAfterIntermediateResult = runtime.getState().windows["app:agents"]?.route;
    await vi.waitFor(() => expect(executeWindowManagerCommand).toHaveBeenCalledTimes(2));
    resolveSecond({
      snapshot: snapshotWithAgentsRoute(opsRoute, 9),
      applied: true,
      changes: {
        ...EMPTY_CHANGES,
        windowIds: ["app:agents"],
      },
      diagnostics: [],
      client: null,
      deprecations: [],
      rebasedFrom: null,
    });

    await expect(second.completion).resolves.toBe(true);
    expect(routeWhileQueued).toEqual(opsRoute);
    expect(routeAfterIntermediateResult).toEqual(opsRoute);
    expect(runtime.getState().windows["app:agents"]?.route).toEqual(opsRoute);
    runtime.stop();
  });

  it.each([
    { label: "named", instanceKey: "reviewer", projectedInstanceKey: "reviewer" },
    { label: "empty", instanceKey: "", projectedInstanceKey: null },
  ])(
    "Should project a pending $label retarget as one semantic route and instance identity",
    async ({ instanceKey, projectedInstanceKey }) => {
      const initialRoute = { pathname: "/agents", search: {} };
      const retargetRoute = { pathname: "/agents/reviewer", search: {} };
      const queryClient = new QueryClient();
      queryClient.setQueryData(
        windowManagerKeys.snapshot("workspace:test", "marketing"),
        snapshotWithAgentsRoute(initialRoute)
      );
      queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
      vi.mocked(executeWindowManagerCommand).mockReturnValue(
        new Promise<Awaited<ReturnType<typeof executeWindowManagerCommand>>>(() => {})
      );
      const runtime = new WindowManagerRuntime(queryClient);
      runtime.bind({
        workspaceId: "workspace:test",
        profileId: "marketing",
        clientId: "client:web",
      });
      runtime.setClient({
        ...CLIENT_VIEW_DEFAULTS,
        workspaceId: "workspace:test",
        clientId: "client:web",
        presentationRevision: 1,
        activeDesktopId: "desktop:one",
        focusedWindowId: "app:agents",
        focusOrder: ["app:agents"],
        connectedAt: "2026-07-22T00:00:00Z",
      });

      runtime.retargetWindow("app:agents", instanceKey, retargetRoute);
      await vi.waitFor(() => expect(executeWindowManagerCommand).toHaveBeenCalledOnce());

      expect(runtime.getState().windows["app:agents"]).toMatchObject({
        instanceKey: projectedInstanceKey,
        route: retargetRoute,
      });
      runtime.stop();
    }
  );

  it("Should restore the authoritative route when navigation is rejected", async () => {
    const initialRoute = { pathname: "/agents", search: { q: "release" } };
    const rejectedRoute = { pathname: "/agents", search: { q: "ops" } };
    const queryClient = new QueryClient();
    queryClient.setQueryData(
      windowManagerKeys.snapshot("workspace:test", "marketing"),
      snapshotWithAgentsRoute(initialRoute)
    );
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    vi.mocked(executeWindowManagerCommand).mockRejectedValueOnce(new Error("navigation rejected"));
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: "app:agents",
      focusOrder: ["app:agents"],
      connectedAt: "2026-07-22T00:00:00Z",
    });

    const outcome = runtime.navigateWindow("app:agents", rejectedRoute);

    const optimisticRoute = runtime.getState().windows["app:agents"]?.route;
    await expect(outcome.completion).resolves.toBe(false);
    expect(optimisticRoute).toEqual(rejectedRoute);
    expect(runtime.getState().windows["app:agents"]?.route).toEqual(initialRoute);
    runtime.stop();
  });

  it("Should reject a queued command after the runtime binding changes", async () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), SNAPSHOT);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    let resolveFirst!: (result: {
      snapshot: WindowManagerSnapshot;
      applied: boolean;
      changes: WindowManagerChangeSet;
      diagnostics: [];
      client: null;
      deprecations: [];
      rebasedFrom: null;
    }) => void;
    vi.mocked(executeWindowManagerCommand).mockReturnValueOnce(
      new Promise(resolve => {
        resolveFirst = resolve;
      })
    );
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: null,
      focusOrder: [],
      connectedAt: "2026-07-22T00:00:00Z",
    });

    const first = runtime.focusWindow("app:first");
    const stale = runtime.focusWindow("app:stale");
    await flushCommandQueue();

    const nextSnapshot = { ...SNAPSHOT, workspaceId: "workspace:next" };
    queryClient.setQueryData(
      windowManagerKeys.snapshot("workspace:next", "marketing"),
      nextSnapshot
    );
    runtime.bind({
      workspaceId: "workspace:next",
      profileId: "marketing",
      clientId: "client:next",
    });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:next",
      clientId: "client:next",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: null,
      focusOrder: [],
      connectedAt: "2026-07-22T00:00:00Z",
    });
    resolveFirst({
      snapshot: SNAPSHOT,
      applied: true,
      changes: EMPTY_CHANGES,
      diagnostics: [],
      client: null,
      deprecations: [],
      rebasedFrom: null,
    });

    await expect(first.completion).resolves.toBe(true);
    await expect(stale.completion).resolves.toBe(false);
    expect(executeWindowManagerCommand).toHaveBeenCalledOnce();
    runtime.stop();
  });

  it("Should synthesize a slide transition when reconciliation changes the active desktop", () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), SNAPSHOT);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: null,
      focusOrder: [],
      connectedAt: "2026-07-22T00:00:00Z",
    });
    expect(selectDesktopTransitionIntent(windowManagerStore.getSnapshot().context)).toBeNull();

    // A tab activation or cross-desktop focus arrives as a reconciled client frame with
    // no locally staged intent; the runtime supplies the transition itself.
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 2,
      activeDesktopId: "desktop:two",
      focusedWindowId: null,
      focusOrder: [],
      connectedAt: "2026-07-22T00:00:00Z",
    });

    expect(selectDesktopTransitionIntent(windowManagerStore.getSnapshot().context)).toEqual({
      fromDesktopId: "desktop:one",
      toDesktopId: "desktop:two",
      direction: "later",
      mode: "slide",
    });
    runtime.stop();
  });

  it("Should keep a staged optimistic transition when an intermediate client result lands", () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), SNAPSHOT);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: null,
      focusOrder: [],
      connectedAt: "2026-07-22T00:00:00Z",
    });
    // Rapid pager clicks: the latest queued switch staged its own intent; the
    // earlier switch's result must not replace it with a synthesized one.
    const staged = {
      fromDesktopId: "desktop:one",
      toDesktopId: "desktop:three",
      direction: "later",
      mode: "slide",
    } as const;
    windowManagerStore.trigger.transitionIntentChanged({ intent: staged });

    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 2,
      activeDesktopId: "desktop:two",
      focusedWindowId: null,
      focusOrder: [],
      connectedAt: "2026-07-22T00:00:00Z",
    });

    expect(selectDesktopTransitionIntent(windowManagerStore.getSnapshot().context)).toEqual(staged);
    runtime.stop();
  });

  it("Should not clear a new binding transition when an old desktop switch rejects", async () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), SNAPSHOT);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: null,
      focusOrder: [],
      connectedAt: "2026-07-22T00:00:00Z",
    });
    let rejectCommand!: (reason: Error) => void;
    vi.mocked(executeWindowManagerCommand).mockReturnValueOnce(
      new Promise((_, reject) => {
        rejectCommand = reject;
      })
    );

    runtime.switchDesktop("desktop:two");
    await flushCommandQueue();
    expect(executeWindowManagerCommand).toHaveBeenCalledOnce();
    runtime.bind({
      workspaceId: "workspace:next",
      profileId: "marketing",
      clientId: "client:next",
    });
    windowManagerStore.trigger.transitionIntentChanged({
      intent: {
        fromDesktopId: "desktop:next-one",
        toDesktopId: "desktop:next-two",
        direction: "later",
        mode: "crossfade",
      },
    });
    rejectCommand(new Error("old binding failed"));
    await vi.waitFor(() => {
      expect(selectDesktopTransitionIntent(windowManagerStore.getSnapshot().context)).toEqual({
        fromDesktopId: "desktop:next-one",
        toDesktopId: "desktop:next-two",
        direction: "later",
        mode: "crossfade",
      });
    });
    runtime.stop();
  });

  it("Should accept a reset presentation revision after explicit client invalidation", () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), SNAPSHOT);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    const view = {
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      profileId: "marketing",
      clientId: "client:web",
      activeDesktopId: "desktop:one",
      focusedWindowId: null,
      focusOrder: [],
      connectedAt: "2026-07-22T00:00:00Z",
    };
    runtime.setClient({ ...view, presentationRevision: 7 });

    runtime.setClient(null);
    runtime.setClient({ ...view, presentationRevision: 1 });
    runtime.setClient({ ...view, presentationRevision: 2, activeDesktopId: "desktop:two" });

    expect(runtime.getState().client).toMatchObject({
      presentationRevision: 2,
      activeDesktopId: "desktop:two",
    });
    runtime.stop();
  });

  it("Should retain the local attachment token across tokenless stream updates", () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), SNAPSHOT);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    const view = {
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      profileId: "marketing",
      clientId: "client:web",
      activeDesktopId: "desktop:one",
      focusedWindowId: null,
      focusOrder: [],
      connectedAt: "2026-07-22T00:00:00Z",
    };

    runtime.setClient({ ...view, presentationRevision: 1 });
    runtime.setClient({ ...view, presentationRevision: 1, attachmentToken: "attachment-token" });
    runtime.setClient({ ...view, presentationRevision: 2 });

    expect(runtime.getState().clientAttachmentToken).toBe("attachment-token");
    runtime.setClient(null);
    expect(runtime.getState().clientAttachmentToken).toBeNull();
    runtime.stop();
  });

  it("[UT-040] Should open a distinct opaque Tasks instance when the caller explicitly requests one", async () => {
    const existingId = "w-existing-tasks-instance";
    const snapshot: WindowManagerSnapshot = {
      ...SNAPSHOT,
      desktops: tabsOnDesktopOne(SNAPSHOT.desktops, [existingId]),
      windows: {
        [existingId]: {
          id: existingId,
          app: "tasks",
          instanceKey: null,
          route: { pathname: "/tasks", search: {} },
          navStack: [],
          desktopId: "desktop:one",
          tabId: tabIdFor(existingId),
        },
      },
    };
    const queryClient = new QueryClient();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), snapshot);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    vi.mocked(executeWindowManagerCommand).mockResolvedValue({
      snapshot,
      applied: true,
      changes: EMPTY_CHANGES,
      diagnostics: [],
      client: null,
      deprecations: [],
      rebasedFrom: null,
    });
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: existingId,
      focusOrder: [existingId],
      connectedAt: "2026-07-22T00:00:00Z",
    });

    const outcome = runtime.openOrFocus({
      app: "tasks",
      forceNewInstance: true,
      route: { pathname: "/tasks", search: { view: "board" } },
    });

    expect(outcome.accepted).toBe(true);
    await expect(outcome.completion).resolves.toBe(true);
    expect(executeWindowManagerCommand).toHaveBeenCalledOnce();
    expect(executeWindowManagerCommand).toHaveBeenCalledWith(
      "workspace:test",
      "marketing",
      "client:web",
      snapshot.revision,
      expect.objectContaining({
        commandId: "window.open",
        payload: expect.objectContaining({
          window: expect.objectContaining({
            app: "tasks",
            id: expect.stringMatching(/^w-[0-9a-f]{26}$/),
            route: { pathname: "/tasks", search: { view: "board" } },
          }),
        }),
      }),
      expect.any(AbortSignal)
    );
    const command = vi.mocked(executeWindowManagerCommand).mock.calls[0]?.[4];
    if (command === undefined) {
      throw new Error("Expected the forced instance request to dispatch a window command.");
    }
    const openedId = (command.payload as { window: { id: string } }).window.id;
    expect(openedId).not.toBe(existingId);
    runtime.stop();
  });

  it("Should carry an explicit split, a tab after a window, or a target desktop onto the open command", async () => {
    const queryClient = new QueryClient();
    const snapshot = snapshotWithTabPair();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), snapshot);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    vi.mocked(executeWindowManagerCommand).mockResolvedValue({
      snapshot: { ...snapshot, revision: 8 },
      applied: true,
      changes: EMPTY_CHANGES,
      diagnostics: [],
      deprecations: [],
      client: null,
      rebasedFrom: null,
    });
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: null,
      focusOrder: [],
      connectedAt: "2026-07-22T00:00:00Z",
    });
    const openedWindow = async (target: Parameters<typeof runtime.openOrFocus>[0]) => {
      vi.mocked(executeWindowManagerCommand).mockClear();
      await runtime.openOrFocus(target).completion;
      const command = vi.mocked(executeWindowManagerCommand).mock.calls[0]?.[4];
      return (command?.payload as { window: Record<string, unknown> } | undefined)?.window;
    };

    const plain = await openedWindow({ app: "tasks", forceNewInstance: true });
    expect(plain).toMatchObject({ desktop_id: "desktop:one" });
    // Retired floating fields never travel: every new window is a tab or a split member.
    expect(plain).not.toHaveProperty("floating");
    expect(plain).not.toHaveProperty("floating_rect");
    expect(plain).not.toHaveProperty("insert_tiled");
    expect(
      await openedWindow({ app: "tasks", forceNewInstance: true, placement: "split" })
    ).toMatchObject({ insert_tiled: true });
    expect(
      await openedWindow({ app: "tasks", forceNewInstance: true, afterWindowId: "app:tasks" })
    ).toMatchObject({ stack_target_window_id: "app:tasks", desktop_id: "desktop:one" });
    expect(
      await openedWindow({ app: "tasks", forceNewInstance: true, desktopId: "desktop:two" })
    ).toMatchObject({ desktop_id: "desktop:two" });
    runtime.stop();
  });

  it("[UT-041] Should keep another Tasks instance's route intent after closing its sibling", async () => {
    const firstId = "w-tasks-first";
    const secondId = "w-tasks-second";
    const secondRoute = { pathname: "/tasks", search: { view: "board", owner: "me" } };
    const snapshot: WindowManagerSnapshot = {
      ...SNAPSHOT,
      desktops: tabsOnDesktopOne(SNAPSHOT.desktops, [firstId, secondId]),
      windows: {
        [firstId]: {
          id: firstId,
          app: "tasks",
          instanceKey: null,
          route: { pathname: "/tasks", search: { view: "list" } },
          navStack: [],
          desktopId: "desktop:one",
          tabId: tabIdFor(firstId),
        },
        [secondId]: {
          id: secondId,
          app: "tasks",
          instanceKey: null,
          route: { pathname: "/tasks", search: { view: "calendar" } },
          navStack: [],
          desktopId: "desktop:one",
          tabId: tabIdFor(secondId),
        },
      },
    };
    const closedSnapshot: WindowManagerSnapshot = {
      ...snapshot,
      revision: snapshot.revision + 1,
      desktops: tabsOnDesktopOne(snapshot.desktops, [secondId]),
      windows: {
        [secondId]: snapshot.windows[secondId] as WindowManagerSnapshot["windows"][string],
      },
    };
    const queryClient = new QueryClient();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), snapshot);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    vi.mocked(executeWindowManagerCommand).mockResolvedValue({
      snapshot: closedSnapshot,
      applied: true,
      changes: { ...EMPTY_CHANGES, windowIds: [firstId] },
      diagnostics: [],
      client: null,
      deprecations: [],
      rebasedFrom: null,
    });
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: firstId,
      focusOrder: [firstId, secondId],
      connectedAt: "2026-07-22T00:00:00Z",
    });
    windowManagerStore.trigger.routeIntentSet({
      intent: { id: "intent:tasks-second", windowId: secondId, route: secondRoute },
    });
    runtime.setLoadError(null);

    await expect(runtime.closeWindow(firstId)).resolves.toBe(true);

    expect(runtime.getState().windows[firstId]).toBeUndefined();
    expect(runtime.getState().windows[secondId]?.route).toEqual(secondRoute);
    expect(windowManagerStore.getSnapshot().context.routeIntents[secondId]).toEqual({
      id: "intent:tasks-second",
      windowId: secondId,
      route: secondRoute,
    });
    runtime.stop();
  });

  it("Should focus an existing same-route window instead of navigating without a route change", async () => {
    const queryClient = new QueryClient();
    const snapshot: WindowManagerSnapshot = {
      ...SNAPSHOT,
      desktops: tabsOnDesktopOne(SNAPSHOT.desktops, ["app:tasks"]),
      windows: {
        "app:tasks": {
          id: "app:tasks",
          app: "tasks",
          instanceKey: null,
          route: {
            pathname: "/tasks",
            search: { filters: { owner: "me", state: "open" }, panel: "activity" },
          },
          navStack: [],
          desktopId: "desktop:one",
          tabId: tabIdFor("app:tasks"),
        },
      },
    };
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), snapshot);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    vi.mocked(executeWindowManagerCommand).mockReturnValue(
      new Promise<Awaited<ReturnType<typeof executeWindowManagerCommand>>>(() => {})
    );
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: null,
      focusOrder: [],
      connectedAt: "2026-07-22T00:00:00Z",
    });

    runtime.openOrFocus({
      app: "tasks",
      route: {
        pathname: "/tasks",
        search: { panel: "activity", filters: { state: "open", owner: "me" } },
      },
    });
    await vi.waitFor(() => expect(executeWindowManagerCommand).toHaveBeenCalledOnce());
    expect(executeWindowManagerCommand).toHaveBeenCalledWith(
      "workspace:test",
      "marketing",
      "client:web",
      7,
      {
        commandId: "window.focus",
        payload: { window_id: "app:tasks", direction: "" },
        rebase: { windowId: "app:tasks" },
      },
      expect.any(AbortSignal)
    );
    runtime.stop();
  });

  it("Should publish a command snapshot and its focused client as one projection", async () => {
    const queryClient = new QueryClient();
    const agents = snapshotWithAgentsRoute({ pathname: "/agents", search: {} });
    const sessionId = "session:sess-new";
    const sessionRoute = { pathname: "/agents/general/sessions/sess-new", search: {} };
    const resultSnapshot: WindowManagerSnapshot = {
      ...agents,
      revision: 8,
      desktops: tabsOnDesktopOne(agents.desktops, ["app:agents", sessionId]),
      windows: {
        ...agents.windows,
        [sessionId]: {
          ...agents.windows["app:agents"]!,
          id: sessionId,
          app: "session",
          instanceKey: "sess-new",
          route: sessionRoute,
          tabId: tabIdFor(sessionId),
        },
      },
    };
    const focusedClient = {
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      profileId: "marketing",
      clientId: "client:web",
      presentationRevision: 2,
      activeDesktopId: "desktop:one",
      focusedWindowId: sessionId,
      focusOrder: [sessionId, "app:agents"],
      connectedAt: "2026-07-22T00:00:00Z",
    };
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), agents);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    vi.mocked(executeWindowManagerCommand).mockResolvedValue({
      snapshot: resultSnapshot,
      applied: true,
      changes: { ...EMPTY_CHANGES, windowIds: [sessionId], clientIds: ["client:web"] },
      diagnostics: [],
      client: focusedClient,
      deprecations: [],
      rebasedFrom: null,
    });
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.start();
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({ ...focusedClient, presentationRevision: 1, focusedWindowId: "app:agents" });
    const observed = [runtime.getState()];
    const unsubscribe = runtime.subscribe(state => observed.push(state));

    const outcome = runtime.openOrFocus({
      app: "session",
      instanceKey: "sess-new",
      route: sessionRoute,
    });

    await expect(outcome.completion).resolves.toBe(true);
    const focusedAfterOpen = observed
      .filter(state => state.windows[sessionId] !== undefined)
      .map(state => state.focusedId);
    expect(focusedAfterOpen.length).toBeGreaterThan(0);
    expect(new Set(focusedAfterOpen)).toEqual(new Set([sessionId]));
    unsubscribe();
    runtime.stop();
  });

  it("Should recover a semantic open after a concurrent client advances topology", async () => {
    const queryClient = new QueryClient();
    const marketplace = {
      navStack: [],
      id: "app:marketplace",
      app: "marketplace",
      instanceKey: null,
      route: { pathname: "/marketplace/skills", search: {} },
      desktopId: "desktop:one",
      tabId: tabIdFor("app:marketplace"),
    };
    const session = {
      ...marketplace,
      id: "session:sess-concurrent",
      app: "session",
      instanceKey: "sess-concurrent",
      route: { pathname: "/agents/general/sessions/sess-concurrent", search: {} },
      tabId: tabIdFor("session:sess-concurrent"),
    };
    const initial = {
      ...SNAPSHOT,
      desktops: tabsOnDesktopOne(SNAPSHOT.desktops, [marketplace.id]),
      windows: { [marketplace.id]: marketplace },
    };
    const refreshed = {
      ...initial,
      revision: 8,
      desktops: tabsOnDesktopOne(initial.desktops, [marketplace.id, session.id]),
      windows: { [marketplace.id]: marketplace, [session.id]: session },
    };
    const focusedClient = {
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      profileId: "marketing",
      clientId: "client:web",
      presentationRevision: 2,
      activeDesktopId: "desktop:one",
      focusedWindowId: session.id,
      focusOrder: [session.id, marketplace.id],
      connectedAt: "2026-07-22T00:00:00Z",
    };
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), initial);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    vi.mocked(executeWindowManagerCommand)
      .mockRejectedValueOnce(
        new WindowManagerApiError("Window layout changed.", 409, {
          error: "Window layout changed.",
          code: "revision_conflict",
          workspaceId: "workspace:test",
          currentRevision: refreshed.revision,
          conflicts: [],
          diagnostics: [
            { code: "revision_conflict", path: null, message: "Window layout changed." },
          ],
        })
      )
      .mockResolvedValueOnce({
        snapshot: refreshed,
        applied: true,
        changes: {
          ...EMPTY_CHANGES,
          clientIds: [focusedClient.clientId],
        },
        diagnostics: [],
        client: focusedClient,
        deprecations: [],
        rebasedFrom: null,
      });
    vi.mocked(fetchWindowManagerSnapshot).mockResolvedValue(refreshed);
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      ...focusedClient,
      presentationRevision: 1,
      focusedWindowId: marketplace.id,
    });

    const outcome = runtime.openOrFocus({
      app: "session",
      instanceKey: "sess-concurrent",
      route: session.route,
    });

    // A newer snapshot can arrive between semantic lookup and serialized dispatch.
    // Keep the lookup revision so the daemon refuses and recovery focuses its existing instance.
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), refreshed);

    expect(outcome.accepted).toBe(true);
    await expect(outcome.completion).resolves.toBe(true);
    expect(fetchWindowManagerSnapshot).toHaveBeenCalledOnce();
    expect(executeWindowManagerCommand).toHaveBeenNthCalledWith(
      1,
      "workspace:test",
      "marketing",
      "client:web",
      7,
      expect.objectContaining({ commandId: "window.open" }),
      expect.any(AbortSignal)
    );
    expect(executeWindowManagerCommand).toHaveBeenNthCalledWith(
      2,
      "workspace:test",
      "marketing",
      "client:web",
      8,
      {
        commandId: "window.focus",
        payload: { window_id: session.id, direction: "" },
        rebase: { windowId: session.id },
      },
      expect.any(AbortSignal)
    );
    expect(runtime.getState().focusedId).toBe(session.id);
    runtime.stop();
  });

  it("Should stop after one retry when concurrent topology changes twice", async () => {
    const queryClient = new QueryClient();
    const refreshed = { ...SNAPSHOT, revision: 8 };
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), SNAPSHOT);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    vi.mocked(executeWindowManagerCommand)
      .mockRejectedValueOnce(
        new WindowManagerApiError("Window layout changed.", 409, {
          error: "Window layout changed.",
          code: "revision_conflict",
          workspaceId: "workspace:test",
          currentRevision: 8,
          conflicts: [],
          diagnostics: [
            { code: "revision_conflict", path: null, message: "Window layout changed." },
          ],
        })
      )
      .mockRejectedValueOnce(
        new WindowManagerApiError("Window layout changed again.", 409, {
          error: "Window layout changed again.",
          code: "revision_conflict",
          workspaceId: "workspace:test",
          currentRevision: 9,
          conflicts: [],
          diagnostics: [
            { code: "revision_conflict", path: null, message: "Window layout changed again." },
          ],
        })
      );
    vi.mocked(fetchWindowManagerSnapshot).mockResolvedValue(refreshed);
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: null,
      focusOrder: [],
      connectedAt: "2026-07-22T00:00:00Z",
    });

    const outcome = runtime.openOrFocus({
      app: "tasks",
      route: { pathname: "/tasks", search: {} },
    });

    await expect(outcome.completion).resolves.toBe(false);
    // Every conflict re-reads the layout once so the surface never stays read-only.
    expect(fetchWindowManagerSnapshot).toHaveBeenCalledTimes(2);
    expect(executeWindowManagerCommand).toHaveBeenCalledTimes(2);
    expect(windowManagerStore.getSnapshot().context.commandState.status).not.toBe("conflict");
    runtime.stop();
  });

  it("Should abort and ignore snapshot refreshes after binding lifecycle changes", async () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    const requests: Array<{
      resolve: (snapshot: WindowManagerSnapshot) => void;
      signal: AbortSignal | undefined;
    }> = [];
    vi.mocked(fetchWindowManagerSnapshot).mockImplementation(
      (_workspaceId, _profileId, signal) =>
        new Promise(resolve => {
          requests.push({ resolve, signal });
        })
    );
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({
      workspaceId: "workspace:first",
      profileId: "marketing",
      clientId: "client:first",
    });

    const reboundRefresh = runtime.refreshSnapshot();
    expect(requests).toHaveLength(1);
    runtime.bind({
      workspaceId: "workspace:second",
      profileId: "marketing",
      clientId: "client:second",
    });
    expect(requests[0]?.signal?.aborted).toBe(true);
    requests[0]?.resolve({ ...SNAPSHOT, workspaceId: "workspace:first", revision: 20 });
    await expect(reboundRefresh).resolves.toBe(false);
    expect(
      queryClient.getQueryData(windowManagerKeys.snapshot("workspace:first", "marketing"))
    ).toBeUndefined();

    const unboundRefresh = runtime.refreshSnapshot();
    expect(requests).toHaveLength(2);
    runtime.unbind();
    expect(requests[1]?.signal?.aborted).toBe(true);
    requests[1]?.resolve({ ...SNAPSHOT, workspaceId: "workspace:second", revision: 21 });
    await expect(unboundRefresh).resolves.toBe(false);
    expect(
      queryClient.getQueryData(windowManagerKeys.snapshot("workspace:second", "marketing"))
    ).toBeUndefined();
    runtime.stop();
  });

  it("Should preserve an unrelated conflict when semantic open admission is blocked", async () => {
    const queryClient = new QueryClient();
    const originalRoute = { pathname: "/agents", search: {} };
    queryClient.setQueryData(
      windowManagerKeys.snapshot("workspace:test", "marketing"),
      snapshotWithAgentsRoute(originalRoute)
    );
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: null,
      focusOrder: [],
      connectedAt: "2026-07-22T00:00:00Z",
    });
    expect(
      beginWindowManagerCommand(windowManagerStore, {
        id: "layout:stale",
        kind: "layout.arrange",
        expectedRevision: 6,
      })
    ).toBe(true);
    windowManagerStore.trigger.conflictRecorded({
      conflict: { commandId: "layout:stale", expectedRevision: 6, currentRevision: 7 },
      diagnostic: {
        code: "revision_conflict",
        message: "Desktop layout changed remotely.",
        severity: "error",
        field: null,
      },
      binding: { workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" },
    });

    const outcome = runtime.openOrFocus({
      app: "tasks",
      route: { pathname: "/tasks", search: {} },
    });

    // Invariant: a known conflict refuses admission before publishing an optimistic route.
    // Owner: runtime command admission; canonical suite: WindowManagerRuntime.
    expect(outcome.accepted).toBe(false);
    const navigation = runtime.navigateWindow("app:agents", {
      pathname: "/agents/reviewer",
      search: {},
    });
    expect(navigation.accepted).toBe(false);
    expect(runtime.getState().windows["app:agents"]?.route).toEqual(originalRoute);
    await expect(navigation.completion).resolves.toBe(false);
    await expect(outcome.completion).resolves.toBe(false);
    expect(executeWindowManagerCommand).not.toHaveBeenCalled();
    expect(fetchWindowManagerSnapshot).not.toHaveBeenCalled();
    expect(windowManagerStore.getSnapshot().context.commandState).toMatchObject({
      status: "conflict",
      conflict: { commandId: "layout:stale" },
    });
    runtime.stop();
  });

  it("Should return the exact completion outcomes for tab and layout commands", async () => {
    const queryClient = new QueryClient();
    const snapshot = snapshotWithTabPair();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), snapshot);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    vi.mocked(executeWindowManagerCommand).mockResolvedValue({
      snapshot: { ...snapshot, revision: 8 },
      applied: false,
      changes: EMPTY_CHANGES,
      diagnostics: [],
      deprecations: [],
      client: null,
      rebasedFrom: null,
    });
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: "app:agents",
      focusOrder: ["app:agents"],
      connectedAt: "2026-07-22T00:00:00Z",
    });

    const reorder = runtime.reorderTab("app:agents", 0);
    const activate = runtime.activateTab("app:tasks");
    const move = runtime.moveWindow("app:agents", {
      destinationDesktopId: "desktop:one",
      targetWindowId: "app:tasks",
      placement: "right",
    });

    for (const outcome of [reorder, activate, move]) {
      expect(outcome.accepted).toBe(true);
      await expect(outcome.completion).resolves.toBe(false);
    }
    expect(vi.mocked(executeWindowManagerCommand).mock.calls.map(call => call[4])).toEqual([
      expect.objectContaining({
        commandId: "window.tab.reorder",
        payload: { window_id: "app:agents", index: 0 },
      }),
      expect.objectContaining({
        commandId: "window.tab.activate",
        payload: { window_id: "app:tasks" },
      }),
      expect.objectContaining({
        commandId: "window.move",
        payload: {
          window_id: "app:agents",
          destination_desktop_id: "desktop:one",
          target_window_id: "app:tasks",
          placement: "right",
          move_group: false,
        },
      }),
    ]);
    runtime.stop();
  });

  it("Should move a window's whole tab to the end of another desktop's bar", async () => {
    const queryClient = new QueryClient();
    const snapshot = snapshotWithSplitTab();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), snapshot);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    vi.mocked(executeWindowManagerCommand).mockResolvedValue({
      snapshot: { ...snapshot, revision: 8 },
      applied: true,
      changes: EMPTY_CHANGES,
      diagnostics: [],
      deprecations: [],
      client: null,
      rebasedFrom: null,
    });
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: "app:tasks",
      focusOrder: ["app:tasks"],
      connectedAt: "2026-07-22T00:00:00Z",
    });

    runtime.moveWindowToDesktop("app:tasks", "desktop:two");
    await flushCommandQueue();

    expect(vi.mocked(executeWindowManagerCommand).mock.calls[0]?.[4]).toEqual({
      commandId: "window.move",
      payload: {
        window_id: "app:tasks",
        destination_desktop_id: "desktop:two",
        placement: "tab",
        move_group: true,
      },
    });
    runtime.stop();
  });

  it("Should move focus between the windows on screen in the active tab", async () => {
    const queryClient = new QueryClient();
    const snapshot = snapshotWithSplitTab();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), snapshot);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    windowManagerStore.trigger.workAreaMeasured({
      workArea: { rect: { x: 0, y: 0, w: 1280, h: 800 }, origin: { x: 0, y: 0 } },
    });
    vi.mocked(executeWindowManagerCommand).mockResolvedValue({
      snapshot: { ...snapshot, revision: 8 },
      applied: true,
      changes: EMPTY_CHANGES,
      diagnostics: [],
      deprecations: [],
      client: null,
      rebasedFrom: null,
    });
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: "app:tasks",
      focusOrder: ["app:tasks"],
      connectedAt: "2026-07-22T00:00:00Z",
    });

    runtime.focusDirection("right");
    await flushCommandQueue();

    expect(vi.mocked(executeWindowManagerCommand).mock.calls.map(call => call[4])).toEqual([
      {
        commandId: "window.focus",
        payload: { window_id: "app:agents", direction: "" },
        rebase: { windowId: "app:agents" },
      },
    ]);
    runtime.stop();
  });

  it.each([
    {
      anchor: "app:board",
      preset: "columns" as const,
      windowIds: ["app:board", "app:tasks", "app:agents"],
    },
    {
      anchor: "app:agents",
      preset: "two-up" as const,
      windowIds: ["app:agents", "app:tasks"],
    },
  ])(
    "Should arrange the anchor with one window per other tab ($preset from $anchor)",
    async ({ anchor, preset, windowIds }) => {
      const queryClient = new QueryClient();
      const snapshot = withWindows([
        testWindow("app:tasks", "tasks"),
        testWindow("app:agents", "agents"),
        testWindow("app:board", "tasks", { pathname: "/tasks", search: { view: "board" } }),
      ]);
      queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), snapshot);
      queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
      vi.mocked(executeWindowManagerCommand).mockResolvedValue({
        snapshot: { ...snapshot, revision: 8 },
        applied: true,
        changes: EMPTY_CHANGES,
        diagnostics: [],
        deprecations: [],
        client: null,
        rebasedFrom: null,
      });
      const runtime = new WindowManagerRuntime(queryClient);
      runtime.bind({
        workspaceId: "workspace:test",
        profileId: "marketing",
        clientId: "client:web",
      });
      runtime.setClient({
        ...CLIENT_VIEW_DEFAULTS,
        workspaceId: "workspace:test",
        clientId: "client:web",
        presentationRevision: 1,
        activeDesktopId: "desktop:one",
        focusedWindowId: anchor,
        focusOrder: [anchor],
        connectedAt: "2026-07-22T00:00:00Z",
      });

      runtime.arrangeLayout(anchor, preset);

      await vi.waitFor(() => expect(executeWindowManagerCommand).toHaveBeenCalledTimes(1));
      expect(vi.mocked(executeWindowManagerCommand).mock.calls[0]?.[4]).toEqual({
        commandId: "layout.arrange",
        payload: {
          desktop_id: "desktop:one",
          window_ids: windowIds,
          arrangement: "horizontal",
        },
      });
      runtime.stop();
    }
  );

  it("Should balance every split of the focused window's tab, never its leaf node", async () => {
    const queryClient = new QueryClient();
    const snapshot = snapshotWithSplitTab();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), snapshot);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    vi.mocked(executeWindowManagerCommand).mockResolvedValue({
      snapshot: { ...snapshot, revision: 8 },
      applied: true,
      changes: EMPTY_CHANGES,
      diagnostics: [],
      deprecations: [],
      client: null,
      rebasedFrom: null,
    });
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: "app:tasks",
      focusOrder: ["app:tasks"],
      connectedAt: "2026-07-22T00:00:00Z",
    });

    runtime.balanceFocusedLayout();
    await flushCommandQueue();

    // The daemon rejects a split_id that names a leaf, so the tab is the target.
    expect(vi.mocked(executeWindowManagerCommand).mock.calls.map(call => call[4])).toEqual([
      expect.objectContaining({ commandId: "layout.balance", payload: { tab_id: "tab:split" } }),
    ]);
    runtime.stop();
  });

  it("Should describe a bare daemon refusal code in plain language", async () => {
    vi.useFakeTimers();
    try {
      const queryClient = new QueryClient();
      queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), SNAPSHOT);
      queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
      vi.mocked(executeWindowManagerCommand).mockRejectedValueOnce(
        new WindowManagerApiError("window_manager_invalid_command", 422, {
          error: "window_manager_invalid_command",
          code: "window_manager_invalid_command",
          workspaceId: "workspace:test",
          currentRevision: null,
          conflicts: [],
          diagnostics: [],
        })
      );
      const runtime = new WindowManagerRuntime(queryClient);
      runtime.bind({
        workspaceId: "workspace:test",
        profileId: "marketing",
        clientId: "client:web",
      });
      runtime.setClient({
        ...CLIENT_VIEW_DEFAULTS,
        workspaceId: "workspace:test",
        clientId: "client:web",
        presentationRevision: 1,
        activeDesktopId: "desktop:one",
        focusedWindowId: null,
        focusOrder: [],
        connectedAt: "2026-07-22T00:00:00Z",
      });

      runtime.createDesktop();
      await vi.advanceTimersByTimeAsync(0);

      const state = windowManagerStore.getSnapshot().context.commandState;
      expect(state.status).toBe("idle");
      expect(state.diagnostic?.code).toBe("window_manager_invalid_command");
      expect(state.diagnostic?.message).toBe("That arrangement isn't possible here.");
      runtime.stop();
    } finally {
      vi.useRealTimers();
    }
  });

  it("Should accept the daemon's lower revision when a conflict reveals a reset", async () => {
    vi.useFakeTimers();
    try {
      const queryClient = new QueryClient();
      const reset = { ...SNAPSHOT, revision: 0 };
      queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), {
        ...SNAPSHOT,
        revision: 102,
      });
      queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
      vi.mocked(executeWindowManagerCommand).mockRejectedValueOnce(
        new WindowManagerApiError("window_manager_revision_conflict", 409, {
          error: "window_manager_revision_conflict",
          code: "window_manager_revision_conflict",
          workspaceId: "workspace:test",
          currentRevision: 0,
          conflicts: [],
          diagnostics: [],
        })
      );
      vi.mocked(fetchWindowManagerSnapshot).mockResolvedValue(reset);
      const runtime = new WindowManagerRuntime(queryClient);
      runtime.bind({
        workspaceId: "workspace:test",
        profileId: "marketing",
        clientId: "client:web",
      });
      runtime.setClient({
        ...CLIENT_VIEW_DEFAULTS,
        workspaceId: "workspace:test",
        clientId: "client:web",
        presentationRevision: 1,
        activeDesktopId: "desktop:one",
        focusedWindowId: null,
        focusOrder: [],
        connectedAt: "2026-07-22T00:00:00Z",
      });

      runtime.createDesktop();
      await vi.advanceTimersByTimeAsync(0);

      expect(fetchWindowManagerSnapshot).toHaveBeenCalledOnce();
      expect(
        queryClient.getQueryData<WindowManagerSnapshot>(
          windowManagerKeys.snapshot("workspace:test", "marketing")
        )?.revision
      ).toBe(0);
      const state = windowManagerStore.getSnapshot().context.commandState;
      expect(state.status).toBe("idle");
      expect(state.diagnostic?.message).toBe("Layout changed elsewhere. Refreshed.");
      runtime.stop();
    } finally {
      vi.useRealTimers();
    }
  });

  it("Should reopen the surface by itself after a revision conflict", async () => {
    vi.useFakeTimers();
    try {
      const queryClient = new QueryClient();
      const refreshed = { ...SNAPSHOT, revision: 9 };
      queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), SNAPSHOT);
      queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
      vi.mocked(executeWindowManagerCommand).mockRejectedValueOnce(
        new WindowManagerApiError("Window layout changed.", 409, {
          error: "Window layout changed.",
          code: "revision_conflict",
          workspaceId: "workspace:test",
          currentRevision: 9,
          conflicts: [],
          diagnostics: [
            { code: "revision_conflict", path: null, message: "Window layout changed." },
          ],
        })
      );
      vi.mocked(fetchWindowManagerSnapshot).mockResolvedValue(refreshed);
      const runtime = new WindowManagerRuntime(queryClient);
      runtime.bind({
        workspaceId: "workspace:test",
        profileId: "marketing",
        clientId: "client:web",
      });
      runtime.setClient({
        ...CLIENT_VIEW_DEFAULTS,
        workspaceId: "workspace:test",
        clientId: "client:web",
        presentationRevision: 1,
        activeDesktopId: "desktop:one",
        focusedWindowId: null,
        focusOrder: [],
        connectedAt: "2026-07-22T00:00:00Z",
      });

      runtime.createDesktop();
      await vi.advanceTimersByTimeAsync(0);

      expect(fetchWindowManagerSnapshot).toHaveBeenCalledOnce();
      const state = windowManagerStore.getSnapshot().context.commandState;
      expect(state.status).toBe("idle");
      expect(state.diagnostic?.code).toBe("revision_conflict");
      expect(
        queryClient.getQueryData<WindowManagerSnapshot>(
          windowManagerKeys.snapshot("workspace:test", "marketing")
        )?.revision
      ).toBe(9);

      await vi.advanceTimersByTimeAsync(WINDOW_MANAGER_DIAGNOSTIC_TTL_MS);
      expect(windowManagerStore.getSnapshot().context.commandState.diagnostic).toBeNull();
      runtime.stop();
    } finally {
      vi.useRealTimers();
    }
  });

  it("Should keep the surface conflicted when its recovery refresh fails", async () => {
    vi.useFakeTimers();
    try {
      const queryClient = new QueryClient();
      queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), SNAPSHOT);
      queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
      vi.mocked(executeWindowManagerCommand).mockRejectedValueOnce(
        new WindowManagerApiError("Window layout changed.", 409, {
          error: "Window layout changed.",
          code: "revision_conflict",
          workspaceId: "workspace:test",
          currentRevision: 9,
          conflicts: [],
          diagnostics: [
            { code: "revision_conflict", path: null, message: "Window layout changed." },
          ],
        })
      );
      vi.mocked(fetchWindowManagerSnapshot).mockRejectedValueOnce(new Error("refresh failed"));
      const runtime = new WindowManagerRuntime(queryClient);
      runtime.bind({
        workspaceId: "workspace:test",
        profileId: "marketing",
        clientId: "client:web",
      });
      runtime.setClient({
        ...CLIENT_VIEW_DEFAULTS,
        workspaceId: "workspace:test",
        clientId: "client:web",
        presentationRevision: 1,
        activeDesktopId: "desktop:one",
        focusedWindowId: null,
        focusOrder: [],
        connectedAt: "2026-07-22T00:00:00Z",
      });

      runtime.createDesktop();
      await vi.advanceTimersByTimeAsync(0);

      expect(fetchWindowManagerSnapshot).toHaveBeenCalledOnce();
      expect(windowManagerStore.getSnapshot().context.commandState.status).toBe("conflict");
      runtime.stop();
    } finally {
      vi.useRealTimers();
    }
  });
});

// Invariant: operator close admission freezes exactly the daemon close scope,
// never duplicating termination or expanding to newly joined tabs.
describe("WindowManagerRuntime guarded close", () => {
  function setup() {
    const queryClient = new QueryClient();
    const snapshot = snapshotWithTabPair();
    const key = windowManagerKeys.snapshot("workspace:test", "marketing");
    queryClient.setQueryData(key, snapshot);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.start();
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: "app:agents",
      focusOrder: ["app:agents", "app:tasks"],
      connectedAt: "2026-07-22T00:00:00Z",
    });
    return { runtime, queryClient, key, snapshot };
  }

  it.each([
    ["tab", ["app:tasks"]],
    ["group", ["app:tasks"]],
    ["others", ["app:agents"]],
    ["right", ["app:agents"]],
    ["all", ["app:tasks", "app:agents"]],
  ] as const)("Should confirm only targets in the %s scope", async (scope, ids) => {
    const { runtime } = setup();
    const guard = vi.fn<WindowCloseGuard>(async () => false);
    runtime.setCloseGuard(guard);
    await expect(runtime.closeWindowScoped("app:tasks", scope)).resolves.toBe(false);
    expect(guard).toHaveBeenCalledTimes(1);
    const [targets, isCurrent] = guard.mock.calls[0]!;
    expect(targets.map(target => target.id)).toEqual(ids);
    expect(isCurrent).toEqual(expect.any(Function));
    expect(executeWindowManagerCommand).not.toHaveBeenCalled();
    runtime.stop();
  });

  it("Should not close pinned tabs through Close others", async () => {
    const { runtime, queryClient, key, snapshot } = setup();
    queryClient.setQueryData(key, {
      ...snapshot,
      desktops: snapshotWithTabPair(["app:agents"]).desktops,
    });
    const guard = vi.fn(async () => true);
    runtime.setCloseGuard(guard);
    await expect(runtime.closeWindowScoped("app:tasks", "others")).resolves.toBe(false);
    expect(guard).not.toHaveBeenCalled();
    runtime.stop();
  });

  it("Should suppress double clicks and refuse a close after the target changes", async () => {
    const { runtime, queryClient, key, snapshot } = setup();
    let finish!: (allowed: boolean) => void;
    const guard = vi.fn(
      () =>
        new Promise<boolean>(resolve => {
          finish = resolve;
        })
    );
    runtime.setCloseGuard(guard);
    const pending = runtime.closeWindow("app:tasks");
    await expect(runtime.closeWindow("app:tasks")).resolves.toBe(false);
    queryClient.setQueryData(key, {
      ...snapshot,
      revision: snapshot.revision + 1,
      windows: {
        ...snapshot.windows,
        "app:tasks": { ...snapshot.windows["app:tasks"], instanceKey: "changed" },
      },
    });
    finish(true);
    await expect(pending).resolves.toBe(false);
    expect(guard).toHaveBeenCalledTimes(1);
    expect(executeWindowManagerCommand).not.toHaveBeenCalled();
    runtime.stop();
  });

  it("Should fence the confirmed close to the inspected layout revision", async () => {
    const { runtime, snapshot } = setup();
    runtime.setCloseGuard(async () => true);
    vi.mocked(executeWindowManagerCommand).mockResolvedValue({
      snapshot,
      applied: true,
      changes: EMPTY_CHANGES,
      diagnostics: [],
      client: null,
      deprecations: [],
      rebasedFrom: null,
    });
    await expect(runtime.closeWindowScoped("app:tasks", "group")).resolves.toBe(true);
    expect(executeWindowManagerCommand).toHaveBeenCalledWith(
      "workspace:test",
      "marketing",
      "client:web",
      snapshot.revision,
      expect.objectContaining({
        expectedRevision: snapshot.revision,
        payload: {
          window_id: "app:tasks",
          scope: "group",
        },
      }),
      expect.any(AbortSignal)
    );
    runtime.stop();
  });

  it("Should take a window off the desk without the guard on a view-only close [#1194]", async () => {
    const { runtime, snapshot } = setup();
    const guard = vi.fn<WindowCloseGuard>(async () => true);
    runtime.setCloseGuard(guard);
    vi.mocked(executeWindowManagerCommand).mockResolvedValue({
      snapshot,
      applied: true,
      changes: EMPTY_CHANGES,
      diagnostics: [],
      client: null,
      deprecations: [],
      rebasedFrom: null,
    });
    await expect(runtime.closeWindow("app:tasks", true)).resolves.toBe(true);
    // The guard is what terminates a running terminal; a view-only close never asks it.
    expect(guard).not.toHaveBeenCalled();
    expect(executeWindowManagerCommand).toHaveBeenCalledWith(
      "workspace:test",
      "marketing",
      "client:web",
      snapshot.revision,
      // Fenced: a command that repurposes the window first makes the daemon refuse this close.
      expect.objectContaining({
        expectedRevision: snapshot.revision,
        payload: { window_id: "app:tasks" },
      }),
      expect.any(AbortSignal)
    );
    runtime.stop();
  });

  it("Should refuse a view-only close while an operator close is being confirmed [#1194]", async () => {
    const { runtime } = setup();
    let finish!: (allowed: boolean) => void;
    runtime.setCloseGuard(
      () =>
        new Promise<boolean>(resolve => {
          finish = resolve;
        })
    );
    const pending = runtime.closeWindow("app:agents");
    await expect(runtime.closeWindow("app:tasks", true)).resolves.toBe(false);
    expect(executeWindowManagerCommand).not.toHaveBeenCalled();
    finish(false);
    await pending;
    runtime.stop();
  });
});

describe("WindowManagerRuntime in-window view transitions", () => {
  const listRoute = { pathname: "/agents", search: {} };
  const detailRoute = { pathname: "/agents/release", search: {} };

  function stubViewTransition(reducedMotion = false) {
    const start = vi.fn((arg: (() => void) | { update: () => void }) => {
      if (typeof arg === "function") arg();
      else arg.update();
      const settled = Promise.resolve();
      return { updateCallbackDone: settled, finished: settled, ready: settled };
    });
    Reflect.set(document, "startViewTransition", start);
    vi.stubGlobal(
      "matchMedia",
      vi.fn(() => ({ matches: reducedMotion }) as MediaQueryList)
    );
    return start;
  }

  function focusedAgentsRuntime(options: { focused: boolean; navStack?: OsWindowRoute[] }) {
    const queryClient = new QueryClient();
    const base = snapshotWithAgentsRoute(options.navStack ? detailRoute : listRoute);
    const agents = base.windows["app:agents"];
    const snapshot =
      agents && options.navStack
        ? { ...base, windows: { "app:agents": { ...agents, navStack: options.navStack } } }
        : base;
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), snapshot);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    vi.mocked(executeWindowManagerCommand).mockReturnValue(new Promise(() => undefined));
    const runtime = new WindowManagerRuntime(queryClient);
    runtime.bind({ workspaceId: "workspace:test", profileId: "marketing", clientId: "client:web" });
    runtime.setClient({
      ...CLIENT_VIEW_DEFAULTS,
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: options.focused ? "app:agents" : null,
      focusOrder: ["app:agents"],
      connectedAt: "2026-07-22T00:00:00Z",
    });
    return runtime;
  }

  afterEach(() => {
    Reflect.deleteProperty(document, "startViewTransition");
    vi.unstubAllGlobals();
  });

  it("Should animate a push inside the focused window as a drill-in", () => {
    const start = stubViewTransition();
    const runtime = focusedAgentsRuntime({ focused: true });

    const outcome = runtime.navigateWindow("app:agents", detailRoute, "push");

    expect(outcome.accepted).toBe(true);
    expect(start).toHaveBeenCalledTimes(1);
    expect(runtime.getState().windows["app:agents"]?.route).toEqual(detailRoute);
    runtime.stop();
  });

  it("Should not animate replace navigation or a push into an unfocused window", () => {
    const start = stubViewTransition();
    const focusedRuntime = focusedAgentsRuntime({ focused: true });
    focusedRuntime.navigateWindow("app:agents", detailRoute);
    expect(focusedRuntime.getState().windows["app:agents"]?.route).toEqual(detailRoute);
    focusedRuntime.stop();

    const unfocusedRuntime = focusedAgentsRuntime({ focused: false });
    unfocusedRuntime.navigateWindow("app:agents", detailRoute, "push");
    expect(unfocusedRuntime.getState().windows["app:agents"]?.route).toEqual(detailRoute);
    unfocusedRuntime.stop();

    expect(start).not.toHaveBeenCalled();
  });

  it("Should skip the transition but still navigate when motion is reduced", () => {
    const start = stubViewTransition(true);
    const runtime = focusedAgentsRuntime({ focused: true });

    runtime.navigateWindow("app:agents", detailRoute, "push");

    expect(start).not.toHaveBeenCalled();
    expect(runtime.getState().windows["app:agents"]?.route).toEqual(detailRoute);
    runtime.stop();
  });

  it("Should render a pop optimistically as a drill-out only when the route matches the stack top", () => {
    const start = stubViewTransition();
    const runtime = focusedAgentsRuntime({ focused: true, navStack: [listRoute] });

    runtime.popWindowRoute("app:agents", { pathname: "/agents/other", search: {} });
    expect(start).not.toHaveBeenCalled();
    expect(runtime.getState().windows["app:agents"]?.route).toEqual(detailRoute);

    runtime.popWindowRoute("app:agents", listRoute);
    expect(start).toHaveBeenCalledTimes(1);
    expect(runtime.getState().windows["app:agents"]?.route).toEqual(listRoute);
    runtime.stop();
  });
});
