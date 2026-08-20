import { QueryClient } from "@tanstack/react-query";
import { fn } from "storybook/test";

import type { OsShellHandle } from "../../contexts/os-shell-context";
import { RoutingCoordinator, type OsRouterPort } from "../../lib/routing-coordinator";
import type {
  OsDesktopRuntimeStore,
  OsWindow,
  WindowManagerCommandOutcome,
  WindowManagerController,
} from "../../lib/os-types";
import type { WindowManagerConfig, WindowManagerSnapshot } from "../../lib/window-manager-types";
import { WindowManagerRuntime } from "../../runtime/window-manager-runtime";
import { createWindowManagerProjectionAtom } from "../../lib/window-manager-projection";

const ROUTER: OsRouterPort = { navigate: () => {}, replace: () => {} };

/**
 * Story shell over the real runtime. Nothing is bound, so window commands read
 * as unavailable — which is exactly the resting state of a cold desktop.
 */
export function createStoryShell() {
  const manager = new WindowManagerRuntime(new QueryClient());
  const coordinator = new RoutingCoordinator(manager, ROUTER);
  coordinator.completeHydration();
  return { projection: manager.projectionAtom, manager, coordinator } satisfies OsShellHandle;
}

const STORY_DESKTOP_ID = "desktop:story";

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
  snap: { edgeBand: 32, cornerReach: 150, exitSlack: 16, repeatRatios: [0.5, 0.666667, 0.333333] },
  bindings: { topCenter: "zoom", bottomCenter: "reserved" },
  shortcuts: {},
  globalShortcuts: {},
  shortcutDefaults: {},
  effectiveShortcuts: {},
};

const SNAPSHOT: WindowManagerSnapshot = {
  version: 3,
  workspaceId: "workspace-compozy",
  revision: 12,
  desktops: [],
  windows: {},
  closedEntryCount: 0,
  overrides: {},
  updatedAt: "2026-07-23T01:00:00Z",
};

function storyWindow(id: string, layer: number): OsWindow {
  return {
    id,
    app: "tasks",
    instanceKey: null,
    route: { pathname: "/tasks", search: {} },
    navStack: [],
    pinned: false,
    desktopId: STORY_DESKTOP_ID,
    placement: "tiled",
    rect: { x: 40 * layer, y: 40, w: 600, h: 420 },
    layer,
    minimized: false,
    groupId: null,
    nodeId: null,
    stackId: null,
    stackActive: true,
    parentAxis: null,
  };
}

function outcome(): WindowManagerCommandOutcome {
  return { accepted: true, completion: Promise.resolve(true) };
}

/**
 * Story shell with a live client, two desktops, and a focused tiled window with
 * a visible peer — the state in which every Window-menu predicate passes, so
 * the enabled menu can be captured.
 */
export interface LiveStoryShellOptions {
  windows?: Record<string, OsWindow>;
  focusedWindowId?: string | null;
  windowManagerConfig?: WindowManagerConfig;
}

export function createLiveStoryShell({
  windows: suppliedWindows,
  focusedWindowId: suppliedFocusedWindowId,
  windowManagerConfig = CONFIG,
}: LiveStoryShellOptions = {}): OsShellHandle {
  const focused = storyWindow("w-story-tasks", 2);
  const peer = storyWindow("w-story-agents", 1);
  const windows = suppliedWindows ?? { [focused.id]: focused, [peer.id]: peer };
  const focusedWindowId = suppliedFocusedWindowId ?? focused.id;
  const state: OsDesktopRuntimeStore = {
    snapshot: SNAPSHOT,
    windowManagerConfig,
    client: {
      workspaceId: "workspace-compozy",
      clientId: "client:story",
      activeDesktopId: STORY_DESKTOP_ID,
      focusedWindowId,
      focusOrder: focusedWindowId === null ? [] : [focusedWindowId],
      stackActive: {},
      connectedAt: "2026-07-23T01:00:00Z",
      presentationRevision: 1,
    },
    desktops: [
      {
        id: STORY_DESKTOP_ID,
        name: "Launch",
        order: 0,
        purpose: "standard",
        focusOwner: focusedWindowId,
        groups: [],
        floating: [],
        floatingStacks: [],
      },
      {
        id: "desktop:second",
        name: "Review",
        order: 1,
        purpose: "standard",
        focusOwner: null,
        groups: [],
        floating: [],
        floatingStacks: [],
      },
    ],
    projections: {},
    frames: {},
    windows,
    activeDesktopId: STORY_DESKTOP_ID,
    focusedId: focusedWindowId,
    wallpaper: "ember",
    reduceMotion: false,
    dockMagnify: true,
    presentation: "floating",
    viewportState: "ready",
    hydration: "live",
    connectionStatus: "connected",
    desktopBounds: { width: 1440, height: 900, origin: { x: 0, y: 0 } },
  };
  const projection = createWindowManagerProjectionAtom(state);
  const controller: WindowManagerController = {
    getState: () => state,
    getInitialState: () => state,
    subscribe: () => () => undefined,
    bind: fn(),
    unbind: fn(),
    setClient: fn(),
    setConnectionStatus: fn(),
    setLoadError: fn(),
    openOrFocus: target => ({ windowId: `w-story-${target.app}`, ...outcome() }),
    closeWindow: async () => true,
    focusWindow: () => outcome(),
    minimizeWindow: async () => true,
    restoreWindow: () => outcome(),
    zoomWindow: () => outcome(),
    toggleFloating: () => outcome(),
    moveWindow: () => outcome(),
    arrangeLayout: fn(),
    commitFloatingRect: () => outcome(),
    resizeLayout: () => outcome(),
    resizeWindowFrame: () => outcome(),
    resizeGroupFrames: () => outcome(),
    balanceLayout: fn(),
    navigateWindow: () => outcome(),
    retargetWindow: () => outcome(),
    popWindowRoute: () => outcome(),
    groupWindows: () => outcome(),
    reorderStackMember: () => outcome(),
    activateStackMember: () => outcome(),
    pinWindow: () => outcome(),
    reopenWindow: () => outcome(),
    closeWindowScoped: async () => true,
    setWallpaper: fn(),
    setDockMagnify: fn(),
    setReduceMotion: fn(),
    setDesktopBounds: fn(),
    createDesktop: fn(),
    renameDesktop: fn(),
    reorderDesktop: fn(),
    switchDesktop: fn(),
    switchDesktopDirection: fn(),
    deleteDesktop: fn(),
    moveWindowToDesktop: fn(),
    tileWindow: fn(),
    applySnapTarget: () => outcome(),
    focusDirection: fn(),
    undoLayout: fn(),
    redoLayout: fn(),
    balanceFocusedLayout: fn(),
    clearConflict: fn(),
    refreshSnapshot: fn(),
  };
  return {
    projection,
    manager: controller,
    coordinator: new RoutingCoordinator(controller, ROUTER),
  };
}
