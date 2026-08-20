// Suite: live palette client context
// Invariant: chrome publishes the current scope, focused session, destination
// route, and a fail-closed trust echo instead of the registration snapshot.
// Owning layer: unit — the chrome context helper.
import { describe, expect, it } from "vitest";

import {
  resolveLivePaletteClientContext,
  selectPaletteDestinationRoute,
} from "../desktop-chrome-client-context";
import type { OsDesktopRuntimeStore, OsWindow } from "../os-types";

describe("resolveLivePaletteClientContext", () => {
  it("Should publish live shell fields and fail closed when trust is unknown", () => {
    expect(
      resolveLivePaletteClientContext({
        scope: "global",
        focusedSessionState: "waiting-for-input",
        registeredWorkspaceTrusted: undefined,
        destinationRoute: { pathname: "/tasks", search: { query: "review" } },
        globalShortcuts: [],
      })
    ).toEqual({
      scopeGlobal: true,
      focusedSessionState: "waiting-for-input",
      workspaceTrusted: false,
      destinationIntent: { pathname: "/tasks", search: { query: "review" } },
      globalShortcuts: [],
    });
  });

  it("Should echo the registered trust bit when the daemon already supplied one", () => {
    expect(
      resolveLivePaletteClientContext({
        scope: "workspace",
        focusedSessionState: null,
        registeredWorkspaceTrusted: true,
        destinationRoute: null,
        globalShortcuts: [],
      }).workspaceTrusted
    ).toBe(true);
  });
});

describe("selectPaletteDestinationRoute", () => {
  it("Should prefer the palette intent window over the focused window", () => {
    const focused: OsWindow = {
      id: "window:focused",
      app: "tasks",
      instanceKey: null,
      route: { pathname: "/tasks", search: {} },
      navStack: [],
      pinned: false,
      desktopId: "desktop:main",
      placement: "floating",
      rect: { x: 0, y: 0, w: 600, h: 400 },
      layer: 1,
      minimized: false,
      groupId: null,
      nodeId: null,
      stackId: null,
      stackActive: true,
      parentAxis: null,
    };
    const intended: OsWindow = {
      ...focused,
      id: "window:intended",
      app: "agents",
      route: { pathname: "/agents", search: { view: "fleet" } },
    };
    const state: OsDesktopRuntimeStore = {
      snapshot: null,
      windowManagerConfig: null,
      client: null,
      desktops: [],
      projections: {},
      frames: {},
      windows: { [focused.id]: focused, [intended.id]: intended },
      activeDesktopId: "desktop:main",
      focusedId: focused.id,
      wallpaper: "ember",
      reduceMotion: false,
      dockMagnify: false,
      presentation: "floating",
      viewportState: "ready",
      hydration: "live",
      connectionStatus: "connected",
      desktopBounds: null,
    };

    expect(selectPaletteDestinationRoute(state, intended.id)).toEqual(intended.route);
    expect(selectPaletteDestinationRoute(state, undefined)).toEqual(focused.route);
    expect(selectPaletteDestinationRoute({ ...state, focusedId: null }, undefined)).toBeNull();
  });
});
