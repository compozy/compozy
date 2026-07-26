// Suite: OS interaction hooks
// Invariant: OS hooks translate keyboard, pointer, and viewport changes into current shell actions and geometry.
// Owning layer: the browser-to-OS-hook interaction boundary.
import { act, fireEvent, render, renderHook, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { OsShellContext, type OsShellHandle } from "../../contexts/os-shell-context";
import { RoutingCoordinator } from "../../lib/routing-coordinator";
import type {
  OsDesktopRuntimeStore,
  OsWindow,
  WindowManagerController,
  WindowManagerCommandOutcome,
} from "../../lib/os-types";
import type { WindowManagerConfig, WindowManagerSnapshot } from "../../lib/window-manager-types";
import {
  OS_ZOOM_MENU_CLOSE_GRACE_MS,
  OS_ZOOM_MENU_OPEN_DELAY_MS,
  useOsZoomMenu,
} from "../use-os-zoom-menu";
import { useOsWindow } from "../use-os-window";
import { useOsWindowCommands } from "../use-os-window-commands";
import { useOsWinLayer } from "../use-os-win-layer";
import { useOsShortcuts, type OsShortcutHandlers } from "../use-os-shortcuts";
import { windowManagerStore } from "../../stores/window-manager-store";

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

const SNAPSHOT: WindowManagerSnapshot = {
  version: 1,
  workspaceId: "workspace:test",
  revision: 7,
  desktops: [],
  windows: {},
  history: { undo: [], redo: [] },
  overrides: {},
  updatedAt: "2026-07-22T00:00:00Z",
};

function windowFixture(id: string, layer: number): OsWindow {
  return {
    id,
    app: "tasks",
    instanceKey: null,
    route: { pathname: "/tasks", search: {} },
    desktopId: "desktop:main",
    placement: "floating",
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

function acceptedOutcome(): WindowManagerCommandOutcome {
  return { accepted: true, completion: Promise.resolve(true) };
}

function createShell({ live = true, withPeer = true } = {}) {
  const primary = windowFixture("window:primary", 2);
  const peer = windowFixture("window:peer", 1);
  const zoomWindow = vi.fn(() => acceptedOutcome());
  const toggleFloating = vi.fn();
  const arrangeLayout = vi.fn();
  const state: OsDesktopRuntimeStore = {
    snapshot: SNAPSHOT,
    windowManagerConfig: CONFIG,
    client: {
      workspaceId: "workspace:test",
      clientId: "client:web",
      activeDesktopId: "desktop:main",
      focusedWindowId: primary.id,
      focusOrder: [primary.id],
      connectedAt: "2026-07-22T00:00:00Z",
      presentationRevision: 1,
    },
    desktops: [],
    projections: {},
    windows: withPeer ? { [primary.id]: primary, [peer.id]: peer } : { [primary.id]: primary },
    activeDesktopId: "desktop:main",
    focusedId: primary.id,
    railCollapsedAgentIds: [],
    wallpaper: "ember",
    reduceMotion: false,
    dockMagnify: true,
    presentation: "floating",
    viewportState: "ready",
    hydration: "live",
    connectionStatus: live ? "connected" : "disconnected",
    desktopBounds: { width: 1280, height: 800, origin: { x: 0, y: 0 } },
    openOrFocus: vi.fn(target => ({
      windowId: `app:${target.app}`,
      ...acceptedOutcome(),
    })),
    closeWindow: vi.fn(async () => true),
    focusWindow: vi.fn(() => acceptedOutcome()),
    minimizeWindow: vi.fn(async () => true),
    restoreWindow: vi.fn(),
    zoomWindow,
    toggleFloating,
    moveWindow: vi.fn(),
    arrangeLayout,
    commitFloatingRect: vi.fn(() => acceptedOutcome()),
    resizeLayout: vi.fn(),
    balanceLayout: vi.fn(),
    navigateWindow: vi.fn(() => acceptedOutcome()),
    toggleRailGroup: vi.fn(),
    setWallpaper: vi.fn(),
    setDockMagnify: vi.fn(),
    setReduceMotion: vi.fn(),
    setDesktopBounds: vi.fn(),
  };
  let currentState = state;
  const listeners = new Set<Parameters<WindowManagerController["subscribe"]>[0]>();
  const controller: WindowManagerController = {
    getState: () => currentState,
    getInitialState: () => state,
    subscribe: listener => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    bind: vi.fn(),
    unbind: vi.fn(),
    setClient: vi.fn(),
    setConnectionStatus: vi.fn(),
    setLoadError: vi.fn(),
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
    refreshSnapshot: vi.fn(),
  };
  const coordinator = new RoutingCoordinator(controller, {
    navigate: vi.fn(),
    replace: vi.fn(),
  });
  const shell: OsShellHandle = { store: controller, manager: controller, coordinator };
  const wrapper = ({ children }: { children: ReactNode }) => (
    <OsShellContext.Provider value={shell}>{children}</OsShellContext.Provider>
  );
  return {
    controller,
    get state() {
      return currentState;
    },
    setRuntimeState(patch: Partial<OsDesktopRuntimeStore>) {
      const previous = currentState;
      currentState = { ...currentState, ...patch };
      for (const listener of listeners) listener(currentState, previous);
    },
    wrapper,
  };
}

afterEach(() => {
  act(() => {
    windowManagerStore.getState().actions.clearGesture();
    windowManagerStore.getState().actions.setWorkArea(null);
  });
  vi.useRealTimers();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

function beginPrimarySnapGesture(): void {
  const workArea = { x: 0, y: 0, w: 1280, h: 800 };
  const actions = windowManagerStore.getState().actions;
  actions.setWorkArea({ rect: workArea, origin: { x: 0, y: 0 } });
  actions.beginGesture({
    pointerId: 0,
    point: { x: 320, y: 200 },
    workArea,
    layoutRevision: SNAPSHOT.revision,
    source: {
      windowId: "window:primary",
      nodeId: null,
      groupId: null,
      moveMode: "window",
    },
  });
}

function dragData(x: number, y: number) {
  return {
    node: document.createElement("div"),
    x,
    y,
    deltaX: 0,
    deltaY: 0,
    lastX: x,
    lastY: y,
  };
}

describe("useOsWindow", () => {
  it("Should defer a never-visible window and retain it after first visibility", () => {
    const shell = createShell();
    shell.setRuntimeState({ activeDesktopId: "desktop:other" });
    const { result } = renderHook(() => useOsWindow("window:primary"), {
      wrapper: shell.wrapper,
    });

    expect(result.current.keepMounted).toBe(false);

    act(() => shell.setRuntimeState({ activeDesktopId: "desktop:main" }));
    expect(result.current.keepMounted).toBe(true);

    act(() => shell.setRuntimeState({ activeDesktopId: "desktop:other" }));
    expect(result.current.keepMounted).toBe(true);
  });

  it("Should restore the current authoritative position when a snap command is rejected", async () => {
    const shell = createShell();
    const completion = Promise.resolve(false);
    vi.mocked(shell.controller.applySnapTarget).mockReturnValue({ accepted: true, completion });
    beginPrimarySnapGesture();
    const { result } = renderHook(() => useOsWindow("window:primary"), {
      wrapper: shell.wrapper,
    });
    const updatePosition = vi.fn();
    const updateSize = vi.fn();
    Object.defineProperty(result.current.rndRef, "current", {
      configurable: true,
      value: { updatePosition, updateSize },
    });

    await act(async () => {
      result.current.handleDragStop(
        new MouseEvent("mouseup", { clientX: 1, clientY: 300 }),
        dragData(1, 300)
      );
      await completion;
    });

    expect(shell.controller.applySnapTarget).toHaveBeenCalledOnce();
    expect(updatePosition).toHaveBeenCalledWith({ x: 80, y: 40 });
    expect(updateSize).toHaveBeenCalledWith({ width: 600, height: 420 });
  });

  it("Should retain the local preview when a snap command is applied", async () => {
    const shell = createShell();
    const completion = Promise.resolve(true);
    vi.mocked(shell.controller.applySnapTarget).mockReturnValue({ accepted: true, completion });
    beginPrimarySnapGesture();
    const { result } = renderHook(() => useOsWindow("window:primary"), {
      wrapper: shell.wrapper,
    });
    const updatePosition = vi.fn();
    const updateSize = vi.fn();
    Object.defineProperty(result.current.rndRef, "current", {
      configurable: true,
      value: { updatePosition, updateSize },
    });

    await act(async () => {
      result.current.handleDragStop(
        new MouseEvent("mouseup", { clientX: 1, clientY: 300 }),
        dragData(1, 300)
      );
      await completion;
    });

    expect(shell.controller.applySnapTarget).toHaveBeenCalledOnce();
    expect(updatePosition).not.toHaveBeenCalled();
    expect(updateSize).not.toHaveBeenCalled();
  });

  it("Should restore the current authoritative rect when a floating commit is rejected", async () => {
    const shell = createShell();
    const completion = Promise.resolve(false);
    vi.mocked(shell.state.commitFloatingRect).mockReturnValue({ accepted: true, completion });
    beginPrimarySnapGesture();
    const { result } = renderHook(() => useOsWindow("window:primary"), {
      wrapper: shell.wrapper,
    });
    const updatePosition = vi.fn();
    const updateSize = vi.fn();
    Object.defineProperty(result.current.rndRef, "current", {
      configurable: true,
      value: { updatePosition, updateSize },
    });

    await act(async () => {
      result.current.handleDragStop(
        new MouseEvent("mouseup", { clientX: 500, clientY: 300 }),
        dragData(500, 300)
      );
      await completion;
    });

    expect(shell.state.commitFloatingRect).toHaveBeenCalledOnce();
    expect(shell.controller.applySnapTarget).not.toHaveBeenCalled();
    expect(updatePosition).toHaveBeenCalledWith({ x: 80, y: 40 });
    expect(updateSize).toHaveBeenCalledWith({ width: 600, height: 420 });
  });

  it("Should restore the authoritative rect after a stale drag stop finishes", async () => {
    const shell = createShell();
    beginPrimarySnapGesture();
    shell.state.snapshot = { ...SNAPSHOT, revision: SNAPSHOT.revision + 1 };
    const { result } = renderHook(() => useOsWindow("window:primary"), {
      wrapper: shell.wrapper,
    });
    const updatePosition = vi.fn();
    const updateSize = vi.fn();
    Object.defineProperty(result.current.rndRef, "current", {
      configurable: true,
      value: { updatePosition, updateSize },
    });

    await act(async () => {
      result.current.handleDragStop(
        new MouseEvent("mouseup", { clientX: 500, clientY: 300 }),
        dragData(500, 300)
      );
      expect(updatePosition).not.toHaveBeenCalled();
      await Promise.resolve();
    });

    expect(shell.state.commitFloatingRect).not.toHaveBeenCalled();
    expect(shell.controller.applySnapTarget).not.toHaveBeenCalled();
    expect(updatePosition).toHaveBeenCalledWith({ x: 80, y: 40 });
    expect(updateSize).toHaveBeenCalledWith({ width: 600, height: 420 });
  });

  it("Should restore the authoritative rect when drag stop has no active gesture", async () => {
    const shell = createShell();
    const { result } = renderHook(() => useOsWindow("window:primary"), {
      wrapper: shell.wrapper,
    });
    const updatePosition = vi.fn();
    const updateSize = vi.fn();
    Object.defineProperty(result.current.rndRef, "current", {
      configurable: true,
      value: { updatePosition, updateSize },
    });

    await act(async () => {
      result.current.handleDragStop(
        new MouseEvent("mouseup", { clientX: 500, clientY: 300 }),
        dragData(500, 300)
      );
      expect(updatePosition).not.toHaveBeenCalled();
      await Promise.resolve();
    });

    expect(shell.state.commitFloatingRect).not.toHaveBeenCalled();
    expect(shell.controller.applySnapTarget).not.toHaveBeenCalled();
    expect(updatePosition).toHaveBeenCalledWith({ x: 80, y: 40 });
    expect(updateSize).toHaveBeenCalledWith({ width: 600, height: 420 });
  });
});

function WinLayerHarness() {
  const { layerRef } = useOsWinLayer();
  return <div data-testid="win-layer" ref={layerRef} />;
}

describe("useOsShortcuts", () => {
  it("Should route shell shortcuts independently of window-manager availability", () => {
    const { wrapper } = createShell({ live: false });
    const handlers: OsShortcutHandlers = {
      onPalette: vi.fn(),
      onNewSession: vi.fn(),
      onDesktops: vi.fn(),
      onEscape: vi.fn(),
    };
    renderHook(() => useOsShortcuts(handlers), { wrapper });

    fireEvent.keyDown(document, { key: "k", code: "KeyK", metaKey: true });
    fireEvent.keyDown(document, { key: "n", code: "KeyN", metaKey: true });
    fireEvent.keyDown(document, { key: "Escape", code: "Escape" });

    expect(handlers.onPalette).toHaveBeenCalledOnce();
    expect(handlers.onNewSession).toHaveBeenCalledOnce();
    expect(handlers.onEscape).toHaveBeenCalledOnce();
  });

  it("Should dispatch window-manager chords only for a live client outside editable targets", () => {
    const liveShell = createShell();
    const unavailableShell = createShell({ live: false });
    const handlers: OsShortcutHandlers = {
      onPalette: vi.fn(),
      onNewSession: vi.fn(),
      onDesktops: vi.fn(),
      onEscape: vi.fn(),
    };
    const unavailable = renderHook(() => useOsShortcuts(handlers), {
      wrapper: unavailableShell.wrapper,
    });

    fireEvent.keyDown(document, { key: "z", code: "KeyZ", metaKey: true });
    expect(unavailableShell.controller.undoLayout).not.toHaveBeenCalled();
    unavailable.unmount();

    renderHook(() => useOsShortcuts(handlers), { wrapper: liveShell.wrapper });
    const input = document.createElement("input");
    document.body.append(input);
    fireEvent.keyDown(input, { key: "z", code: "KeyZ", metaKey: true });
    expect(liveShell.controller.undoLayout).not.toHaveBeenCalled();

    fireEvent.keyDown(document, { key: "z", code: "KeyZ", metaKey: true });
    expect(liveShell.controller.undoLayout).toHaveBeenCalledOnce();
    input.remove();
  });

  it("Should dispatch nothing while disabled, so first-run setup blocks the whole set", () => {
    const { wrapper, controller } = createShell();
    const handlers: OsShortcutHandlers = {
      onPalette: vi.fn(),
      onNewSession: vi.fn(),
      onDesktops: vi.fn(),
      onEscape: vi.fn(),
    };
    renderHook(() => useOsShortcuts(handlers, { enabled: false }), { wrapper });

    fireEvent.keyDown(document, { key: "k", code: "KeyK", metaKey: true });
    fireEvent.keyDown(document, { key: "n", code: "KeyN", metaKey: true });
    fireEvent.keyDown(document, { key: "z", code: "KeyZ", metaKey: true });
    fireEvent.keyDown(document, { key: "Escape", code: "Escape" });

    expect(handlers.onPalette).not.toHaveBeenCalled();
    expect(handlers.onNewSession).not.toHaveBeenCalled();
    expect(handlers.onDesktops).not.toHaveBeenCalled();
    expect(handlers.onEscape).not.toHaveBeenCalled();
    expect(controller.undoLayout).not.toHaveBeenCalled();
  });
});

describe("useOsZoomMenu", () => {
  it("Should apply mouse-only hover intent and preserve the close grace period", () => {
    vi.useFakeTimers();
    const { wrapper } = createShell();
    const { result } = renderHook(() => useOsZoomMenu("window:primary"), { wrapper });
    const mouseEvent = { pointerType: "mouse" } as Parameters<
      typeof result.current.onHoverEnter
    >[0];
    const touchEvent = { pointerType: "touch" } as Parameters<
      typeof result.current.onHoverEnter
    >[0];

    act(() => result.current.onHoverEnter(touchEvent));
    act(() => vi.advanceTimersByTime(OS_ZOOM_MENU_OPEN_DELAY_MS));
    expect(result.current.open).toBe(false);

    act(() => result.current.onHoverEnter(mouseEvent));
    act(() => vi.advanceTimersByTime(OS_ZOOM_MENU_OPEN_DELAY_MS - 1));
    expect(result.current.open).toBe(false);
    act(() => vi.advanceTimersByTime(1));
    expect(result.current.open).toBe(true);

    act(() => result.current.onHoverLeave());
    act(() => vi.advanceTimersByTime(OS_ZOOM_MENU_CLOSE_GRACE_MS - 1));
    expect(result.current.open).toBe(true);
    act(() => result.current.onContentEnter());
    act(() => vi.advanceTimersByTime(1));
    expect(result.current.open).toBe(true);
  });

  it("Should gate placement dispatch on live availability and close after an accepted action", () => {
    const liveShell = createShell();
    const unavailableShell = createShell({ live: false });
    const live = renderHook(() => useOsZoomMenu("window:primary"), {
      wrapper: liveShell.wrapper,
    });
    const unavailable = renderHook(() => useOsZoomMenu("window:primary"), {
      wrapper: unavailableShell.wrapper,
    });

    act(() => live.result.current.onOpenChange(true));
    act(() =>
      live.result.current.dispatchPlacement({
        id: "window.tile.left",
        placement: "left",
        label: "Tile left half",
      })
    );
    expect(liveShell.controller.tileWindow).toHaveBeenCalledWith("window:primary", "left");
    expect(live.result.current.open).toBe(false);

    act(() => unavailable.result.current.dispatchFill());
    expect(unavailableShell.state.zoomWindow).not.toHaveBeenCalled();
  });
});

describe("useOsWinLayer", () => {
  it("Should refresh the work-area origin when viewport chrome moves without resizing content", () => {
    let callback!: ResizeObserverCallback;
    let observer!: ResizeObserver;
    class ResizeObserverMock implements ResizeObserver {
      constructor(nextCallback: ResizeObserverCallback) {
        callback = nextCallback;
        observer = this;
      }

      observe() {}
      unobserve() {}
      disconnect() {}
    }
    vi.stubGlobal("ResizeObserver", ResizeObserverMock);
    let rect = new DOMRect(0, 44, 1280, 700);
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(() => rect);
    const shell = createShell();
    render(<WinLayerHarness />, { wrapper: shell.wrapper });
    const layer = screen.getByTestId("win-layer");
    const entry: ResizeObserverEntry = {
      target: layer,
      contentRect: new DOMRectReadOnly(0, 0, 1280, 700),
      borderBoxSize: [],
      contentBoxSize: [],
      devicePixelContentBoxSize: [],
    };
    callback([entry], observer);
    vi.mocked(shell.state.setDesktopBounds).mockClear();

    rect = new DOMRect(24, 52, 1280, 700);
    window.dispatchEvent(new Event("orientationchange"));

    expect(shell.state.setDesktopBounds).toHaveBeenCalledWith({
      width: 1280,
      height: 700,
      origin: { x: 24, y: 52 },
    });
  });
});

describe("useOsWindowCommands", () => {
  it("Should route focused-window lifecycle through the coordinator when the client is live", async () => {
    const shell = createShell();
    const { result } = renderHook(() => useOsWindowCommands(), { wrapper: shell.wrapper });

    expect(result.current.commandsAvailable).toBe(true);
    const actions = result.current.focusedWindowActions;
    if (actions === null) throw new Error("a focused live window must expose lifecycle actions");
    await act(async () => actions.close());
    expect(shell.state.closeWindow).toHaveBeenCalledWith("window:primary");
    await act(async () => actions.minimize());
    expect(shell.state.minimizeWindow).toHaveBeenCalledWith("window:primary");
    expect(actions.zoom).not.toBeNull();
    // The fixture window is already floating, so there is nothing to convert.
    expect(actions.makeFloating).toBeNull();
  });

  it("Should withhold every window command while the client is not live", () => {
    const shell = createShell({ live: false });
    const { result } = renderHook(() => useOsWindowCommands(), { wrapper: shell.wrapper });

    expect(result.current.commandsAvailable).toBe(false);
    expect(result.current.focusedWindowActions).toBeNull();
    expect(result.current.placementCommands).toHaveLength(0);
    expect(result.current.arrangeCommands).toHaveLength(0);
    expect(result.current.canToggleFloating).toBe(false);
    expect(result.current.canBalanceLayout).toBe(false);
    expect(result.current.canEditLayoutHistory).toBe(false);
    expect(result.current.canFocusDirection).toBe(false);
    expect(result.current.canSwitchDesktop).toBe(false);
  });

  it("Should offer arrange presets only while a visible peer exists", () => {
    const withPeer = renderHook(() => useOsWindowCommands(), { wrapper: createShell().wrapper });
    expect(withPeer.result.current.arrangeCommands.length).toBeGreaterThan(0);

    const alone = renderHook(() => useOsWindowCommands(), {
      wrapper: createShell({ withPeer: false }).wrapper,
    });
    expect(alone.result.current.arrangeCommands).toHaveLength(0);
    expect(alone.result.current.placementCommands.length).toBeGreaterThan(0);
  });

  it("Should enable desktop switching only past a second desktop", () => {
    const shell = createShell();
    const { result } = renderHook(() => useOsWindowCommands(), { wrapper: shell.wrapper });
    expect(result.current.canSwitchDesktop).toBe(false);

    act(() =>
      shell.setRuntimeState({
        desktops: [
          {
            id: "desktop:main",
            name: "Main",
            order: 0,
            purpose: "standard",
            focusOwner: null,
            groups: [],
            floating: [],
          },
          {
            id: "desktop:second",
            name: "Second",
            order: 1,
            purpose: "standard",
            focusOwner: null,
            groups: [],
            floating: [],
          },
        ],
      })
    );
    expect(result.current.canSwitchDesktop).toBe(true);
    act(() => result.current.switchDesktop("next"));
    expect(shell.controller.switchDesktopDirection).toHaveBeenCalledWith("next");
  });

  it("Should resolve shortcut glyphs from the live window-manager config", () => {
    const shell = createShell();
    const { result } = renderHook(() => useOsWindowCommands(), { wrapper: shell.wrapper });
    expect(result.current.shortcutLabels["window.close"]).toBe("⌘W");

    act(() =>
      shell.setRuntimeState({
        windowManagerConfig: { ...CONFIG, shortcuts: { "window.close": "control+shift+KeyW" } },
      })
    );
    expect(result.current.shortcutLabels["window.close"]).toBe("⌃⇧W");
  });
});
