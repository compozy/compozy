// Suite: window-manager runtime projection bridge
// Invariant: WM-WEB-002 — the shell view resolves this client's active tab per desktop (own
// choice → desktop default → first tab), projects every tab from its weights with real app
// minimums, and marks each window on screen only when its tab is active and no collapsed split
// hides it; the viewport breakpoint, not the dock's work area, picks compact or rejected.
// Boundary IN: authoritative snapshot + effective config + client view.
// Boundary OUT: React rendering and daemon command execution.
import { describe, expect, it } from "vitest";

import {
  buildActiveTabIds,
  buildOsDesktopRuntimeView,
  buildWindowManagerMinimums,
  buildWindowManagerProjections,
  buildWindowManagerWindows,
} from "../window-manager-view";
import type {
  WindowManagerClientView,
  WindowManagerConfig,
  WindowManagerSnapshot,
  WindowManagerWindow,
} from "../window-manager-types";

function managedWindow(
  id: string,
  app: string,
  tabId: string,
  pathname: string
): WindowManagerWindow {
  return {
    id,
    app,
    instanceKey: null,
    route: { pathname, search: {} },
    navStack: [],
    desktopId: "desktop:main",
    tabId,
  };
}

const SNAPSHOT: WindowManagerSnapshot = {
  version: 6,
  workspaceId: "workspace:test",
  revision: 7,
  desktops: [
    {
      id: "desktop:main",
      name: "Main",
      order: 0,
      tabs: [
        {
          id: "tab:pinned",
          pinned: true,
          root: { id: "leaf:usage", kind: "leaf", windowId: "window:usage" },
        },
        {
          id: "tab:split",
          pinned: false,
          root: {
            id: "split:main",
            kind: "split",
            axis: "horizontal",
            weights: [1, 1],
            children: [
              { id: "leaf:dashboard", kind: "leaf", windowId: "window:dashboard" },
              { id: "leaf:tasks", kind: "leaf", windowId: "window:tasks" },
            ],
          },
        },
      ],
      activeTabId: "tab:split",
    },
  ],
  windows: {
    "window:usage": managedWindow("window:usage", "usage", "tab:pinned", "/usage"),
    "window:dashboard": managedWindow("window:dashboard", "dashboard", "tab:split", "/"),
    "window:tasks": managedWindow("window:tasks", "tasks", "tab:split", "/tasks"),
  },
  closedEntryCount: 0,
  overrides: {},
  migration: null,
  updatedAt: "2026-07-22T00:00:00Z",
};

const CLIENT: WindowManagerClientView = {
  workspaceId: "workspace:test",
  clientId: "client:web",
  presentationRevision: 1,
  activeDesktopId: "desktop:main",
  focusedWindowId: "window:tasks",
  focusOrder: ["window:tasks", "window:dashboard"],
  activeTab: {},
  connectedAt: "2026-07-22T00:00:00Z",
};

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

function viewInput(overrides: Partial<Parameters<typeof buildOsDesktopRuntimeView>[0]> = {}) {
  return {
    snapshot: SNAPSHOT,
    globalConfig: CONFIG,
    client: CLIENT,
    viewportWidth: 1440,
    workArea: { x: 0, y: 0, w: 1440, h: 820 },
    workAreaOrigin: { x: 0, y: 52 },
    seamPreview: null,
    routeIntents: {},
    connectionStatus: "connected" as const,
    loadError: null,
    wallpaper: "flat" as const,
    reduceMotion: false,
    ...overrides,
  };
}

describe("window-manager view", () => {
  // Invariant: changing dock tracks cannot flip the viewport's presentation policy.
  it.each([939, 1000])("Should stay split at a 1000px viewport with work area %s", width => {
    const input = viewInput({ viewportWidth: 1000, workArea: { x: 0, y: 0, w: width, h: 700 } });

    expect(buildOsDesktopRuntimeView(input).presentation).toBe("split");
    expect(buildOsDesktopRuntimeView({ ...input, viewportWidth: 900 }).presentation).toBe(
      "compact"
    );
  });

  it("Should reject a compact viewport under the reject policy", () => {
    const view = buildOsDesktopRuntimeView(
      viewInput({ viewportWidth: 900, globalConfig: { ...CONFIG, smallViewportPolicy: "reject" } })
    );

    expect(view.viewportState).toBe("rejected");
    expect(view.presentation).toBe("split");
  });

  it("Should derive each window minimum from the app catalog", () => {
    expect(
      buildWindowManagerMinimums({
        windows: {
          "window:dashboard": { id: "window:dashboard", app: "dashboard" },
          "window:tasks": { id: "window:tasks", app: "tasks" },
        },
      })
    ).toEqual({
      "window:dashboard": { width: 220, height: 220 },
      "window:tasks": { width: 220, height: 220 },
    });
  });

  it("Should resolve the client's tab, then the desktop default, then the first tab", () => {
    expect(
      buildActiveTabIds(SNAPSHOT, { ...CLIENT, activeTab: { "desktop:main": "tab:pinned" } })
    ).toEqual({ "desktop:main": "tab:pinned" });
    expect(
      buildActiveTabIds(SNAPSHOT, { ...CLIENT, activeTab: { "desktop:main": "tab:closed" } })
    ).toEqual({ "desktop:main": "tab:split" });
    const staleDefault = {
      ...SNAPSHOT,
      desktops: SNAPSHOT.desktops.map(desktop => ({ ...desktop, activeTabId: "tab:closed" })),
    };
    expect(buildActiveTabIds(staleDefault, null)).toEqual({ "desktop:main": "tab:pinned" });
  });

  it("Should mark windows shown only on the active tab and behind a collapsed split", () => {
    const workArea = { x: 0, y: 0, w: 400, h: 400 };
    const projections = buildWindowManagerProjections({
      snapshot: SNAPSHOT,
      client: CLIENT,
      workArea,
      config: CONFIG,
      compact: false,
    });
    const windows = buildWindowManagerWindows({
      snapshot: SNAPSHOT,
      activeTabIds: buildActiveTabIds(SNAPSHOT, CLIENT),
      projections,
    });

    expect(projections["tab:split"]?.collapsed).toEqual([
      expect.objectContaining({
        splitId: "split:main",
        windowIds: ["window:dashboard", "window:tasks"],
        shownWindowId: "window:tasks",
      }),
    ]);
    expect(windows["window:tasks"]).toMatchObject({
      tabId: "tab:split",
      nodeId: "leaf:tasks",
      tabActive: true,
      inSplit: true,
      pinned: false,
      shown: true,
    });
    expect(windows["window:dashboard"]).toMatchObject({ tabActive: true, shown: false });
    expect(windows["window:usage"]).toMatchObject({
      pinned: true,
      inSplit: false,
      tabActive: false,
      shown: false,
    });
    const root = SNAPSHOT.desktops[0]?.tabs[1]?.root;
    expect(root?.kind === "split" ? root.weights : null).toEqual([1, 1]);
  });

  it("Should show every member of a split that fits", () => {
    const view = buildOsDesktopRuntimeView(viewInput());

    expect(view.activeTabIds).toEqual({ "desktop:main": "tab:split" });
    expect(view.projections["tab:split"]?.collapsed).toEqual([]);
    expect(view.windows["window:dashboard"]?.shown).toBe(true);
    expect(view.windows["window:tasks"]?.shown).toBe(true);
    expect(view.hydration).toBe("live");
  });

  // Invariant: a seam drag previews in the rendered tab and its projection alike, while the
  // authoritative snapshot keeps the committed weights until `layout.resize` lands.
  it("Should re-project and render a split against a live seam preview", () => {
    const view = buildOsDesktopRuntimeView(
      viewInput({ seamPreview: { splitId: "split:main", boundaryIndex: 0, deltaPx: 100 } })
    );
    const [dashboard, tasks] = view.projections["tab:split"]?.windows ?? [];

    expect(dashboard?.rect.w).toBeGreaterThan(tasks?.rect.w ?? Number.POSITIVE_INFINITY);
    const rendered = view.desktops[0]?.tabs[1]?.root;
    expect(rendered?.kind === "split" && rendered.weights[0]).toBeGreaterThan(
      rendered?.kind === "split" ? (rendered.weights[1] ?? 1) : 1
    );
    expect(view.snapshot?.desktops[0]?.tabs[1]?.root).toBe(SNAPSHOT.desktops[0]?.tabs[1]?.root);
  });

  it("Should stay pending without a snapshot and degrade on a load error", () => {
    const pending = buildOsDesktopRuntimeView(viewInput({ snapshot: null }));
    expect(pending.hydration).toBe("pending");
    expect(pending.windows).toEqual({});

    expect(buildOsDesktopRuntimeView(viewInput({ loadError: new Error("boom") })).hydration).toBe(
      "degraded"
    );
  });
});
