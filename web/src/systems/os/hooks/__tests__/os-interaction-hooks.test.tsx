// Suite: OS interaction hooks
// Invariant: OS hooks translate keyboard, pointer, and viewport changes into current shell actions and geometry.
// Owning layer: the browser-to-OS-hook interaction boundary.
import { act, fireEvent, render, renderHook, screen } from "@testing-library/react";
import {
  type PointerEvent as ReactPointerEvent,
  type ReactNode,
  type RefObject,
  useEffect,
  useLayoutEffect,
} from "react";
import type { Rnd } from "react-rnd";
import { afterEach, describe, expect, it, vi } from "vitest";

import { OsShellContext, type OsShellHandle } from "../../contexts/os-shell-context";
import { WindowLiveDataContext } from "../../contexts/window-live-data-context";
import type { OsWindowFrameModel } from "../../lib/group-projection";
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
import { useAnimationFrameLatest } from "../use-animation-frame-latest";
import { useOsWindowCommands } from "../use-os-window-commands";
import { useWindowMergeTarget } from "../use-window-merge-target";
import {
  useCurrentWindowLiveDataEnabled,
  useWindowLiveDataEnabled,
} from "../use-window-live-data-enabled";
import { useOsWinLayer } from "../use-os-win-layer";
import { useOsShortcuts, type OsShortcutHandlers } from "../use-os-shortcuts";
import { useWorkspacesStripScroll } from "../use-workspaces-strip-scroll";
import { windowManagerStore } from "../../stores/window-manager-store";
import { createWindowManagerProjectionAtom } from "../../lib/window-manager-projection";

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
};

const SNAPSHOT: WindowManagerSnapshot = {
  version: 3,
  workspaceId: "workspace:test",
  revision: 7,
  desktops: [],
  windows: {},
  closedEntryCount: 0,
  overrides: {},
  updatedAt: "2026-07-22T00:00:00Z",
};

function windowFixture(id: string, layer: number): OsWindow {
  return {
    id,
    app: "tasks",
    instanceKey: null,
    route: { pathname: "/tasks", search: {} },
    navStack: [],
    pinned: false,
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
  const toggleFloating = vi.fn(() => acceptedOutcome());
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
      stackActive: {},
      connectedAt: "2026-07-22T00:00:00Z",
      presentationRevision: 1,
    },
    desktops: [],
    projections: {},
    frames: {},
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
  };
  let currentState = state;
  const projection = createWindowManagerProjectionAtom(state);
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
    openOrFocus: vi.fn(target => ({ windowId: `w-test-${target.app}`, ...acceptedOutcome() })),
    closeWindow: vi.fn(async () => true),
    focusWindow: vi.fn(() => acceptedOutcome()),
    minimizeWindow: vi.fn(async () => true),
    restoreWindow: vi.fn(() => acceptedOutcome()),
    zoomWindow,
    toggleFloating,
    moveWindow: vi.fn(() => acceptedOutcome()),
    arrangeLayout,
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
    toggleRailGroup: vi.fn(),
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
    refreshSnapshot: vi.fn(),
  };
  const coordinator = new RoutingCoordinator(controller, {
    navigate: vi.fn(),
    replace: vi.fn(),
  });
  const shell: OsShellHandle = { projection, manager: controller, coordinator };
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
      projection.set(currentState);
      for (const listener of listeners) listener(currentState, previous);
    },
    wrapper,
  };
}

afterEach(() => {
  act(() => {
    windowManagerStore.trigger.gestureCleared();
    windowManagerStore.trigger.workAreaMeasured({ workArea: null });
  });
  vi.useRealTimers();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

function beginPrimarySnapGesture(moveMode: "window" | "group" = "window"): void {
  const workArea = { x: 0, y: 0, w: 1280, h: 800 };
  windowManagerStore.trigger.workAreaMeasured({
    workArea: { rect: workArea, origin: { x: 0, y: 0 } },
  });
  windowManagerStore.trigger.gestureBegan({
    pointerId: 0,
    point: { x: 320, y: 200 },
    workArea,
    layoutRevision: SNAPSHOT.revision,
    source: {
      windowId: "window:primary",
      nodeId: null,
      groupId: null,
      moveMode,
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

function installAnimationFrameQueue() {
  let nextID = 1;
  const callbacks = new Map<number, FrameRequestCallback>();
  vi.stubGlobal(
    "requestAnimationFrame",
    vi.fn((callback: FrameRequestCallback) => {
      const id = nextID;
      nextID += 1;
      callbacks.set(id, callback);
      return id;
    })
  );
  vi.stubGlobal(
    "cancelAnimationFrame",
    vi.fn((id: number) => callbacks.delete(id))
  );
  const flushNow = () => {
    const pending = [...callbacks.values()];
    callbacks.clear();
    for (const callback of pending) callback(performance.now());
  };
  return {
    flushNow,
    flush() {
      act(() => {
        flushNow();
      });
    },
  };
}

/** Solo floating frame for the primary fixture window (frame rect = authoritative rect). */
function primaryFrame(): OsWindowFrameModel {
  return {
    id: "window:primary",
    desktopId: "desktop:main",
    kind: "floating",
    rect: { x: 80, y: 40, w: 600, h: 420 },
    members: ["window:primary"],
    activeWindowId: "window:primary",
    stackId: null,
    minimized: false,
    adapted: false,
    layer: 2,
    zone: null,
    resizableEdges: { left: true, right: true, top: true, bottom: true },
  };
}

function stackedFrame(): OsWindowFrameModel {
  return {
    ...primaryFrame(),
    id: "stack:primary",
    members: ["window:primary", "window:peer"],
    stackId: "stack:primary",
  };
}

describe("useAnimationFrameLatest", () => {
  it("Should flush pending work with the latest callback before passive effects", () => {
    const frames = installAnimationFrameQueue();
    const calls: string[] = [];
    const { rerender } = renderHook(
      ({ callbackVersion, flushBeforePassive }) => {
        const api = useAnimationFrameLatest<string>(value => {
          calls.push(`${callbackVersion}:${value}`);
        });
        useEffect(() => {
          if (callbackVersion === 1) api.schedule({ value: "drag" });
        }, [api, callbackVersion]);
        useLayoutEffect(() => {
          if (flushBeforePassive) frames.flushNow();
        }, [flushBeforePassive]);
        return api;
      },
      { initialProps: { callbackVersion: 1, flushBeforePassive: false } }
    );

    rerender({ callbackVersion: 2, flushBeforePassive: true });

    expect(calls).toEqual(["2:drag"]);
  });
});

describe("useWorkspacesStripScroll", () => {
  it("Should bind wheel scrubbing when strip elements arrive after the first render", () => {
    vi.useFakeTimers();
    const overlayRef: RefObject<HTMLElement | null> = { current: null };
    const onSettleFocus = vi.fn();
    const { result, rerender } = renderHook(
      ({ itemCount }) =>
        useWorkspacesStripScroll({
          overlayRef,
          reducedMotion: true,
          itemCount,
          onSettleFocus,
        }),
      { initialProps: { itemCount: 0 } }
    );
    const overlay = document.createElement("div");
    const track = document.createElement("div");
    Object.defineProperties(track, {
      clientWidth: { configurable: true, value: 100 },
      scrollWidth: { configurable: true, value: 400 },
    });
    overlay.append(track);
    overlayRef.current = overlay;
    result.current.trackRef.current = track;

    rerender({ itemCount: 1 });
    act(() => result.current.trackProps.onScroll());
    const wheel = new WheelEvent("wheel", { bubbles: true, cancelable: true, deltaY: 30 });
    act(() => overlay.dispatchEvent(wheel));

    expect(wheel.defaultPrevented).toBe(true);
    expect(track.scrollLeft).toBe(30);
  });

  it("Should settle wheel scrubbing that interrupts focus scrolling", () => {
    vi.useFakeTimers();
    const overlay = document.createElement("div");
    const overlayRef: RefObject<HTMLElement | null> = { current: overlay };
    const onSettleFocus = vi.fn();
    const { result, rerender } = renderHook(
      ({ itemCount }) =>
        useWorkspacesStripScroll({
          overlayRef,
          reducedMotion: false,
          itemCount,
          onSettleFocus,
        }),
      { initialProps: { itemCount: 0 } }
    );
    const track = document.createElement("div");
    const tile = document.createElement("button");
    tile.dataset.slot = "os-workspace-tile";
    track.append(tile);
    overlay.append(track);
    Object.defineProperties(track, {
      clientWidth: { configurable: true, value: 100 },
      scrollWidth: { configurable: true, value: 300 },
    });
    track.getBoundingClientRect = () => new DOMRect(0, 0, 100, 40);
    tile.getBoundingClientRect = () => new DOMRect(120, 0, 40, 40);
    track.scrollTo = vi.fn(options => {
      track.scrollLeft = typeof options === "number" ? options : (options.left ?? 0);
    });
    result.current.trackRef.current = track;

    rerender({ itemCount: 1 });
    act(() => result.current.trackProps.onScroll());
    onSettleFocus.mockClear();
    vi.clearAllTimers();
    act(() => result.current.keepInView(tile));
    const wheel = new WheelEvent("wheel", { bubbles: true, cancelable: true, deltaY: 30 });
    act(() => overlay.dispatchEvent(wheel));
    act(() => vi.advanceTimersByTime(90));

    expect(onSettleFocus).toHaveBeenCalledExactlyOnceWith(0);
  });

  it("Should settle a completed drag once and swallow its trailing click", () => {
    const overlay = document.createElement("div");
    const overlayRef: RefObject<HTMLElement | null> = { current: overlay };
    const onSettleFocus = vi.fn();
    const { result } = renderHook(() =>
      useWorkspacesStripScroll({
        overlayRef,
        reducedMotion: true,
        itemCount: 2,
        onSettleFocus,
      })
    );
    const track = document.createElement("div");
    track.setPointerCapture = vi.fn();
    track.getBoundingClientRect = () => new DOMRect(0, 0, 200, 50);
    for (const left of [0, 120]) {
      const tile = document.createElement("button");
      tile.dataset.slot = "os-workspace-tile";
      tile.getBoundingClientRect = () => new DOMRect(left, 0, 80, 40);
      track.append(tile);
    }
    result.current.trackRef.current = track;

    act(() => {
      result.current.trackProps.onPointerDown({
        button: 0,
        pointerId: 7,
        clientX: 100,
      } as ReactPointerEvent<HTMLDivElement>);
      result.current.trackProps.onPointerMove({
        pointerId: 7,
        clientX: 70,
      } as ReactPointerEvent<HTMLDivElement>);
      result.current.trackProps.onPointerUp({
        pointerId: 7,
      } as ReactPointerEvent<HTMLDivElement>);
    });

    expect(onSettleFocus).toHaveBeenCalledExactlyOnceWith(0);
    expect(result.current.consumeDragClick()).toBe(true);
    expect(result.current.consumeDragClick()).toBe(false);
  });
});

describe("useWindowLiveDataEnabled", () => {
  it("Should default to live outside the retained-window OS shell", () => {
    const { result } = renderHook(() => useCurrentWindowLiveDataEnabled());

    expect(result.current).toBe(true);
  });

  it("Should derive a retained descendant's live lease from its own window", () => {
    const shell = createShell();
    const Shell = shell.wrapper;
    const LiveDataProvider = ({ children }: { children: ReactNode }) => {
      const liveDataEnabled = useWindowLiveDataEnabled("window:primary");
      return <WindowLiveDataContext value={liveDataEnabled}>{children}</WindowLiveDataContext>;
    };
    const wrapper = ({ children }: { children: ReactNode }) => (
      <Shell>
        <LiveDataProvider>{children}</LiveDataProvider>
      </Shell>
    );
    const { result } = renderHook(() => useCurrentWindowLiveDataEnabled(), { wrapper });

    expect(result.current).toBe(true);
    act(() => shell.setRuntimeState({ activeDesktopId: "desktop:other" }));
    expect(result.current).toBe(false);
  });

  it("Should disable retained window work while the browser document is hidden", () => {
    let visibilityState: DocumentVisibilityState = "visible";
    vi.spyOn(document, "visibilityState", "get").mockImplementation(() => visibilityState);
    const shell = createShell();
    const { result } = renderHook(() => useWindowLiveDataEnabled("window:primary"), {
      wrapper: shell.wrapper,
    });

    expect(result.current).toBe(true);
    visibilityState = "hidden";
    fireEvent(document, new Event("visibilitychange"));
    expect(result.current).toBe(false);

    visibilityState = "visible";
    fireEvent(document, new Event("visibilitychange"));
    expect(result.current).toBe(true);
  });
});

describe("useOsWindow", () => {
  it("Should coalesce drag previews to the latest point in each animation frame", () => {
    const frames = installAnimationFrameQueue();
    const shell = createShell();
    beginPrimarySnapGesture();
    const previewed = vi.spyOn(windowManagerStore.trigger, "gesturePreviewed");
    const { result } = renderHook(() => useOsWindow(primaryFrame()), {
      wrapper: shell.wrapper,
    });

    act(() => {
      result.current.handleDrag(
        new MouseEvent("mousemove", { clientX: 200, clientY: 160 }),
        dragData(200, 160)
      );
      result.current.handleDrag(
        new MouseEvent("mousemove", { clientX: 360, clientY: 240 }),
        dragData(360, 240)
      );
    });

    expect(previewed).not.toHaveBeenCalled();
    frames.flush();
    expect(previewed).toHaveBeenCalledOnce();
    expect(previewed).toHaveBeenCalledWith(expect.objectContaining({ point: { x: 360, y: 240 } }));
  });

  it("Should defer a never-visible window and retain it after first visibility", () => {
    const shell = createShell();
    shell.setRuntimeState({ activeDesktopId: "desktop:other" });
    const { result } = renderHook(() => useOsWindow(primaryFrame()), {
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
    const { result } = renderHook(() => useOsWindow(primaryFrame()), {
      wrapper: shell.wrapper,
    });
    const updatePosition = vi.fn();
    const updateSize = vi.fn();
    result.current.registerRnd({ updatePosition, updateSize } as unknown as Rnd);

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
    const { result } = renderHook(() => useOsWindow(primaryFrame()), {
      wrapper: shell.wrapper,
    });
    const updatePosition = vi.fn();
    const updateSize = vi.fn();
    result.current.registerRnd({ updatePosition, updateSize } as unknown as Rnd);

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
    vi.mocked(shell.controller.commitFloatingRect).mockReturnValue({ accepted: true, completion });
    beginPrimarySnapGesture();
    const { result } = renderHook(() => useOsWindow(primaryFrame()), {
      wrapper: shell.wrapper,
    });
    const updatePosition = vi.fn();
    const updateSize = vi.fn();
    result.current.registerRnd({ updatePosition, updateSize } as unknown as Rnd);

    await act(async () => {
      result.current.handleDragStop(
        new MouseEvent("mouseup", { clientX: 500, clientY: 300 }),
        dragData(500, 300)
      );
      await completion;
    });

    expect(shell.controller.commitFloatingRect).toHaveBeenCalledOnce();
    expect(shell.controller.applySnapTarget).not.toHaveBeenCalled();
    expect(updatePosition).toHaveBeenCalledWith({ x: 80, y: 40 });
    expect(updateSize).toHaveBeenCalledWith({ width: 600, height: 420 });
  });

  it("Should restore the authoritative rect after a stale drag stop finishes", async () => {
    const shell = createShell();
    beginPrimarySnapGesture();
    shell.state.snapshot = { ...SNAPSHOT, revision: SNAPSHOT.revision + 1 };
    const { result } = renderHook(() => useOsWindow(primaryFrame()), {
      wrapper: shell.wrapper,
    });
    const updatePosition = vi.fn();
    const updateSize = vi.fn();
    result.current.registerRnd({ updatePosition, updateSize } as unknown as Rnd);

    await act(async () => {
      result.current.handleDragStop(
        new MouseEvent("mouseup", { clientX: 500, clientY: 300 }),
        dragData(500, 300)
      );
      expect(updatePosition).not.toHaveBeenCalled();
      await Promise.resolve();
    });

    expect(shell.controller.commitFloatingRect).not.toHaveBeenCalled();
    expect(shell.controller.applySnapTarget).not.toHaveBeenCalled();
    expect(updatePosition).toHaveBeenCalledWith({ x: 80, y: 40 });
    expect(updateSize).toHaveBeenCalledWith({ width: 600, height: 420 });
  });

  it("Should restore the authoritative rect when drag stop has no active gesture", async () => {
    const shell = createShell();
    const { result } = renderHook(() => useOsWindow(primaryFrame()), {
      wrapper: shell.wrapper,
    });
    const updatePosition = vi.fn();
    const updateSize = vi.fn();
    result.current.registerRnd({ updatePosition, updateSize } as unknown as Rnd);

    await act(async () => {
      result.current.handleDragStop(
        new MouseEvent("mouseup", { clientX: 500, clientY: 300 }),
        dragData(500, 300)
      );
      expect(updatePosition).not.toHaveBeenCalled();
      await Promise.resolve();
    });

    expect(shell.controller.commitFloatingRect).not.toHaveBeenCalled();
    expect(shell.controller.applySnapTarget).not.toHaveBeenCalled();
    expect(updatePosition).toHaveBeenCalledWith({ x: 80, y: 40 });
    expect(updateSize).toHaveBeenCalledWith({ width: 600, height: 420 });
  });

  it("Should commit a group snap when a floating tab frame drops on a tile zone", async () => {
    const shell = createShell();
    beginPrimarySnapGesture("group");
    const { result } = renderHook(() => useOsWindow(stackedFrame()), {
      wrapper: shell.wrapper,
    });

    await act(async () => {
      result.current.handleDragStop(
        new MouseEvent("mouseup", { clientX: 1, clientY: 300 }),
        dragData(1, 300)
      );
      await Promise.resolve();
    });

    expect(shell.controller.applySnapTarget).toHaveBeenCalledOnce();
    expect(shell.controller.applySnapTarget).toHaveBeenCalledWith(
      "window:primary",
      expect.objectContaining({ kind: "tile", edge: "left" }),
      true
    );
    expect(shell.controller.commitFloatingRect).not.toHaveBeenCalled();
  });

  it("Should commit a group snap when a floating tab frame drops on a split zone", async () => {
    const shell = createShell();
    const board = {
      ...windowFixture("window:board", 3),
      nodeId: "node:board",
      rect: { x: 660, y: 80, w: 400, h: 400 },
    };
    shell.setRuntimeState({ windows: { ...shell.state.windows, [board.id]: board } });
    beginPrimarySnapGesture("group");
    const { result } = renderHook(() => useOsWindow(stackedFrame()), {
      wrapper: shell.wrapper,
    });

    await act(async () => {
      result.current.handleDragStop(
        new MouseEvent("mouseup", { clientX: 700, clientY: 280 }),
        dragData(700, 280)
      );
      await Promise.resolve();
    });

    expect(shell.controller.applySnapTarget).toHaveBeenCalledWith(
      "window:primary",
      expect.objectContaining({ kind: "split", targetWindowId: "window:board" }),
      true
    );
    expect(shell.controller.commitFloatingRect).not.toHaveBeenCalled();
  });

  it("Should commit a whole-frame swap when a group move drops on an occupied center", async () => {
    const shell = createShell();
    const board = {
      ...windowFixture("window:board", 3),
      nodeId: "node:board",
      rect: { x: 660, y: 80, w: 400, h: 400 },
    };
    shell.setRuntimeState({ windows: { ...shell.state.windows, [board.id]: board } });
    beginPrimarySnapGesture("group");
    const { result } = renderHook(() => useOsWindow(stackedFrame()), {
      wrapper: shell.wrapper,
    });

    await act(async () => {
      result.current.handleDragStop(
        new MouseEvent("mouseup", { clientX: 860, clientY: 280 }),
        dragData(860, 280)
      );
      await Promise.resolve();
    });

    expect(shell.controller.applySnapTarget).toHaveBeenCalledOnce();
    expect(shell.controller.applySnapTarget).toHaveBeenCalledWith(
      "window:primary",
      expect.objectContaining({ kind: "swap", targetWindowId: "window:board" }),
      true
    );
    expect(shell.controller.commitFloatingRect).not.toHaveBeenCalled();
  });

  it("Should drag a tiled tab frame as one unit from its deck bar", async () => {
    const shell = createShell();
    act(() => {
      windowManagerStore.trigger.workAreaMeasured({
        workArea: { rect: { x: 0, y: 0, w: 1280, h: 800 }, origin: { x: 0, y: 0 } },
      });
    });
    const frame: OsWindowFrameModel = { ...stackedFrame(), kind: "tiled" };
    const { result } = renderHook(() => useOsWindow(frame), { wrapper: shell.wrapper });

    act(() => {
      result.current.handleDragStart(
        new MouseEvent("mousedown", { clientX: 320, clientY: 200 }),
        dragData(320, 200)
      );
    });

    const gesture = windowManagerStore.getSnapshot().context.gesture;
    expect(gesture?.status).toBe("active");
    expect(gesture?.status === "active" ? gesture.source.moveMode : null).toBe("group");

    await act(async () => {
      result.current.handleDragStop(
        new MouseEvent("mouseup", { clientX: 500, clientY: 300 }),
        dragData(500, 300)
      );
      await Promise.resolve();
    });

    expect(shell.controller.commitFloatingRect).toHaveBeenCalledOnce();
    expect(vi.mocked(shell.controller.commitFloatingRect).mock.calls[0]?.[3]).toBe(true);
    expect(shell.controller.applySnapTarget).not.toHaveBeenCalled();
  });

  it("Should hold the released rect while a floating commit is in flight", async () => {
    const shell = createShell();
    let resolveCompletion: (applied: boolean) => void = () => {};
    const completion = new Promise<boolean>(resolve => {
      resolveCompletion = resolve;
    });
    vi.mocked(shell.controller.commitFloatingRect).mockReturnValue({ accepted: true, completion });
    beginPrimarySnapGesture();
    const { result } = renderHook(() => useOsWindow(primaryFrame()), {
      wrapper: shell.wrapper,
    });

    act(() => {
      result.current.handleDragStop(
        new MouseEvent("mouseup", { clientX: 500, clientY: 300 }),
        dragData(500, 300)
      );
    });

    expect(shell.controller.commitFloatingRect).toHaveBeenCalledWith(
      "window:primary",
      { x: 500, y: 300, w: 600, h: 420 },
      undefined,
      false
    );
    expect(result.current.rect).toEqual({ x: 500, y: 300, w: 600, h: 420 });

    await act(async () => {
      resolveCompletion(true);
      await completion;
    });

    expect(result.current.rect).toEqual(primaryFrame().rect);
  });

  it("Should commit one ordered group command when the release point targets a deck [UT-095]", async () => {
    const shell = createShell();
    const target = windowFixture("window:target", 3);
    shell.setRuntimeState({
      windows: { ...shell.state.windows, [target.id]: target },
    });
    beginPrimarySnapGesture();
    act(() => {
      windowManagerStore.trigger.deckDropTargeted({
        target: {
          frameId: "stack:target",
          targetWindowId: target.id,
          insertIndex: 1,
        },
      });
    });
    const { result } = renderHook(() => useOsWindow(stackedFrame()), {
      wrapper: shell.wrapper,
    });

    await act(async () => {
      result.current.handleDragStop(
        new MouseEvent("mouseup", { clientX: 500, clientY: 300 }),
        dragData(500, 300)
      );
      await Promise.resolve();
    });

    expect(shell.controller.groupWindows).toHaveBeenCalledOnce();
    expect(shell.controller.groupWindows).toHaveBeenCalledWith(
      target.id,
      ["window:primary", "window:peer"],
      1
    );
    expect(shell.controller.applySnapTarget).not.toHaveBeenCalled();
    expect(shell.controller.commitFloatingRect).not.toHaveBeenCalled();
    expect(windowManagerStore.getSnapshot().context.deckDropTarget).toBeNull();
  });
});

describe("useWindowMergeTarget", () => {
  function targetFrame(): OsWindowFrameModel {
    return {
      id: "window:target",
      desktopId: "desktop:main",
      kind: "floating",
      rect: { x: 100, y: 50, w: 300, h: 260 },
      members: ["window:target"],
      activeWindowId: "window:target",
      stackId: null,
      minimized: false,
      adapted: false,
      layer: 3,
      zone: null,
      resizableEdges: { left: true, right: true, top: true, bottom: true },
    };
  }

  function chromeWithHead(): HTMLElement {
    const chrome = document.createElement("section");
    const head = document.createElement("div");
    head.setAttribute("data-slot", "os-window-head");
    Object.defineProperty(head, "getBoundingClientRect", {
      configurable: true,
      value: () => ({
        left: 100,
        right: 400,
        top: 50,
        bottom: 94,
        width: 300,
        height: 44,
        x: 100,
        y: 50,
        toJSON: () => ({}),
      }),
    });
    chrome.appendChild(head);
    return chrome;
  }

  function pointerMove(clientX: number, clientY: number): void {
    const event = new Event("pointermove");
    Object.defineProperties(event, {
      clientX: { value: clientX },
      clientY: { value: clientY },
    });
    window.dispatchEvent(event);
  }

  it("Should advertise a solo head as the group target while another frame drags over it", () => {
    const frames = installAnimationFrameQueue();
    const shell = createShell();
    shell.setRuntimeState({ frames: { "desktop:main": [targetFrame()] } });
    const { result } = renderHook(() => useWindowMergeTarget(targetFrame(), true), {
      wrapper: shell.wrapper,
    });
    result.current.chromeRef.current = chromeWithHead();

    act(() => beginPrimarySnapGesture());
    act(() => pointerMove(200, 60));
    frames.flush();

    expect(windowManagerStore.getSnapshot().context.deckDropTarget).toEqual({
      frameId: "window:target",
      targetWindowId: "window:target",
      insertIndex: 1,
    });
    expect(result.current.mergeTargeted).toBe(true);

    act(() => pointerMove(600, 60));
    frames.flush();
    expect(windowManagerStore.getSnapshot().context.deckDropTarget).toBeNull();
  });

  it("Should preserve a drop target published by a deck", () => {
    const frames = installAnimationFrameQueue();
    const shell = createShell();
    shell.setRuntimeState({ frames: { "desktop:main": [targetFrame()] } });
    renderHook(() => useWindowMergeTarget(targetFrame(), true), {
      wrapper: shell.wrapper,
    });

    act(() => beginPrimarySnapGesture());
    act(() => {
      windowManagerStore.trigger.deckDropTargeted({
        target: {
          frameId: "frame:deck",
          targetWindowId: "window:deck-target",
          insertIndex: 1,
        },
      });
      pointerMove(600, 60);
    });
    frames.flush();

    expect(windowManagerStore.getSnapshot().context.deckDropTarget).toEqual({
      frameId: "frame:deck",
      targetWindowId: "window:deck-target",
      insertIndex: 1,
    });
  });

  it("Should measure only the visible top frame when heads overlap", () => {
    const frames = installAnimationFrameQueue();
    const shell = createShell();
    const over: OsWindowFrameModel = {
      ...targetFrame(),
      id: "frame:over",
      members: ["window:over"],
      activeWindowId: "window:over",
      layer: 9,
      rect: { x: 0, y: 0, w: 1280, h: 800 },
    };
    shell.setRuntimeState({ frames: { "desktop:main": [targetFrame(), over] } });
    const { result } = renderHook(
      () => ({
        lower: useWindowMergeTarget(targetFrame(), true),
        upper: useWindowMergeTarget(over, true),
      }),
      { wrapper: shell.wrapper }
    );
    const lowerChrome = chromeWithHead();
    const upperChrome = chromeWithHead();
    const lowerHead = lowerChrome.querySelector('[data-slot="os-window-head"]');
    const upperHead = upperChrome.querySelector('[data-slot="os-window-head"]');
    if (!(lowerHead instanceof HTMLElement) || !(upperHead instanceof HTMLElement)) {
      throw new Error("window head fixtures are required");
    }
    const measureLower = vi.spyOn(lowerHead, "getBoundingClientRect");
    const measureUpper = vi.spyOn(upperHead, "getBoundingClientRect");
    result.current.lower.chromeRef.current = lowerChrome;
    result.current.upper.chromeRef.current = upperChrome;

    act(() => beginPrimarySnapGesture());
    act(() => pointerMove(200, 60));
    frames.flush();

    expect(measureUpper).toHaveBeenCalledOnce();
    expect(measureLower).not.toHaveBeenCalled();
    expect(windowManagerStore.getSnapshot().context.deckDropTarget).toEqual({
      frameId: "frame:over",
      targetWindowId: "window:over",
      insertIndex: 1,
    });
  });

  it("Should measure only the latest pointer position once per animation frame", () => {
    const frames = installAnimationFrameQueue();
    const shell = createShell();
    shell.setRuntimeState({ frames: { "desktop:main": [targetFrame()] } });
    const { result } = renderHook(() => useWindowMergeTarget(targetFrame(), true), {
      wrapper: shell.wrapper,
    });
    const chrome = chromeWithHead();
    const head = chrome.querySelector('[data-slot="os-window-head"]');
    if (!(head instanceof HTMLElement)) throw new Error("window head fixture is required");
    const measure = vi.spyOn(head, "getBoundingClientRect");
    result.current.chromeRef.current = chrome;

    act(() => beginPrimarySnapGesture());
    act(() => {
      pointerMove(200, 60);
      pointerMove(600, 60);
      pointerMove(240, 70);
    });

    expect(measure).not.toHaveBeenCalled();
    frames.flush();
    expect(measure).toHaveBeenCalledOnce();
    expect(windowManagerStore.getSnapshot().context.deckDropTarget?.frameId).toBe("window:target");
  });

  it("Should use the committed frame when the coordinator runs before passive effects", () => {
    const frames = installAnimationFrameQueue();
    const shell = createShell();
    const committedFrame = {
      ...targetFrame(),
      members: ["window:target", "window:latest"],
      activeWindowId: "window:latest",
    };
    act(() => beginPrimarySnapGesture());
    const { result, rerender } = renderHook(
      ({ frame, flushBeforePassive }) => {
        const model = useWindowMergeTarget(frame, true);
        useLayoutEffect(() => {
          if (!flushBeforePassive) return;
          pointerMove(200, 60);
          frames.flushNow();
        }, [flushBeforePassive]);
        return model;
      },
      {
        initialProps: {
          frame: targetFrame(),
          flushBeforePassive: false,
        },
        wrapper: shell.wrapper,
      }
    );
    result.current.chromeRef.current = chromeWithHead();

    rerender({ frame: committedFrame, flushBeforePassive: true });

    expect(windowManagerStore.getSnapshot().context.deckDropTarget).toEqual({
      frameId: "window:target",
      targetWindowId: "window:latest",
      insertIndex: 2,
    });
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
      onWorkspaces: vi.fn(),
      onEscape: vi.fn(),
      onToggleGlobalScope: vi.fn(),
    };
    renderHook(() => useOsShortcuts(handlers), { wrapper });

    fireEvent.keyDown(document, { key: "k", code: "KeyK", metaKey: true });
    fireEvent.keyDown(document, { key: "n", code: "KeyN", metaKey: true });
    fireEvent.keyDown(document, { key: "Escape", code: "Escape" });

    expect(handlers.onPalette).toHaveBeenCalledOnce();
    expect(handlers.onNewSession).toHaveBeenCalledOnce();
    expect(handlers.onEscape).toHaveBeenCalledOnce();
  });

  it("Should toggle Global scope on ⇧⌘G and skip editable targets", () => {
    const { wrapper } = createShell({ live: false });
    const handlers: OsShortcutHandlers = {
      onPalette: vi.fn(),
      onNewSession: vi.fn(),
      onDesktops: vi.fn(),
      onWorkspaces: vi.fn(),
      onEscape: vi.fn(),
      onToggleGlobalScope: vi.fn(),
    };
    renderHook(() => useOsShortcuts(handlers), { wrapper });

    fireEvent.keyDown(document, { key: "g", code: "KeyG", metaKey: true, shiftKey: true });
    expect(handlers.onToggleGlobalScope).toHaveBeenCalledOnce();

    const input = document.createElement("input");
    document.body.append(input);
    fireEvent.keyDown(input, { key: "g", code: "KeyG", metaKey: true, shiftKey: true });
    expect(handlers.onToggleGlobalScope).toHaveBeenCalledOnce();
    input.remove();
  });

  it("Should open the workspaces overview on ⇧⌘W through the action registry", () => {
    const shell = createShell();
    const handlers: OsShortcutHandlers = {
      onPalette: vi.fn(),
      onNewSession: vi.fn(),
      onDesktops: vi.fn(),
      onWorkspaces: vi.fn(),
      onEscape: vi.fn(),
      onToggleGlobalScope: vi.fn(),
    };
    renderHook(() => useOsShortcuts(handlers), { wrapper: shell.wrapper });

    fireEvent.keyDown(document, { key: "w", code: "KeyW", metaKey: true, shiftKey: true });

    expect(handlers.onWorkspaces).toHaveBeenCalledOnce();
    expect(shell.controller.closeWindow).not.toHaveBeenCalled();

    fireEvent.keyDown(document, { key: "w", code: "KeyW", metaKey: true });
    expect(shell.controller.closeWindow).toHaveBeenCalledExactlyOnceWith("window:primary");
  });

  it("Should leave a prevented Escape to the nested control", () => {
    const { wrapper } = createShell({ live: false });
    const handlers: OsShortcutHandlers = {
      onPalette: vi.fn(),
      onNewSession: vi.fn(),
      onDesktops: vi.fn(),
      onWorkspaces: vi.fn(),
      onEscape: vi.fn(),
      onToggleGlobalScope: vi.fn(),
    };
    renderHook(() => useOsShortcuts(handlers), { wrapper });

    const event = new KeyboardEvent("keydown", {
      key: "Escape",
      code: "Escape",
      bubbles: true,
      cancelable: true,
    });
    event.preventDefault();
    document.dispatchEvent(event);

    expect(handlers.onEscape).not.toHaveBeenCalled();
  });

  it("Should protect editor-owned chords while keeping tab lifecycle shortcuts global", () => {
    const liveShell = createShell();
    const unavailableShell = createShell({ live: false });
    const handlers: OsShortcutHandlers = {
      onPalette: vi.fn(),
      onNewSession: vi.fn(),
      onDesktops: vi.fn(),
      onWorkspaces: vi.fn(),
      onEscape: vi.fn(),
      onToggleGlobalScope: vi.fn(),
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
    fireEvent.keyDown(input, { key: "w", code: "KeyW", metaKey: true });
    expect(liveShell.controller.closeWindow).toHaveBeenCalledWith("window:primary");

    fireEvent.keyDown(document, { key: "z", code: "KeyZ", metaKey: true });
    expect(liveShell.controller.undoLayout).toHaveBeenCalledOnce();
    input.remove();
  });

  it("Should honor live tab overrides and ignore the shipped chord [UT-051]", () => {
    const shell = createShell();
    const handlers: OsShortcutHandlers = {
      onPalette: vi.fn(),
      onNewSession: vi.fn(),
      onDesktops: vi.fn(),
      onWorkspaces: vi.fn(),
      onEscape: vi.fn(),
      onToggleGlobalScope: vi.fn(),
    };
    act(() => {
      shell.setRuntimeState({
        frames: { "desktop:main": [stackedFrame()] },
        windowManagerConfig: {
          ...CONFIG,
          shortcuts: { "window.tab.next": "meta+KeyL" },
        },
      });
    });
    renderHook(() => useOsShortcuts(handlers), { wrapper: shell.wrapper });

    fireEvent.keyDown(document, { key: "Tab", code: "Tab", ctrlKey: true });
    expect(shell.controller.focusWindow).not.toHaveBeenCalled();

    fireEvent.keyDown(document, { key: "l", code: "KeyL", metaKey: true });
    expect(shell.controller.focusWindow).toHaveBeenCalledWith("window:peer");
  });

  it("Should dispatch the shipped tab lifecycle chords through the live shell", () => {
    const shell = createShell();
    const handlers: OsShortcutHandlers = {
      onPalette: vi.fn(),
      onNewSession: vi.fn(),
      onDesktops: vi.fn(),
      onWorkspaces: vi.fn(),
      onEscape: vi.fn(),
      onToggleGlobalScope: vi.fn(),
    };
    act(() => {
      shell.setRuntimeState({ frames: { "desktop:main": [stackedFrame()] } });
    });
    renderHook(() => useOsShortcuts(handlers), { wrapper: shell.wrapper });

    fireEvent.keyDown(document, { key: "t", code: "KeyT", metaKey: true });
    expect(shell.controller.openOrFocus).toHaveBeenCalledWith({
      app: "new-tab",
      stackTargetWindowId: "window:primary",
    });

    fireEvent.keyDown(document, {
      key: "t",
      code: "KeyT",
      metaKey: true,
      shiftKey: true,
    });
    expect(shell.controller.reopenWindow).toHaveBeenCalledOnce();

    fireEvent.keyDown(document, { key: "9", code: "Digit9", metaKey: true });
    expect(shell.controller.focusWindow).toHaveBeenCalledWith("window:peer");

    const jumpTargets = Array.from({ length: 8 }, (_, index) =>
      index === 0 ? "window:primary" : `window:slot-${index + 1}`
    );
    const activeWindowId = "window:active";
    const jumpWindows = Object.fromEntries(
      [...jumpTargets, activeWindowId].map((id, index) => [id, windowFixture(id, index + 1)])
    );
    act(() => {
      shell.setRuntimeState({
        frames: {
          "desktop:main": [
            {
              ...stackedFrame(),
              members: [...jumpTargets, activeWindowId],
              activeWindowId,
            },
          ],
        },
        windows: jumpWindows,
        focusedId: activeWindowId,
      });
    });
    vi.mocked(shell.controller.focusWindow).mockClear();

    jumpTargets.forEach((target, index) => {
      const slot = index + 1;
      fireEvent.keyDown(document, {
        key: String(slot),
        code: `Digit${slot}`,
        metaKey: true,
      });
      expect(shell.controller.focusWindow).toHaveBeenCalledTimes(slot);
      expect(shell.controller.focusWindow).toHaveBeenNthCalledWith(slot, target);
    });
  });

  it("Should dispatch the primary shortcut with Control on non-Apple platforms", () => {
    vi.spyOn(window.navigator, "platform", "get").mockReturnValue("Linux x86_64");
    const shell = createShell();
    const handlers: OsShortcutHandlers = {
      onPalette: vi.fn(),
      onNewSession: vi.fn(),
      onDesktops: vi.fn(),
      onWorkspaces: vi.fn(),
      onEscape: vi.fn(),
      onToggleGlobalScope: vi.fn(),
    };
    act(() => {
      shell.setRuntimeState({ frames: { "desktop:main": [stackedFrame()] } });
    });
    renderHook(() => useOsShortcuts(handlers), { wrapper: shell.wrapper });

    fireEvent.keyDown(document, { key: "t", code: "KeyT", ctrlKey: true });

    expect(shell.controller.openOrFocus).toHaveBeenCalledWith({
      app: "new-tab",
      stackTargetWindowId: "window:primary",
    });

    fireEvent.keyDown(document, { key: "[", code: "BracketLeft", ctrlKey: true });

    expect(shell.controller.popWindowRoute).toHaveBeenCalledWith("window:primary");
  });

  it("Should leave editable descendants and repeated chords untouched [UT-099]", () => {
    const shell = createShell();
    const handlers: OsShortcutHandlers = {
      onPalette: vi.fn(),
      onNewSession: vi.fn(),
      onDesktops: vi.fn(),
      onWorkspaces: vi.fn(),
      onEscape: vi.fn(),
      onToggleGlobalScope: vi.fn(),
    };
    renderHook(() => useOsShortcuts(handlers), { wrapper: shell.wrapper });
    const editable = document.createElement("div");
    editable.setAttribute("contenteditable", "true");
    const nestedTarget = document.createElement("span");
    editable.append(nestedTarget);
    document.body.append(editable);

    fireEvent.keyDown(nestedTarget, { key: "z", code: "KeyZ", metaKey: true });
    expect(shell.controller.undoLayout).not.toHaveBeenCalled();

    fireEvent.keyDown(document, { key: "z", code: "KeyZ", metaKey: true, repeat: true });

    expect(shell.controller.undoLayout).not.toHaveBeenCalled();
    editable.remove();
  });

  it("Should dispatch nothing while disabled, so first-run setup blocks the whole set", () => {
    const { wrapper, controller } = createShell();
    const handlers: OsShortcutHandlers = {
      onPalette: vi.fn(),
      onNewSession: vi.fn(),
      onDesktops: vi.fn(),
      onWorkspaces: vi.fn(),
      onEscape: vi.fn(),
      onToggleGlobalScope: vi.fn(),
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
    expect(unavailableShell.controller.zoomWindow).not.toHaveBeenCalled();
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
    vi.mocked(shell.controller.setDesktopBounds).mockClear();

    rect = new DOMRect(24, 52, 1280, 700);
    window.dispatchEvent(new Event("orientationchange"));

    expect(shell.controller.setDesktopBounds).toHaveBeenCalledWith({
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
    expect(shell.controller.closeWindow).toHaveBeenCalledWith("window:primary");
    await act(async () => actions.minimize());
    expect(shell.controller.minimizeWindow).toHaveBeenCalledWith("window:primary");
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
            floatingStacks: [],
          },
          {
            id: "desktop:second",
            name: "Second",
            order: 1,
            purpose: "standard",
            focusOwner: null,
            groups: [],
            floating: [],
            floatingStacks: [],
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
