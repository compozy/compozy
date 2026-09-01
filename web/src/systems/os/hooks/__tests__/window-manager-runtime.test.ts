// Suite: window-manager runtime projection and command admission
// Invariant: cache data changes drive projections while observer lifecycle stays render-pure;
// accepted presentation intents persist until the serialized daemon command settles.
// Owning layer: the Query cache → runtime projection and runtime → command boundaries.
import { QueryClient, QueryObserver } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  executeWindowManagerCommand,
  fetchWindowManagerSnapshot,
  WindowManagerApiError,
} from "../../adapters/window-manager-api";
import type { OsWindowRoute } from "../../lib/os-types";
import { createTileSnapTarget } from "../../lib/snap-targets";
import { windowManagerKeys } from "../../lib/window-manager-query";
import type { WindowManagerSettingsSection } from "../../lib/window-manager-settings-section";
import type {
  WindowManagerChangeSet,
  WindowManagerConfig,
  WindowManagerSnapshot,
} from "../../lib/window-manager-types";
import {
  selectDesktopTransitionIntent,
  windowManagerStore,
} from "../../stores/window-manager-store";
import { beginWindowManagerCommand } from "../../stores/window-manager-store-commands";
import { WindowManagerRuntime } from "../../runtime/window-manager-runtime";
import { WINDOW_MANAGER_DIAGNOSTIC_TTL_MS } from "../../runtime/window-manager-runtime-core";

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
  newWindowPolicy: "floating",
  smallViewportPolicy: "stack",
  focusPolicy: "click_directional",
  focusWrap: true,
  focusFollowsPointer: false,
  raiseOnFocus: true,
  dragAwayPolicy: "window",
  groupMoveModifier: "alt",
  swapModifier: "shift",
  historyLimit: 50,
  navStackLimit: 50,
  closedEntryLimit: 20,
  desktopTransition: "slide",
  gaps: { inner: 8, top: 0, right: 0, bottom: 0, left: 0 },
  snap: {
    edgeBand: 32,
    cornerReach: 150,
    exitSlack: 16,
    repeatRatios: [0.5, 0.666667, 0.333333],
  },
  bindings: { topCenter: "zoom", bottomCenter: "reserved" },
  shortcuts: {},
  globalShortcuts: {},
  shortcutDefaults: {},
  effectiveShortcuts: {},
};

const TEST_CONFIG_KEY = windowManagerKeys.config("workspace:test", "client:web");
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
  version: 4,
  workspaceId: "workspace:test",
  revision: 7,
  desktops: [
    {
      id: "desktop:one",
      name: "One",
      order: 0,
      groups: [],
      floating: [],
      floatingStacks: [],
    },
    {
      id: "desktop:two",
      name: "Two",
      order: 1,
      groups: [],
      floating: [],
      floatingStacks: [],
    },
  ],
  windows: {},
  closedEntryCount: 0,
  overrides: {},
  updatedAt: "2026-07-22T00:00:00Z",
};

const CLIENT_VIEW_DEFAULTS = { stackActive: {} } as const;
const EMPTY_CHANGES: WindowManagerChangeSet = {
  desktopIds: [],
  windowIds: [],
  groupIds: [],
  nodeIds: [],
  clientIds: [],
  stackGrouped: [],
  stackUngrouped: [],
};

/** Two stacked members on desktop:one whose durable active tab is the second member. */
function snapshotWithFloatingStack(): WindowManagerSnapshot {
  const member = {
    instanceKey: null,
    route: { pathname: "/tasks", search: {} },
    navStack: [],
    pinned: false,
    placement: "stacked" as const,
    desktopId: "desktop:one",
    floatingRect: { x: 0.1, y: 0.1, w: 0.5, h: 0.5 },
    minimized: false,
    zoomed: false,
    returnAnchor: null,
  };
  return {
    ...SNAPSHOT,
    desktops: SNAPSHOT.desktops.map(desktop =>
      desktop.id === "desktop:one"
        ? {
            ...desktop,
            floatingStacks: [
              {
                id: "stack:pair",
                windowIds: ["app:tasks", "app:agents"],
                activeId: "app:agents",
                rect: { x: 0.1, y: 0.1, w: 0.5, h: 0.5 },
                minimized: false,
              },
            ],
          }
        : desktop
    ),
    windows: {
      "app:tasks": { ...member, id: "app:tasks", app: "tasks" },
      "app:agents": { ...member, id: "app:agents", app: "agents" },
    },
  };
}

function snapshotWithAgentsRoute(
  route: OsWindowRoute,
  revision = SNAPSHOT.revision
): WindowManagerSnapshot {
  return {
    ...SNAPSHOT,
    revision,
    desktops: SNAPSHOT.desktops.map(desktop =>
      desktop.id === "desktop:one" ? { ...desktop, floating: ["app:agents"] } : desktop
    ),
    windows: {
      "app:agents": {
        id: "app:agents",
        app: "agents",
        instanceKey: null,
        route,
        navStack: [],
        pinned: false,
        placement: "floating",
        desktopId: "desktop:one",
        floatingRect: { x: 0.1, y: 0.1, w: 0.5, h: 0.5 },
        minimized: false,
        zoomed: false,
        returnAnchor: null,
      },
    },
  };
}

afterEach(() => {
  windowManagerStore.trigger.bindingUnbound();
  windowManagerStore.trigger.workAreaMeasured({ workArea: null });
  vi.clearAllMocks();
});

describe("WindowManagerRuntime", () => {
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

  it("Should keep gesture samples off the runtime projection channel", () => {
    const runtime = new WindowManagerRuntime(new QueryClient());
    runtime.start();
    const onProjection = vi.fn();
    const unsubscribe = runtime.subscribe(onProjection);
    windowManagerStore.trigger.gestureBegan({
      pointerId: 4,
      point: { x: 640, y: 90 },
      workArea: { x: 0, y: 0, w: 1280, h: 800 },
      layoutRevision: 7,
      source: {
        windowId: "window:tasks",
        nodeId: "node:tasks",
        groupId: "group:primary",
        moveMode: "window",
      },
    });
    windowManagerStore.trigger.gesturePreviewed({
      point: { x: 320, y: 100 },
      preview: null,
      currentWorkArea: { x: 0, y: 0, w: 1280, h: 800 },
    });
    windowManagerStore.trigger.gesturePreviewed({
      point: { x: 12, y: 120 },
      preview: null,
      currentWorkArea: { x: 0, y: 0, w: 1280, h: 800 },
    });

    expect(onProjection).not.toHaveBeenCalled();

    windowManagerStore.trigger.connectionStatusChanged({ status: "connecting" });

    expect(onProjection).toHaveBeenCalledOnce();
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

  it("Should dispatch frame resizes as wire rects for windows and islands", async () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), SNAPSHOT);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    vi.mocked(executeWindowManagerCommand).mockResolvedValue({
      snapshot: SNAPSHOT,
      applied: true,
      changes: EMPTY_CHANGES,
      diagnostics: [],
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

    const windowResize = runtime.resizeWindowFrame("window:solo", { x: 0, y: 0, w: 0.5, h: 0.6 });
    const frameResize = runtime.resizeGroupFrames("desktop:one", [
      { groupId: "group:left", frame: { x: 0, y: 0, w: 0.6, h: 1 } },
      { groupId: "group:right", frame: { x: 0.6, y: 0, w: 0.4, h: 1 } },
    ]);
    expect(windowResize.accepted).toBe(true);
    expect(frameResize.accepted).toBe(true);
    await Promise.all([windowResize.completion, frameResize.completion]);

    const bodies = vi.mocked(executeWindowManagerCommand).mock.calls.map(call => call[4]);
    expect(bodies[0]).toMatchObject({
      commandId: "window.resize",
      payload: {
        window_id: "window:solo",
        frame: { x: 0, y: 0, width: 0.5, height: 0.6 },
      },
    });
    expect(bodies[1]).toMatchObject({
      commandId: "layout.frame_resize",
      payload: {
        desktop_id: "desktop:one",
        edits: [
          { group_id: "group:left", frame: { x: 0, y: 0, width: 0.6, height: 1 } },
          { group_id: "group:right", frame: { x: 0.6, y: 0, width: 0.4, height: 1 } },
        ],
      },
    });
    expect(runtime.resizeGroupFrames("desktop:one", []).accepted).toBe(false);
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
      rebasedFrom: null,
    });

    await expect(second.completion).resolves.toBe(true);
    expect(routeWhileQueued).toEqual(opsRoute);
    expect(routeAfterIntermediateResult).toEqual(opsRoute);
    expect(runtime.getState().windows["app:agents"]?.route).toEqual(opsRoute);
    runtime.stop();
  });

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

    // A zoom or cross-desktop focus arrives as a reconciled client frame with
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

  it("[UT-040] Should open a distinct opaque Tasks instance when the caller explicitly requests one", async () => {
    const existingId = "w-existing-tasks-instance";
    const snapshot: WindowManagerSnapshot = {
      ...SNAPSHOT,
      desktops: SNAPSHOT.desktops.map(desktop =>
        desktop.id === "desktop:one" ? { ...desktop, floating: [existingId] } : desktop
      ),
      windows: {
        [existingId]: {
          id: existingId,
          app: "tasks",
          instanceKey: null,
          route: { pathname: "/tasks", search: {} },
          navStack: [],
          pinned: false,
          placement: "floating",
          desktopId: "desktop:one",
          floatingRect: { x: 0.1, y: 0.1, w: 0.5, h: 0.5 },
          minimized: false,
          zoomed: false,
          returnAnchor: null,
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
      })
    );
    const command = vi.mocked(executeWindowManagerCommand).mock.calls[0]?.[4];
    if (command === undefined) {
      throw new Error("Expected the forced instance request to dispatch a window command.");
    }
    const openedId = (command.payload as { window: { id: string } }).window.id;
    expect(openedId).not.toBe(existingId);
    runtime.stop();
  });

  it("[UT-041] Should keep another Tasks instance's route intent after closing its sibling", async () => {
    const firstId = "w-tasks-first";
    const secondId = "w-tasks-second";
    const secondRoute = { pathname: "/tasks", search: { view: "board", owner: "me" } };
    const snapshot: WindowManagerSnapshot = {
      ...SNAPSHOT,
      desktops: SNAPSHOT.desktops.map(desktop =>
        desktop.id === "desktop:one" ? { ...desktop, floating: [firstId, secondId] } : desktop
      ),
      windows: {
        [firstId]: {
          id: firstId,
          app: "tasks",
          instanceKey: null,
          route: { pathname: "/tasks", search: { view: "list" } },
          navStack: [],
          pinned: false,
          placement: "floating",
          desktopId: "desktop:one",
          floatingRect: { x: 0.1, y: 0.1, w: 0.5, h: 0.5 },
          minimized: false,
          zoomed: false,
          returnAnchor: null,
        },
        [secondId]: {
          id: secondId,
          app: "tasks",
          instanceKey: null,
          route: { pathname: "/tasks", search: { view: "calendar" } },
          navStack: [],
          pinned: false,
          placement: "floating",
          desktopId: "desktop:one",
          floatingRect: { x: 0.2, y: 0.2, w: 0.5, h: 0.5 },
          minimized: false,
          zoomed: false,
          returnAnchor: null,
        },
      },
    };
    const closedSnapshot: WindowManagerSnapshot = {
      ...snapshot,
      revision: snapshot.revision + 1,
      desktops: snapshot.desktops.map(desktop =>
        desktop.id === "desktop:one" ? { ...desktop, floating: [secondId] } : desktop
      ),
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
      desktops: SNAPSHOT.desktops.map(desktop =>
        desktop.id === "desktop:one" ? { ...desktop, floating: ["app:tasks"] } : desktop
      ),
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
          pinned: false,
          placement: "floating",
          desktopId: "desktop:one",
          floatingRect: { x: 0.1, y: 0.1, w: 0.5, h: 0.5 },
          minimized: false,
          zoomed: false,
          returnAnchor: null,
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
      }
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
      desktops: agents.desktops.map(desktop =>
        desktop.id === "desktop:one" ? { ...desktop, floating: ["app:agents", sessionId] } : desktop
      ),
      windows: {
        ...agents.windows,
        [sessionId]: {
          ...agents.windows["app:agents"]!,
          id: sessionId,
          app: "session",
          instanceKey: "sess-new",
          route: sessionRoute,
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
      pinned: false,
      id: "app:marketplace",
      app: "marketplace",
      instanceKey: null,
      route: { pathname: "/marketplace/skills", search: {} },
      placement: "floating" as const,
      desktopId: "desktop:one",
      floatingRect: { x: 0.1, y: 0.1, w: 0.5, h: 0.5 },
      minimized: false,
      zoomed: false,
      returnAnchor: null,
    };
    const session = {
      ...marketplace,
      id: "session:sess-concurrent",
      app: "session",
      instanceKey: "sess-concurrent",
      route: { pathname: "/agents/general/sessions/sess-concurrent", search: {} },
    };
    const initial = {
      ...SNAPSHOT,
      desktops: SNAPSHOT.desktops.map(desktop =>
        desktop.id === "desktop:one" ? { ...desktop, floating: [marketplace.id] } : desktop
      ),
      windows: { [marketplace.id]: marketplace },
    };
    const refreshed = {
      ...initial,
      revision: 8,
      desktops: initial.desktops.map(desktop =>
        desktop.id === "desktop:one"
          ? { ...desktop, floating: [marketplace.id, session.id] }
          : desktop
      ),
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

    expect(outcome.accepted).toBe(true);
    await expect(outcome.completion).resolves.toBe(true);
    expect(fetchWindowManagerSnapshot).toHaveBeenCalledOnce();
    expect(executeWindowManagerCommand).toHaveBeenNthCalledWith(
      1,
      "workspace:test",
      "marketing",
      "client:web",
      7,
      expect.objectContaining({ commandId: "window.open" })
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
      }
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

    await expect(outcome.completion).resolves.toBe(false);
    expect(fetchWindowManagerSnapshot).not.toHaveBeenCalled();
    expect(windowManagerStore.getSnapshot().context.commandState).toMatchObject({
      status: "conflict",
      conflict: { commandId: "layout:stale" },
    });
    runtime.stop();
  });

  it("Should normalize a tile command against the same gap-inset area used by its preview", async () => {
    const queryClient = new QueryClient();
    const snapshot: WindowManagerSnapshot = {
      ...SNAPSHOT,
      desktops: SNAPSHOT.desktops.map(desktop =>
        desktop.id === "desktop:one" ? { ...desktop, floating: ["app:tasks"] } : desktop
      ),
      windows: {
        "app:tasks": {
          id: "app:tasks",
          app: "tasks",
          instanceKey: null,
          route: { pathname: "/tasks", search: {} },
          navStack: [],
          pinned: false,
          placement: "floating",
          desktopId: "desktop:one",
          floatingRect: { x: 0.1, y: 0.1, w: 0.5, h: 0.5 },
          minimized: false,
          zoomed: false,
          returnAnchor: null,
        },
      },
    };
    const config: WindowManagerConfig = {
      ...CONFIG,
      gaps: { inner: 8, top: 7, right: 10, bottom: 13, left: 10 },
    };
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), snapshot);
    queryClient.setQueryData(TEST_CONFIG_KEY, { ...SETTINGS_SECTION, config });
    windowManagerStore.trigger.workAreaMeasured({
      workArea: {
        rect: { x: 10, y: 20, w: 1200, h: 800 },
        origin: { x: 0, y: 0 },
      },
    });
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
      focusedWindowId: "app:tasks",
      focusOrder: ["app:tasks"],
      connectedAt: "2026-07-22T00:00:00Z",
    });

    runtime.tileWindow("app:tasks", "left");
    await vi.waitFor(() => expect(executeWindowManagerCommand).toHaveBeenCalled());

    expect(executeWindowManagerCommand).toHaveBeenCalledWith(
      "workspace:test",
      "marketing",
      "client:web",
      7,
      expect.objectContaining({
        commandId: "layout.arrange",
        payload: expect.objectContaining({
          frame: { x: 0, y: 0, width: 0.5, height: 1 },
        }),
      })
    );
    runtime.stop();
  });

  it("Should return the exact completion outcomes for gesture commands", async () => {
    const queryClient = new QueryClient();
    const snapshot = snapshotWithAgentsRoute({ pathname: "/agents", search: {} });
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), snapshot);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    vi.mocked(executeWindowManagerCommand).mockResolvedValue({
      snapshot: { ...snapshot, revision: 8 },
      applied: false,
      changes: {
        ...EMPTY_CHANGES,
      },
      diagnostics: [],
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

    const outcome = runtime.applySnapTarget(
      "app:agents",
      createTileSnapTarget({ x: 0, y: 0, w: 1280, h: 800 }, "left", [0.5])
    );

    expect(outcome.accepted).toBe(true);
    await expect(outcome.completion).resolves.toBe(false);
    const floating = runtime.commitFloatingRect("app:agents", { x: 120, y: 90, w: 640, h: 480 });
    expect(floating.accepted).toBe(true);
    await expect(floating.completion).resolves.toBe(false);
    expect(executeWindowManagerCommand).toHaveBeenCalledTimes(2);
    runtime.stop();
  });

  it("Should arrange a floating tab frame as one tiled stack and reactivate its active tab", async () => {
    const queryClient = new QueryClient();
    const snapshot = snapshotWithFloatingStack();
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), snapshot);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    windowManagerStore.trigger.workAreaMeasured({
      workArea: { rect: { x: 0, y: 0, w: 1280, h: 800 }, origin: { x: 0, y: 0 } },
    });
    vi.mocked(executeWindowManagerCommand).mockResolvedValue({
      snapshot: { ...snapshot, revision: 8 },
      applied: true,
      changes: { ...EMPTY_CHANGES },
      diagnostics: [],
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

    const outcome = runtime.applySnapTarget(
      "app:agents",
      createTileSnapTarget({ x: 0, y: 0, w: 1280, h: 800 }, "left", [0.5]),
      true
    );

    expect(outcome.accepted).toBe(true);
    await expect(outcome.completion).resolves.toBe(true);
    await vi.waitFor(() => expect(executeWindowManagerCommand).toHaveBeenCalledTimes(2));
    expect(vi.mocked(executeWindowManagerCommand).mock.calls[0]?.[4]).toMatchObject({
      commandId: "layout.arrange",
      payload: { window_ids: ["app:tasks", "app:agents"], arrangement: "stack" },
    });
    expect(vi.mocked(executeWindowManagerCommand).mock.calls[1]?.[4]).toMatchObject({
      commandId: "window.stack.set_active",
      payload: { window_id: "app:agents" },
      rebase: { windowId: "app:agents" },
    });
    runtime.stop();
  });

  it("Should splice a group move on a split target as one window.move with move_group", async () => {
    const queryClient = new QueryClient();
    const stackedSnapshot = snapshotWithFloatingStack();
    const sourceWindow = stackedSnapshot.windows["app:tasks"]!;
    const snapshot: WindowManagerSnapshot = {
      ...stackedSnapshot,
      desktops: stackedSnapshot.desktops.map(desktop =>
        desktop.id === "desktop:one"
          ? {
              ...desktop,
              groups: [
                {
                  id: "group:board",
                  frame: { x: 0, y: 0, w: 0.5, h: 1 },
                  root: { id: "node:board", kind: "leaf", windowId: "app:board" },
                },
              ],
            }
          : desktop
      ),
      windows: {
        ...stackedSnapshot.windows,
        "app:board": {
          ...sourceWindow,
          id: "app:board",
          app: "board",
          route: { pathname: "/board", search: {} },
          placement: "tiled",
        },
      },
    };
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), snapshot);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    windowManagerStore.trigger.workAreaMeasured({
      workArea: { rect: { x: 0, y: 0, w: 1280, h: 800 }, origin: { x: 0, y: 0 } },
    });
    vi.mocked(executeWindowManagerCommand).mockResolvedValue({
      snapshot: { ...snapshot, revision: 8 },
      applied: true,
      changes: { ...EMPTY_CHANGES },
      diagnostics: [],
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

    const outcome = runtime.applySnapTarget(
      "app:agents",
      {
        kind: "split",
        id: "split:node:board:right",
        targetWindowId: "app:board",
        targetNodeId: "node:board",
        side: "right",
        axis: "horizontal",
        rect: { x: 200, y: 0, w: 200, h: 300 },
      },
      true
    );

    expect(outcome.accepted).toBe(true);
    await expect(outcome.completion).resolves.toBe(true);
    expect(executeWindowManagerCommand).toHaveBeenCalledOnce();
    expect(vi.mocked(executeWindowManagerCommand).mock.calls[0]?.[4]).toMatchObject({
      commandId: "window.move",
      payload: {
        window_id: "app:agents",
        target_window_id: "app:board",
        placement: "right",
        move_group: true,
      },
    });
    runtime.stop();
  });

  it("Should reject snap application without dispatch when its window is unknown", async () => {
    const runtime = new WindowManagerRuntime(new QueryClient());

    const outcome = runtime.applySnapTarget(
      "app:missing",
      createTileSnapTarget({ x: 0, y: 0, w: 1280, h: 800 }, "left", [0.5])
    );

    expect(outcome.accepted).toBe(false);
    await expect(outcome.completion).resolves.toBe(false);
    const floating = runtime.commitFloatingRect("app:missing", { x: 120, y: 90, w: 640, h: 480 });
    expect(floating.accepted).toBe(false);
    await expect(floating.completion).resolves.toBe(false);
    expect(executeWindowManagerCommand).not.toHaveBeenCalled();
    runtime.stop();
  });

  it("Should carry a rebase proof when a stale gesture lands on a tile zone", async () => {
    const queryClient = new QueryClient();
    const snapshot = snapshotWithAgentsRoute({ pathname: "/agents", search: {} }, 9);
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), snapshot);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    windowManagerStore.trigger.workAreaMeasured({
      workArea: { rect: { x: 0, y: 0, w: 1280, h: 800 }, origin: { x: 0, y: 0 } },
    });
    vi.mocked(executeWindowManagerCommand).mockResolvedValue({
      snapshot: { ...snapshot, revision: 10 },
      applied: true,
      changes: { ...EMPTY_CHANGES },
      diagnostics: [],
      client: null,
      rebasedFrom: 7,
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

    const outcome = runtime.applySnapTarget(
      "app:agents",
      createTileSnapTarget({ x: 0, y: 0, w: 1280, h: 800 }, "left", [0.5]),
      false,
      { expectedRevision: 7, sourceNodeId: null }
    );

    await expect(outcome.completion).resolves.toBe(true);
    expect(executeWindowManagerCommand).toHaveBeenCalledWith(
      "workspace:test",
      "marketing",
      "client:web",
      7,
      expect.objectContaining({
        commandId: "layout.arrange",
        expectedRevision: 7,
        rebase: { windowId: "app:agents" },
      })
    );
    runtime.stop();
  });

  it("Should unzoom a covering frame before focusing a tiled window hidden under it", async () => {
    const queryClient = new QueryClient();
    const horizontal = "horizontal" as const;
    const snapshot: WindowManagerSnapshot = {
      ...SNAPSHOT,
      desktops: SNAPSHOT.desktops.map(desktop =>
        desktop.id === "desktop:one"
          ? {
              ...desktop,
              groups: [
                {
                  id: "group:one",
                  frame: { x: 0, y: 0, w: 1, h: 1 },
                  root: {
                    id: "split:one",
                    kind: "split",
                    axis: horizontal,
                    weights: [0.5, 0.5],
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
        "app:tasks": {
          id: "app:tasks",
          app: "tasks",
          instanceKey: null,
          route: { pathname: "/tasks", search: {} },
          navStack: [],
          pinned: false,
          placement: "tiled",
          desktopId: "desktop:one",
          floatingRect: { x: 0.1, y: 0.1, w: 0.5, h: 0.5 },
          minimized: false,
          zoomed: true,
          returnAnchor: null,
        },
        "app:agents": {
          id: "app:agents",
          app: "agents",
          instanceKey: null,
          route: { pathname: "/agents", search: {} },
          navStack: [],
          pinned: false,
          placement: "tiled",
          desktopId: "desktop:one",
          floatingRect: { x: 0.2, y: 0.2, w: 0.5, h: 0.5 },
          minimized: false,
          zoomed: false,
          returnAnchor: null,
        },
      },
    };
    queryClient.setQueryData(windowManagerKeys.snapshot("workspace:test", "marketing"), snapshot);
    queryClient.setQueryData(TEST_CONFIG_KEY, SETTINGS_SECTION);
    windowManagerStore.trigger.workAreaMeasured({
      workArea: { rect: { x: 0, y: 0, w: 1280, h: 800 }, origin: { x: 0, y: 0 } },
    });
    vi.mocked(executeWindowManagerCommand).mockResolvedValue({
      snapshot: { ...snapshot, revision: 8 },
      applied: true,
      changes: { ...EMPTY_CHANGES },
      diagnostics: [],
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

    const outcome = runtime.focusWindow("app:agents");
    await expect(outcome.completion).resolves.toBe(true);

    expect(executeWindowManagerCommand).toHaveBeenNthCalledWith(
      1,
      "workspace:test",
      "marketing",
      "client:web",
      7,
      expect.objectContaining({ commandId: "window.zoom", payload: { window_id: "app:tasks" } })
    );
    expect(executeWindowManagerCommand).toHaveBeenNthCalledWith(
      2,
      "workspace:test",
      "marketing",
      "client:web",
      expect.any(Number),
      expect.objectContaining({ commandId: "window.focus" })
    );
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
});
