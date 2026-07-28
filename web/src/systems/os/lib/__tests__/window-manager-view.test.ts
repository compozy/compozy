// Suite: window-manager runtime projection bridge
// Invariant: WM-WEB-002 — rendered frame minima reach the pure projector, so cramped splits
// adapt to a stack without changing daemon topology.
// Boundary IN: authoritative snapshot + effective config + client view.
// Boundary OUT: React rendering and daemon command execution.
import { describe, expect, it } from "vitest";

import {
  buildWindowManagerMinimums,
  buildWindowManagerProjections,
  buildWindowManagerWindows,
} from "../window-manager-view";
import type { WindowManagerConfig, WindowManagerSnapshot } from "../window-manager-types";

const SNAPSHOT: WindowManagerSnapshot = {
  version: 1,
  workspaceId: "workspace:test",
  revision: 7,
  desktops: [
    {
      id: "desktop:main",
      name: "Main",
      order: 0,
      purpose: "standard",
      focusOwner: null,
      groups: [
        {
          id: "group:main",
          frame: { x: 0, y: 0, w: 1, h: 1 },
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
      floating: [],
    },
  ],
  windows: {
    "window:dashboard": {
      id: "window:dashboard",
      app: "dashboard",
      instanceKey: null,
      route: { pathname: "/", search: {} },
      placement: "tiled",
      desktopId: "desktop:main",
      floatingRect: { x: 0.1, y: 0.1, w: 0.5, h: 0.5 },
      minimized: false,
      returnAnchor: null,
    },
    "window:tasks": {
      id: "window:tasks",
      app: "tasks",
      instanceKey: null,
      route: { pathname: "/tasks", search: {} },
      placement: "tiled",
      desktopId: "desktop:main",
      floatingRect: { x: 0.2, y: 0.2, w: 0.5, h: 0.5 },
      minimized: false,
      returnAnchor: null,
    },
  },
  history: { undo: [], redo: [] },
  overrides: {},
  updatedAt: "2026-07-22T00:00:00Z",
};

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
};

describe("window-manager view", () => {
  it("Should derive each window minimum from the same frame contract", () => {
    expect(buildWindowManagerMinimums(SNAPSHOT)).toEqual({
      "window:dashboard": { width: 280, height: 180 },
      "window:tasks": { width: 280, height: 180 },
    });
  });

  it("Should adapt a real cramped desktop projection to a temporary stack", () => {
    const projections = buildWindowManagerProjections(
      SNAPSHOT,
      {
        workspaceId: "workspace:test",
        clientId: "client:web",
        activeDesktopId: "desktop:main",
        focusedWindowId: "window:tasks",
        focusOrder: ["window:dashboard", "window:tasks"],
        connectedAt: "2026-07-22T00:00:00Z",
        presentationRevision: 1,
      },
      { x: 0, y: 0, w: 540, h: 400 },
      CONFIG
    );

    expect(projections["desktop:main"]?.stacks).toEqual([
      expect.objectContaining({
        kind: "adaptive",
        windowIds: ["window:dashboard", "window:tasks"],
        activeWindowId: "window:tasks",
      }),
    ]);
    const windows = buildWindowManagerWindows({
      snapshot: SNAPSHOT,
      client: null,
      workArea: { x: 0, y: 0, w: 540, h: 400 },
      projections,
      raiseOnFocus: false,
    });
    expect(windows["window:dashboard"]?.parentAxis).toBe("horizontal");
    expect(windows["window:tasks"]?.parentAxis).toBe("horizontal");
    expect(SNAPSHOT.desktops[0]?.groups[0]?.root.kind).toBe("split");
  });

  it("Should project the most recently focused window above every stable peer", () => {
    const floatingSnapshot: WindowManagerSnapshot = {
      ...SNAPSHOT,
      desktops: SNAPSHOT.desktops.map(desktop => ({
        ...desktop,
        floating: ["window:tasks", "window:dashboard"],
      })),
      windows: Object.fromEntries(
        Object.entries(SNAPSHOT.windows).map(([id, window]) => [
          id,
          { ...window, placement: "floating" as const },
        ])
      ),
    };
    const client = {
      workspaceId: "workspace:test",
      clientId: "client:web",
      activeDesktopId: "desktop:main",
      focusedWindowId: "window:tasks",
      focusOrder: ["window:tasks", "window:dashboard"],
      connectedAt: "2026-07-22T00:00:00Z",
      presentationRevision: 1,
    };

    const raised = buildWindowManagerWindows({
      snapshot: floatingSnapshot,
      client,
      workArea: { x: 0, y: 0, w: 1440, h: 820 },
      projections: {},
      raiseOnFocus: true,
    });
    const stable = buildWindowManagerWindows({
      snapshot: floatingSnapshot,
      client,
      workArea: { x: 0, y: 0, w: 1440, h: 820 },
      projections: {},
      raiseOnFocus: false,
    });

    expect(raised["window:tasks"]?.layer).toBeGreaterThan(
      raised["window:dashboard"]?.layer ?? Number.MAX_SAFE_INTEGER
    );
    expect(stable["window:tasks"]?.layer).toBeLessThan(
      stable["window:dashboard"]?.layer ?? Number.MIN_SAFE_INTEGER
    );
  });
});
