import { QueryClient } from "@tanstack/react-query";
import { fn } from "storybook/test";

import type { OsShellHandle } from "../../contexts/os-shell-context";
import { RoutingCoordinator, type OsRouterPort } from "../../lib/routing-coordinator";
import type { WindowManagerCommandOutcome, WindowManagerController } from "../../lib/os-types";
import type {
  LayoutTab,
  WindowManagerConfig,
  WindowManagerSnapshot,
  WindowManagerWindow,
} from "../../lib/window-manager-types";
import { buildOsDesktopRuntimeView } from "../../lib/window-manager-view";
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

function storyWindow(id: string, tabId: string, desktopId = STORY_DESKTOP_ID): WindowManagerWindow {
  return {
    id,
    app: "tasks",
    instanceKey: null,
    route: { pathname: "/tasks", search: {} },
    navStack: [],
    desktopId,
    tabId,
  };
}

function soloTab(id: string, windowId: string): LayoutTab {
  return { id, pinned: false, root: { id: `node:${windowId}`, kind: "leaf", windowId } };
}

/** Two desktops; the first holds two tabs, the focused one and a peer. */
const SNAPSHOT: WindowManagerSnapshot = {
  version: 6,
  workspaceId: "workspace-compozy",
  revision: 12,
  desktops: [
    {
      id: STORY_DESKTOP_ID,
      name: "Launch",
      order: 0,
      tabs: [
        soloTab("tab:story-tasks", "w-story-tasks"),
        soloTab("tab:story-agents", "w-story-agents"),
      ],
      activeTabId: "tab:story-tasks",
    },
    {
      id: "desktop:second",
      name: "Review",
      order: 1,
      tabs: [soloTab("tab:story-review", "w-story-review")],
      activeTabId: "tab:story-review",
    },
  ],
  windows: {
    "w-story-tasks": storyWindow("w-story-tasks", "tab:story-tasks"),
    "w-story-agents": storyWindow("w-story-agents", "tab:story-agents"),
    "w-story-review": storyWindow("w-story-review", "tab:story-review", "desktop:second"),
  },
  closedEntryCount: 0,
  overrides: {},
  migration: null,
  updatedAt: "2026-07-23T01:00:00Z",
};

function outcome(): WindowManagerCommandOutcome {
  return { accepted: true, completion: Promise.resolve(true) };
}

/**
 * Story shell with a live client, two desktops, and a focused window with a peer tab — the
 * state in which every Window-menu predicate passes, so the enabled menu can be captured.
 */
export interface LiveStoryShellOptions {
  snapshot?: WindowManagerSnapshot;
  focusedWindowId?: string | null;
  windowManagerConfig?: WindowManagerConfig;
}

export function createLiveStoryShell({
  snapshot = SNAPSHOT,
  focusedWindowId = "w-story-tasks",
  windowManagerConfig = CONFIG,
}: LiveStoryShellOptions = {}): OsShellHandle {
  const activeDesktopId = snapshot.desktops[0]?.id ?? STORY_DESKTOP_ID;
  const state = {
    ...buildOsDesktopRuntimeView({
      viewportWidth: 1440,
      snapshot,
      globalConfig: windowManagerConfig,
      client: {
        workspaceId: snapshot.workspaceId,
        clientId: "client:story",
        activeDesktopId,
        focusedWindowId,
        focusOrder: focusedWindowId === null ? [] : [focusedWindowId],
        activeTab: {},
        connectedAt: "2026-07-23T01:00:00Z",
        presentationRevision: 1,
      },
      workArea: { x: 0, y: 0, w: 1440, h: 860 },
      workAreaOrigin: { x: 0, y: 0 },
      seamPreview: null,
      routeIntents: {},
      connectionStatus: "connected",
      loadError: null,
      wallpaper: "flat",
      reduceMotion: false,
    }),
    clientAttachmentToken: "attachment-token",
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
    moveWindow: () => outcome(),
    swapWindows: () => outcome(),
    arrangeLayout: fn(),
    unsplitWindow: fn(),
    tileWindow: fn(),
    splitWithTab: fn(),
    moveTab: fn(),
    resizeLayout: () => outcome(),
    balanceLayout: fn(() => outcome()),
    balanceTab: fn(),
    commandsInFlight: () => false,
    commandsSettled: async () => {},
    navigateWindow: () => outcome(),
    retargetWindow: () => outcome(),
    popWindowRoute: () => outcome(),
    reorderTab: () => outcome(),
    activateTab: () => outcome(),
    pinWindow: () => outcome(),
    reopenWindow: () => outcome(),
    closeWindowScoped: async () => true,
    setWallpaper: fn(),
    setReduceMotion: fn(),
    setDesktopBounds: fn(),
    createDesktop: () => outcome(),
    renameDesktop: fn(),
    reorderDesktop: fn(),
    switchDesktop: fn(),
    switchDesktopDirection: fn(),
    deleteDesktop: fn(),
    moveWindowToDesktop: fn(),
    focusDirection: fn(),
    undoLayout: fn(),
    redoLayout: fn(),
    balanceFocusedLayout: fn(),
    clearConflict: fn(),
    refreshSnapshot: async () => true,
  };
  return {
    projection,
    manager: controller,
    coordinator: new RoutingCoordinator(controller, ROUTER),
  };
}
