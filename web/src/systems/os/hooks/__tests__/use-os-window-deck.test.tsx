// Suite: OS window deck gestures and scoped actions
// Invariant: deck gestures commit only their semantic command, while cancellation and no-target paths leave topology unchanged.
// Boundary IN: deck hook, browser pointer events, and the window-manager interaction store.
// Boundary OUT: frame-drag release grouping (use-os-window) and rendered menu affordances (os-window-deck).
import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { OsWindowFrameModel } from "../../lib/group-projection";
import type {
  OsDesktopRuntimeStore,
  OsWindow,
  WindowManagerCommandOutcome,
  WindowManagerController,
} from "../../lib/os-types";
import { windowManagerStore } from "../../stores/window-manager-store";
import { useOsWindowDeck } from "../use-os-window-deck";

const runtime = vi.hoisted(() => ({ state: null as OsDesktopRuntimeStore | null }));
const shell = vi.hoisted(() => ({
  manager: null as WindowManagerController | null,
  coordinator: {
    userActivateWindow: vi.fn(),
    userClose: vi.fn(),
    userFocus: vi.fn(),
    userOpen: vi.fn(),
  },
}));

vi.mock("@/systems/session", () => ({
  useSessions: () => ({ data: [] }),
}));
vi.mock("@/systems/session/hooks/use-sessions", () => ({
  useSessions: () => ({ data: [] }),
}));
vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({ runtimeWorkspaceId: "workspace:test" }),
}));
vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({ runtimeWorkspaceId: "workspace:test" }),
}));
vi.mock("../use-desktop", () => ({
  useDesktop: (selector: (state: OsDesktopRuntimeStore) => unknown) => selector(runtime.state!),
}));
vi.mock("../use-os-shell", () => ({
  useOsShell: () => ({ manager: shell.manager!, coordinator: shell.coordinator }),
}));

function acceptedOutcome(): WindowManagerCommandOutcome {
  return { accepted: true, completion: Promise.resolve(true) };
}

function windowFixture(id: string, overrides: Partial<OsWindow> = {}): OsWindow {
  return {
    id,
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
    groupId: "group:main",
    nodeId: null,
    stackId: "frame:main",
    stackActive: false,
    parentAxis: null,
    ...overrides,
  };
}

function frameFixture(overrides: Partial<OsWindowFrameModel> = {}): OsWindowFrameModel {
  return {
    id: "frame:main",
    desktopId: "desktop:main",
    kind: "floating",
    rect: { x: 0, y: 0, w: 600, h: 400 },
    zone: null,
    resizableEdges: { left: true, right: true, top: true, bottom: true },
    members: ["window:one", "window:two"],
    activeWindowId: "window:one",
    stackId: "frame:main",
    minimized: false,
    adapted: false,
    layer: 1,
    ...overrides,
  };
}

function runtimeFixture(windows: Record<string, OsWindow>): OsDesktopRuntimeStore {
  return {
    snapshot: null,
    windowManagerConfig: null,
    client: null,
    desktops: [],
    projections: {},
    frames: {},
    windows,
    activeDesktopId: "desktop:main",
    focusedId: "window:one",
    wallpaper: "ember",
    reduceMotion: false,
    dockMagnify: true,
    presentation: "floating",
    viewportState: "ready",
    hydration: "live",
    connectionStatus: "connected",
    desktopBounds: { width: 1280, height: 800, origin: { x: 0, y: 0 } },
  };
}

function controllerFixture(state: OsDesktopRuntimeStore): WindowManagerController {
  return {
    getState: () => state,
    getInitialState: () => state,
    subscribe: () => () => undefined,
    bind: vi.fn(),
    unbind: vi.fn(),
    setClient: vi.fn(),
    setConnectionStatus: vi.fn(),
    setLoadError: vi.fn(),
    openOrFocus: vi.fn(),
    closeWindow: vi.fn(),
    focusWindow: vi.fn(() => acceptedOutcome()),
    minimizeWindow: vi.fn(),
    restoreWindow: vi.fn(() => acceptedOutcome()),
    zoomWindow: vi.fn(() => acceptedOutcome()),
    toggleFloating: vi.fn(() => acceptedOutcome()),
    moveWindow: vi.fn(() => acceptedOutcome()),
    arrangeLayout: vi.fn(),
    commitFloatingRect: vi.fn(() => acceptedOutcome()),
    resizeLayout: vi.fn(() => acceptedOutcome()),
    resizeWindowFrame: vi.fn(() => acceptedOutcome()),
    resizeGroupFrames: vi.fn(() => acceptedOutcome()),
    balanceLayout: vi.fn(),
    navigateWindow: vi.fn(() => acceptedOutcome()),
    retargetWindow: vi.fn(() => acceptedOutcome()),
    popWindowRoute: vi.fn(() => acceptedOutcome()),
    groupWindows: vi.fn(() => acceptedOutcome()),
    reorderStackMember: vi.fn(() => acceptedOutcome()),
    activateStackMember: vi.fn(() => acceptedOutcome()),
    pinWindow: vi.fn(() => acceptedOutcome()),
    reopenWindow: vi.fn(() => acceptedOutcome()),
    closeWindowScoped: vi.fn(async () => true),
    setWallpaper: vi.fn(),
    setDockMagnify: vi.fn(),
    setReduceMotion: vi.fn(),
    setDesktopBounds: vi.fn(),
    createDesktop: vi.fn(),
    renameDesktop: vi.fn(),
    reorderDesktop: vi.fn(),
    switchDesktop: vi.fn(),
    switchDesktopDirection: vi.fn(),
    deleteDesktop: vi.fn(),
    moveWindowToDesktop: vi.fn(),
    tileWindow: vi.fn(),
    applySnapTarget: vi.fn(() => acceptedOutcome()),
    focusDirection: vi.fn(),
    undoLayout: vi.fn(),
    redoLayout: vi.fn(),
    balanceFocusedLayout: vi.fn(),
    clearConflict: vi.fn(),
    refreshSnapshot: vi.fn(async () => true),
  };
}

function installDeckGeometry(
  deck: ReturnType<typeof useOsWindowDeck>,
  memberIds: readonly string[]
): HTMLDivElement {
  const tabs = document.createElement("div");
  Object.defineProperty(tabs, "getBoundingClientRect", {
    configurable: true,
    value: () => ({ left: 0, right: 200, top: 20, bottom: 57, width: 200, height: 37 }),
  });
  deck.registerTabs(tabs);

  for (const [index, member] of memberIds.entries()) {
    const tab = document.createElement("button");
    Object.defineProperty(tab, "getBoundingClientRect", {
      configurable: true,
      value: () => ({
        left: index * 100,
        right: (index + 1) * 100,
        top: 20,
        bottom: 57,
        width: 100,
        height: 37,
      }),
    });
    deck.registerTab(member, tab);
  }
  return tabs;
}

function pointerEvent(type: string, clientX: number, clientY: number): Event {
  const event = new Event(type);
  Object.defineProperties(event, {
    clientX: { value: clientX },
    clientY: { value: clientY },
  });
  return event;
}

function prepareDeck(frame = frameFixture()) {
  const windows = {
    "window:one": windowFixture("window:one"),
    "window:two": windowFixture("window:two"),
    "window:three": windowFixture("window:three", { groupId: null, stackId: null }),
  };
  runtime.state = runtimeFixture(windows);
  shell.manager = controllerFixture(runtime.state);
  const hook = renderHook(() => useOsWindowDeck(frame));
  const tabs = installDeckGeometry(hook.result.current, frame.members);
  return { ...hook, frame, tabs, windows, manager: shell.manager };
}

afterEach(() => {
  act(() => windowManagerStore.trigger.gestureCleared());
  runtime.state = null;
  shell.manager = null;
  vi.clearAllMocks();
  vi.restoreAllMocks();
});

describe("useOsWindowDeck", () => {
  it("Should dispatch close-other and close-right scopes without widening either operation [UT-088]", () => {
    const { result, manager } = prepareDeck();

    act(() => {
      result.current.closeOthers("window:one");
      result.current.closeRight("window:two");
    });

    expect(manager.closeWindowScoped).toHaveBeenNthCalledWith(1, "window:one", "others");
    expect(manager.closeWindowScoped).toHaveBeenNthCalledWith(2, "window:two", "right");
  });

  it("Should tear a tab out beyond the deck threshold and focus its standalone window [UT-093]", async () => {
    const { result, manager } = prepareDeck();

    act(() => {
      result.current.handleTabPointerDown("window:one", {
        clientX: 30,
        clientY: 38,
      } as React.PointerEvent<HTMLElement>);
    });
    act(() => {
      window.dispatchEvent(pointerEvent("pointermove", 400, 160));
      window.dispatchEvent(pointerEvent("pointerup", 400, 160));
    });
    await act(async () => undefined);

    expect(manager.toggleFloating).toHaveBeenCalledWith("window:one", {
      x: 250,
      y: 144,
      w: 600,
      h: 400,
    });
    expect(shell.coordinator.userActivateWindow).toHaveBeenCalledWith("window:one");
  });

  it("Should coalesce tab-drag layout reads and release browser listeners at pointer end", () => {
    const animationFrames: FrameRequestCallback[] = [];
    vi.spyOn(window, "requestAnimationFrame").mockImplementation(callback => {
      animationFrames.push(callback);
      return animationFrames.length;
    });
    vi.spyOn(window, "cancelAnimationFrame").mockImplementation(() => undefined);
    const removeListener = vi.spyOn(window, "removeEventListener");
    const { result, tabs } = prepareDeck();
    const measure = vi.spyOn(tabs, "getBoundingClientRect");

    act(() => {
      result.current.handleTabPointerDown("window:one", {
        clientX: 30,
        clientY: 38,
      } as React.PointerEvent<HTMLElement>);
    });
    measure.mockClear();
    act(() => {
      window.dispatchEvent(pointerEvent("pointermove", 80, 38));
      window.dispatchEvent(pointerEvent("pointermove", 120, 38));
      window.dispatchEvent(pointerEvent("pointermove", 160, 38));
    });

    expect(measure).not.toHaveBeenCalled();
    act(() => animationFrames[0]?.(16));
    expect(measure).toHaveBeenCalledOnce();

    act(() => window.dispatchEvent(pointerEvent("pointerup", 160, 38)));
    measure.mockClear();
    act(() => window.dispatchEvent(pointerEvent("pointermove", 190, 38)));
    expect(measure).not.toHaveBeenCalled();
    expect(removeListener).toHaveBeenCalledWith("pointermove", expect.any(Function));
  });

  it.each([
    {
      name: "Escape",
      cancel: () => window.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" })),
    },
    {
      name: "pointer cancellation",
      cancel: () => window.dispatchEvent(pointerEvent("pointercancel", 400, 160)),
    },
  ])("Should leave deck order and topology untouched after $name [UT-094]", ({ cancel }) => {
    const { result, manager } = prepareDeck();

    act(() => {
      result.current.handleTabPointerDown("window:one", {
        clientX: 30,
        clientY: 38,
      } as React.PointerEvent<HTMLElement>);
    });
    act(() => {
      window.dispatchEvent(pointerEvent("pointermove", 400, 160));
      cancel();
      window.dispatchEvent(pointerEvent("pointerup", 400, 160));
    });

    expect(manager.toggleFloating).not.toHaveBeenCalled();
    expect(manager.reorderStackMember).not.toHaveBeenCalled();
  });

  it.each([
    {
      name: "tab activation",
      invoke: (deck: ReturnType<typeof useOsWindowDeck>) => deck.activateTab("window:two"),
      coordinatorMethod: "userFocus" as const,
    },
    {
      name: "tab close",
      invoke: (deck: ReturnType<typeof useOsWindowDeck>) => deck.closeTab("window:two"),
      coordinatorMethod: "userClose" as const,
    },
  ])(
    "Should cancel a live deck and frame gesture before $name [UT-075]",
    ({ invoke, coordinatorMethod }) => {
      const { result, manager } = prepareDeck();

      act(() => {
        result.current.handleTabPointerDown("window:one", {
          clientX: 30,
          clientY: 38,
        } as React.PointerEvent<HTMLElement>);
      });
      act(() => {
        window.dispatchEvent(pointerEvent("pointermove", 400, 160));
      });
      act(() => {
        windowManagerStore.trigger.workAreaMeasured({
          workArea: { rect: { x: 0, y: 0, w: 1280, h: 800 }, origin: { x: 0, y: 0 } },
        });
        windowManagerStore.trigger.gestureBegan({
          pointerId: 1,
          point: { x: 30, y: 38 },
          workArea: { x: 0, y: 0, w: 1280, h: 800 },
          layoutRevision: 1,
          source: { windowId: "window:one", nodeId: null, groupId: null, moveMode: "window" },
        });
        invoke(result.current);
        window.dispatchEvent(pointerEvent("pointerup", 400, 160));
      });

      expect(manager.toggleFloating).not.toHaveBeenCalled();
      expect(manager.reorderStackMember).not.toHaveBeenCalled();
      expect(windowManagerStore.getSnapshot().context.gesture).toMatchObject({
        status: "cancelled",
        reason: "manual",
      });
      expect(shell.coordinator[coordinatorMethod]).toHaveBeenCalledWith("window:two");
    }
  );

  it("Should activate the keyboard-requested tab after a pointer gesture targets another tab [UT-076]", () => {
    const { result } = prepareDeck();

    act(() => {
      result.current.handleTabPointerDown("window:one", {
        clientX: 30,
        clientY: 38,
      } as React.PointerEvent<HTMLElement>);
    });
    act(() => {
      window.dispatchEvent(pointerEvent("pointermove", 150, 38));
      result.current.activateTab("window:two");
    });

    expect(result.current.tabDrag).toBeNull();
    expect(shell.coordinator.userFocus).toHaveBeenCalledWith("window:two");
    expect(shell.coordinator.userFocus).not.toHaveBeenCalledWith("window:one");
  });

  it("Should advertise the pointer-derived insertion index and edge-scroll only while a foreign frame is active [UT-095]", () => {
    const animationFrames: FrameRequestCallback[] = [];
    vi.spyOn(window, "requestAnimationFrame").mockImplementation(callback => {
      animationFrames.push(callback);
      return animationFrames.length;
    });
    vi.spyOn(window, "cancelAnimationFrame").mockImplementation(() => undefined);
    const { result, frame, tabs } = prepareDeck();

    act(() => {
      windowManagerStore.trigger.workAreaMeasured({
        workArea: { rect: { x: 0, y: 0, w: 1280, h: 800 }, origin: { x: 0, y: 0 } },
      });
      windowManagerStore.trigger.gestureBegan({
        pointerId: 1,
        point: { x: 190, y: 38 },
        workArea: { x: 0, y: 0, w: 1280, h: 800 },
        layoutRevision: 1,
        source: { windowId: "window:three", nodeId: null, groupId: null, moveMode: "window" },
      });
    });
    act(() => {
      window.dispatchEvent(pointerEvent("pointermove", 190, 38));
    });
    act(() => animationFrames.shift()?.(16));

    expect(result.current.dropIndex).toBe(2);
    expect(tabs.scrollLeft).toBe(12);
    expect(windowManagerStore.getSnapshot().context.deckDropTarget).toEqual({
      frameId: frame.id,
      targetWindowId: "window:one",
      insertIndex: 2,
    });

    act(() => {
      window.dispatchEvent(pointerEvent("pointermove", 250, 100));
    });
    act(() => animationFrames.shift()?.(32));

    expect(result.current.dropIndex).toBeNull();
    expect(windowManagerStore.getSnapshot().context.deckDropTarget).toBeNull();
  });

  it("Should move one tab or merge exactly the other desktop windows without duplicating frame members [UT-096]", async () => {
    const { result, manager } = prepareDeck();

    act(() => {
      result.current.moveToNewWindow("window:two");
      result.current.mergeAllWindows();
    });
    await act(async () => undefined);

    expect(manager.toggleFloating).toHaveBeenCalledWith("window:two");
    expect(shell.coordinator.userActivateWindow).toHaveBeenCalledWith("window:two");
    expect(manager.groupWindows).toHaveBeenCalledOnce();
    expect(manager.groupWindows).toHaveBeenCalledWith("window:one", ["window:three"]);
  });

  it("Should leave merge-all inert when the frame already owns every window on its desktop [UT-096]", () => {
    const frame = frameFixture({ members: ["window:one", "window:two", "window:three"] });
    const { result, manager } = prepareDeck(frame);

    act(() => result.current.mergeAllWindows());

    expect(manager.groupWindows).not.toHaveBeenCalled();
  });
});
