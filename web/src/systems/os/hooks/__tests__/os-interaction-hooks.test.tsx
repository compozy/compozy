// Suite: OS interaction hooks
// Invariant: OS hooks translate keyboard, pointer, and viewport changes into current shell actions and geometry.
// Owning layer: the browser-to-OS-hook interaction boundary.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, renderHook, screen, waitFor } from "@testing-library/react";
import {
  type PointerEvent as ReactPointerEvent,
  type ReactNode,
  type RefObject,
  useEffect,
  useLayoutEffect,
} from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { OsShellContext, type OsShellHandle } from "../../contexts/os-shell-context";
import { RoutingCoordinator } from "../../lib/routing-coordinator";
import type {
  OsDesktopRuntimeStore,
  OsWindow,
  WindowManagerController,
  WindowManagerCommandOutcome,
} from "../../lib/os-types";
import type {
  LayoutNode,
  ProjectedSeam,
  WindowManagerConfig,
  WindowManagerSnapshot,
} from "../../lib/window-manager-types";
import type { WindowManagerGlobalShortcutMap } from "../../lib/window-manager-shortcut-types";
import { statusKeys } from "@/systems/status";
import { workspaceKeys } from "@/systems/workspace";
import { ensureWindowSlotStore, pruneWindowSlotStores } from "../../lib/window-slot-registry";
import { useCmdPaletteContext } from "../use-cmd-palette-context";
import { usePaletteTabs } from "../use-os-palette-entities";
import { useOsWindowMember } from "../use-os-window-member";
import { useAnimationFrameLatest } from "../use-animation-frame-latest";
import {
  useCurrentWindowLiveDataEnabled,
  useWindowLiveDataEnabled,
} from "../use-window-live-data-enabled";
import { useOsDeskLayer, useVisitedTabs } from "../use-os-desk-layer";
import { useOsShortcuts } from "../use-os-shortcuts";
import { useOsPaletteExecution } from "../use-os-palette-execution";
import type { PaletteRowSubject } from "../../lib/cmd-palette-row-actions";
import type { PaletteRegistry, ResolvedPaletteCommand } from "../../lib/cmd-palette-types";
import { adjacentShortcutItem } from "../../lib/adjacent-shortcut-item";
import { shortcutAttentionTarget } from "../use-desktop-shell-body";
import { useWorkspacesStripScroll } from "../use-workspaces-strip-scroll";
import { useDesktopCardDrag } from "../use-desktop-card-drag";
import { windowManagerStore } from "../../stores/window-manager-store";
import { osTabDragStore } from "../../stores/os-tab-drag-store";
import { useOsSplitSeam } from "../use-os-split-seam";
import { useOsTabDragStart } from "../use-os-tab-drag";
import { createWindowManagerProjectionAtom } from "../../lib/window-manager-projection";
import { useGlobalShortcutReconciliation } from "../use-global-shortcut-reconciliation";
import { useOsDocumentReducedMotion } from "../use-os-reduced-motion";
import { useDesktopPager, useDesktopShellState } from "../use-desktop-shell-state";
import { useDesktopManagerSurfaces } from "../use-desktop-manager-surfaces";
import type { OsSessionAttentionRow } from "../../lib/attention-model";

const TEST_KEYMAP = {
  "palette.open": ["meta+KeyK", "meta+shift+KeyP"],
  "session.new": ["meta+KeyN"],
  "scope.global.toggle": ["meta+shift+KeyG"],
  "workspace.picker": ["meta+shift+KeyO"],
  "window.close": ["meta+KeyW"],
  "window.nav.back": ["meta+BracketLeft"],
  "window.tab.new": ["meta+KeyT"],
  "window.tab.next": ["control+Tab"],
  "window.tab.previous": ["control+shift+Tab"],
  "window.tab.last": ["meta+Digit9"],
  "window.tab.reopen": ["meta+shift+KeyT"],
  "window.tab.jump.1": ["meta+Digit1"],
  "window.tab.jump.2": ["meta+Digit2"],
  "window.tab.jump.3": ["meta+Digit3"],
  "window.tab.jump.4": ["meta+Digit4"],
  "window.tab.jump.5": ["meta+Digit5"],
  "window.tab.jump.6": ["meta+Digit6"],
  "window.tab.jump.7": ["meta+Digit7"],
  "window.tab.jump.8": ["meta+Digit8"],
  "layout.undo": ["meta+KeyZ"],
  "shortcuts.cheatsheet": ["shift+Slash", "meta+Slash"],
} as const;

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
  shortcutDefaults: TEST_KEYMAP,
  effectiveShortcuts: TEST_KEYMAP,
};

/**
 * A registry projection carrying the test keymap. The keyboard listener reads
 * chords, availability and ids from here exactly as it does in the shell — no
 * TypeScript keymap of its own remains.
 */
function shortcutCommand(
  id: string,
  chords: readonly string[],
  available: boolean
): ResolvedPaletteCommand {
  return {
    id,
    title: id,
    section: "Shell",
    icon: "command",
    source: "core",
    bindings: [...chords],
    alias: null,
    destructive: false,
    availability_exempt: id === "shortcuts.cheatsheet",
    arguments: [],
    action: { kind: "client_op", op: id },
    execution: { retry_safe: true, single_flight: false },
    visible: true,
    available,
    reason: available ? "" : "CompozyOS isn't reachable right now",
    chords: [],
  } as ResolvedPaletteCommand;
}

function shortcutRegistry(
  overrides: Record<string, readonly string[]> = {},
  availability: Record<string, boolean> = {}
): PaletteRegistry {
  const bindings: Record<string, readonly string[]> = { ...TEST_KEYMAP, ...overrides };
  const commands = Object.entries(bindings).map(([id, chords]) =>
    shortcutCommand(id, chords, availability[id] ?? true)
  );
  return {
    commands,
    byId: new Map(commands.map(command => [command.id, command])),
    sources: [{ source: "core", status: "healthy" }],
    catalogRevision: "sha256:test",
    stale: false,
    daemonReachable: true,
  };
}

const SNAPSHOT: WindowManagerSnapshot = {
  version: 6,
  workspaceId: "workspace:test",
  revision: 7,
  desktops: [],
  windows: {},
  closedEntryCount: 0,
  overrides: {},
  migration: null,
  updatedAt: "2026-07-22T00:00:00Z",
};

function windowFixture(id: string, overrides: Partial<OsWindow> = {}): OsWindow {
  return {
    id,
    app: "tasks",
    instanceKey: null,
    route: { pathname: "/tasks", search: {} },
    navStack: [],
    desktopId: "desktop:main",
    tabId: `tab:${id}`,
    nodeId: `leaf:${id}`,
    pinned: false,
    inSplit: false,
    tabActive: true,
    shown: true,
    ...overrides,
  };
}

function acceptedOutcome(): WindowManagerCommandOutcome {
  return { accepted: true, completion: Promise.resolve(true) };
}

function createShell({ live = true, authoritative = true, withPeer = true } = {}) {
  // The two windows share one split tab, side by side.
  const primary = windowFixture("window:primary", { tabId: "tab:main", inSplit: true });
  const peer = windowFixture("window:peer", { tabId: "tab:main", inSplit: true });
  const arrangeLayout = vi.fn();
  const state: OsDesktopRuntimeStore = {
    snapshot: authoritative ? SNAPSHOT : null,
    windowManagerConfig: authoritative ? CONFIG : null,
    clientAttachmentToken: null,
    client: authoritative
      ? {
          workspaceId: "workspace:test",
          clientId: "client:web",
          activeDesktopId: "desktop:main",
          focusedWindowId: primary.id,
          focusOrder: [primary.id],
          activeTab: {},
          connectedAt: "2026-07-22T00:00:00Z",
          presentationRevision: 1,
        }
      : null,
    desktops: [],
    activeTabIds: {},
    projections: {},
    windows: withPeer ? { [primary.id]: primary, [peer.id]: peer } : { [primary.id]: primary },
    activeDesktopId: "desktop:main",
    focusedId: primary.id,
    wallpaper: "ember",
    reduceMotion: false,
    presentation: "split",
    viewportState: "ready",
    hydration: authoritative ? "live" : "pending",
    loadError: null,
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
    moveWindow: vi.fn(() => acceptedOutcome()),
    swapWindows: vi.fn(() => acceptedOutcome()),
    arrangeLayout,
    unsplitWindow: vi.fn(),
    tileWindow: vi.fn(),
    splitWithTab: vi.fn(),
    moveTab: vi.fn(),
    resizeLayout: vi.fn(() => acceptedOutcome()),
    balanceLayout: vi.fn(),
    navigateWindow: vi.fn(() => acceptedOutcome()),
    retargetWindow: vi.fn(() => acceptedOutcome()),
    popWindowRoute: vi.fn(() => acceptedOutcome()),
    reorderTab: vi.fn(() => acceptedOutcome()),
    activateTab: vi.fn(() => acceptedOutcome()),
    pinWindow: vi.fn(() => acceptedOutcome()),
    reopenWindow: vi.fn(() => acceptedOutcome()),
    closeWindowScoped: vi.fn(async () => true),
    setWallpaper: vi.fn(),
    setReduceMotion: vi.fn(),
    setDesktopBounds: vi.fn(),
    createDesktop: vi.fn(() => acceptedOutcome()),
    renameDesktop: vi.fn(),
    reorderDesktop: vi.fn(),
    switchDesktop: vi.fn(),
    switchDesktopDirection: vi.fn(),
    deleteDesktop: vi.fn(),
    moveWindowToDesktop: vi.fn(),
    focusDirection: vi.fn(),
    undoLayout: vi.fn(),
    redoLayout: vi.fn(),
    balanceFocusedLayout: vi.fn(),
    clearConflict: vi.fn(),
    refreshSnapshot: vi.fn(async () => true),
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
    windowManagerStore.trigger.workAreaMeasured({ workArea: null });
  });
  vi.useRealTimers();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  delete window.compozyShell;
});

// Invariant: either motion preference controls document animations and live changes stay in sync.
// Owning layer: shell-to-browser preference binding; canonical suite: OS interaction hooks.
describe("document reduced motion", () => {
  it.each([
    [false, false, false],
    [true, false, true],
    [false, true, true],
    [true, true, true],
  ])("Should publish app=%s or system=%s as reduced=%s", (app, system, reduced) => {
    const shell = createShell();
    shell.setRuntimeState({ reduceMotion: app });
    const media = new EventTarget();
    vi.spyOn(window, "matchMedia").mockReturnValue({
      matches: system,
      addEventListener: media.addEventListener.bind(media),
      removeEventListener: media.removeEventListener.bind(media),
    } as MediaQueryList);

    const { result, unmount } = renderHook(useOsDocumentReducedMotion, { wrapper: shell.wrapper });
    expect(result.current).toBe(reduced);
    expect(document.documentElement).toHaveAttribute("data-reduce-motion", String(reduced));
    unmount();
    expect(document.documentElement).not.toHaveAttribute("data-reduce-motion");
  });

  it("Should follow both preferences live and restore the host attribute on unmount", () => {
    const shell = createShell();
    const media = new EventTarget();
    let system = false;
    vi.spyOn(window, "matchMedia").mockReturnValue({
      get matches() {
        return system;
      },
      addEventListener: media.addEventListener.bind(media),
      removeEventListener: media.removeEventListener.bind(media),
    } as MediaQueryList);
    document.documentElement.setAttribute("data-reduce-motion", "false");
    const { unmount } = renderHook(useOsDocumentReducedMotion, { wrapper: shell.wrapper });

    act(() => shell.setRuntimeState({ reduceMotion: true }));
    expect(document.documentElement).toHaveAttribute("data-reduce-motion", "true");
    act(() => {
      system = true;
      media.dispatchEvent(new Event("change"));
      shell.setRuntimeState({ reduceMotion: false });
    });
    expect(document.documentElement).toHaveAttribute("data-reduce-motion", "true");
    act(() => {
      system = false;
      media.dispatchEvent(new Event("change"));
    });
    expect(document.documentElement).toHaveAttribute("data-reduce-motion", "false");
    act(() => shell.setRuntimeState({ reduceMotion: true }));
    unmount();
    expect(document.documentElement).toHaveAttribute("data-reduce-motion", "false");
    document.documentElement.removeAttribute("data-reduce-motion");
  });
});

// Invariant: shell metadata ignores geometry, while window membership, attention,
// and an opened overview immediately reflect the latest authoritative projection.
// Owning layer: OS projection subscriptions; canonical suite: OS interaction hooks.
describe("desktop chrome projection subscriptions", () => {
  it("Should ignore geometry and publish window membership and desktop attention", () => {
    const shell = createShell();
    const primary: OsWindow = {
      ...shell.state.windows["window:primary"]!,
      app: "session",
      instanceKey: "session:attention",
    };
    shell.setRuntimeState({ windows: { [primary.id]: primary } });
    const row: OsSessionAttentionRow = {
      kind: "session",
      id: primary.instanceKey!,
      title: "Needs input",
      agentName: "agent",
      workspaceId: "workspace:test",
      workspaceLabel: "Test",
      badge: "waiting-for-input",
      reason: "Waiting for input",
      changedAt: "2026-10-27T00:00:00Z",
      muted: false,
      stale: false,
    };
    let renders = 0;
    const { result, rerender } = renderHook(
      ({ needsYou }: { needsYou: OsSessionAttentionRow[] }) => {
        renders += 1;
        return { chrome: useDesktopShellState(), pager: useDesktopPager(needsYou) };
      },
      { wrapper: shell.wrapper, initialProps: { needsYou: [row] } }
    );
    const initialRenders = renders;
    // Moving within its tab's split changes nothing the chrome renders.
    const moved = { ...primary, nodeId: "leaf:moved" };
    act(() => shell.setRuntimeState({ windows: { [primary.id]: moved } }));
    expect(renders).toBe(initialRenders);
    expect(result.current.chrome.windowIds).toEqual([primary.id]);
    expect([...result.current.pager.needsYouDesktopIds]).toEqual(["desktop:main"]);

    act(() =>
      shell.setRuntimeState({
        windows: { [primary.id]: { ...moved, desktopId: "desktop:other" } },
      })
    );
    expect([...result.current.pager.needsYouDesktopIds]).toEqual(["desktop:other"]);
    rerender({ needsYou: [{ ...row, stale: true }] });
    expect(result.current.pager.needsYouDesktopIds.size).toBe(0);
    act(() => shell.setRuntimeState({ windows: {}, presentation: "compact" }));
    expect(result.current.chrome.windowIds).toEqual([]);
    expect(result.current.chrome.presentation).toBe("compact");
  });

  it("Should suspend closed overview projections and read the latest state when opened", () => {
    const shell = createShell();
    const primary = shell.state.windows["window:primary"]!;
    const desktop = {
      id: "desktop:main",
      name: "Main",
      order: 0,
      tabs: [
        {
          id: primary.tabId,
          pinned: false,
          root: { id: "leaf:window:primary", kind: "leaf" as const, windowId: primary.id },
        },
      ],
      activeTabId: primary.tabId,
    };
    shell.setRuntimeState({ desktops: [desktop], windows: { [primary.id]: primary } });
    let renders = 0;
    const { result, unmount } = renderHook(
      () => {
        renders += 1;
        return useDesktopManagerSurfaces();
      },
      { wrapper: shell.wrapper }
    );
    try {
      const initialRenders = renders;
      act(() =>
        shell.setRuntimeState({
          desktops: [{ ...desktop, name: "Renamed while closed" }],
          windows: { [primary.id]: { ...primary, nodeId: "leaf:moved" } },
        })
      );
      expect(renders).toBe(initialRenders);
      act(() =>
        windowManagerStore.trigger.overlayOpened({ overlay: { kind: "desktops-overview" } })
      );
      expect(result.current.desktops[0]?.name).toBe("Renamed while closed");
      expect(result.current.desktops[0]?.tabs[0]?.members[0]?.nodeId).toBe("leaf:moved");
      act(() => shell.setRuntimeState({ windows: {} }));
      expect(result.current.desktops[0]?.tabs).toEqual([]);
      // A tab whose app this web does not show leaves the card but still moves on close (R2-08).
      expect(result.current.desktops[0]?.movableTabCount).toBe(1);
      act(() =>
        shell.setRuntimeState({ windows: { [primary.id]: { ...primary, app: "new-tab" } } })
      );
      // A lone blank New Tab has nothing to move.
      expect(result.current.desktops[0]?.movableTabCount).toBe(0);
      act(() => windowManagerStore.trigger.overlayClosed());
      const closedRenders = renders;
      act(() => shell.setRuntimeState({ windows: { [primary.id]: primary } }));
      expect(renders).toBe(closedRenders);
      act(() =>
        windowManagerStore.trigger.overlayOpened({ overlay: { kind: "desktops-overview" } })
      );
      expect(result.current.desktops[0]?.tabs[0]?.members[0]?.nodeId).toBe(primary.nodeId);
    } finally {
      unmount();
      act(() => windowManagerStore.trigger.overlayClosed());
    }
  });
});

describe("useCmdPaletteContext store subscriptions", () => {
  it("Should ignore geometry and publish changed command availability", () => {
    vi.useFakeTimers();
    const shell = createShell();
    let renders = 0;
    const { result } = renderHook(
      () => {
        renders += 1;
        return useCmdPaletteContext({
          shellDesktop: true,
          scopeGlobal: false,
          workspaceTrusted: true,
          focusedSessionState: "",
          attached: true,
          debounceMs: 100,
        });
      },
      { wrapper: shell.wrapper }
    );
    const initialRenders = renders;
    const primary = shell.state.windows["window:primary"]!;
    const count = result.current!["desktop.windowCount"] as number;
    act(() =>
      shell.setRuntimeState({
        windows: {
          ...shell.state.windows,
          [primary.id]: { ...primary, nodeId: "leaf:moved" },
        },
      })
    );
    expect(renders).toBe(initialRenders);
    act(() =>
      shell.setRuntimeState({
        windows: {
          ...shell.state.windows,
          // The daemon counts every window on the active desktop, shown or not (v6): leaving
          // the desktop is what changes the count.
          [primary.id]: { ...primary, desktopId: "desktop:elsewhere" },
        },
      })
    );
    act(() => vi.advanceTimersByTime(100));
    expect(result.current!["desktop.windowCount"]).toBe(count - 1);
  });
});

describe("usePaletteTabs", () => {
  afterEach(() => act(() => pruneWindowSlotStores(new Set())));
  it("Should ignore closed-palette topology and title updates and read current tabs on reopen", () => {
    const shell = createShell();
    let renders = 0;
    const { result, rerender } = renderHook(
      ({ open }) => {
        renders += 1;
        return usePaletteTabs(open, null, []);
      },
      { wrapper: shell.wrapper, initialProps: { open: false } }
    );
    const primary = shell.state.windows["window:primary"]!;
    const slot = ensureWindowSlotStore(primary.id);
    const owner = {};
    const initialRenders = renders;
    act(() => slot.publishSlot(owner, { crumb: "Changed while closed" }));
    expect(renders).toBe(initialRenders);
    act(() =>
      shell.setRuntimeState({
        windows: {
          [primary.id]: { ...primary, nodeId: "leaf:moved", tabActive: false, shown: false },
        },
      })
    );
    expect(renders).toBe(initialRenders);
    expect(result.current).toEqual([]);
    rerender({ open: true });
    expect(result.current).toEqual([
      expect.objectContaining({ windowId: primary.id, label: "Changed while closed" }),
    ]);
    act(() => slot.publishSlot(owner, { crumb: "Changed while open" }));
    expect(result.current[0]?.label).toBe("Changed while open");
    act(() => shell.setRuntimeState({ windows: {} }));
    expect(result.current).toEqual([]);
    rerender({ open: false });
    const closedRenders = renders;
    act(() => ensureWindowSlotStore(primary.id).publishSlot(owner, { crumb: "Reopened title" }));
    expect(renders).toBe(closedRenders);
    act(() => shell.setRuntimeState({ windows: { [primary.id]: primary } }));
    expect(renders).toBe(closedRenders);
    rerender({ open: true });
    expect(result.current).toEqual([
      expect.objectContaining({ windowId: primary.id, label: "Reopened title" }),
    ]);
  });
});

describe("useOsWindowMember", () => {
  it("Should retain the app surface on geometry changes and publish member changes", () => {
    const shell = createShell();
    const queryClient = new QueryClient();
    queryClient.setQueryData(workspaceKeys.list(), []);
    queryClient.setQueryData(statusKeys.current(), { daemon: {} });
    const ShellProvider = shell.wrapper;
    let renders = 0;
    const { result, unmount } = renderHook(
      () => {
        renders += 1;
        return useOsWindowMember("window:primary");
      },
      {
        wrapper: ({ children }) => (
          <QueryClientProvider client={queryClient}>
            <ShellProvider>{children}</ShellProvider>
          </QueryClientProvider>
        ),
      }
    );
    const initialRenders = renders;
    const primary = shell.state.windows["window:primary"]!;
    act(() =>
      shell.setRuntimeState({
        windows: {
          ...shell.state.windows,
          [primary.id]: { ...primary, nodeId: "leaf:moved" },
        },
      })
    );
    expect(renders).toBe(initialRenders);
    act(() =>
      shell.setRuntimeState({
        windows: {
          ...shell.state.windows,
          [primary.id]: {
            ...primary,
            app: "settings",
            instanceKey: "settings:secondary",
            inSplit: true,
            shown: false,
          },
        },
      })
    );
    expect(result.current.win).toMatchObject({
      app: "settings",
      instanceKey: "settings:secondary",
      inSplit: true,
      shown: false,
    });
    act(() => shell.setRuntimeState({ windows: {} }));
    expect(result.current.win).toBeUndefined();
    unmount();
    queryClient.clear();
  });
});

describe("useGlobalShortcutReconciliation", () => {
  it("Should preserve native registrations while configuration is pending", async () => {
    const intended: WindowManagerGlobalShortcutMap = {
      "palette.summon.global": "meta+shift+Space",
    };
    const sync = vi.fn().mockResolvedValue([
      {
        command_id: "palette.summon.global",
        intended_chord: "meta+shift+Space",
        active_chord: "meta+shift+Space",
        status: "registered",
      },
    ]);
    window.compozyShell = {
      platform: "darwin",
      on: vi.fn(() => () => undefined),
      globalShortcuts: { sync, status: vi.fn() },
    };

    const { result, rerender } = renderHook(
      ({ config }: { config: WindowManagerGlobalShortcutMap | undefined }) =>
        useGlobalShortcutReconciliation(config),
      {
        initialProps: {
          config: undefined as WindowManagerGlobalShortcutMap | undefined,
        },
      }
    );

    expect(sync).not.toHaveBeenCalled();
    expect(result.current.registrations).toEqual([]);

    rerender({ config: intended });
    await waitFor(() => expect(result.current.registrations[0]?.status).toBe("registered"));
    expect(sync).toHaveBeenCalledExactlyOnceWith([
      { command_id: "palette.summon.global", chord: "meta+shift+Space" },
    ]);
  });

  it("Should replace stale confirmations with explicit unsupported results after a bridge failure", async () => {
    const initial: WindowManagerGlobalShortcutMap = {
      "palette.summon.global": "meta+shift+Space",
    };
    const changed: WindowManagerGlobalShortcutMap = {
      "palette.summon.global": "meta+alt+Space",
    };
    const sync = vi
      .fn()
      .mockResolvedValueOnce([
        {
          command_id: "palette.summon.global",
          intended_chord: "meta+shift+Space",
          active_chord: "meta+shift+Space",
          status: "registered",
        },
      ])
      .mockRejectedValueOnce(new Error("IPC connection closed"));
    window.compozyShell = {
      platform: "darwin",
      on: vi.fn(() => () => undefined),
      globalShortcuts: { sync, status: vi.fn() },
    };
    const warn = vi.spyOn(console, "warn").mockImplementation(() => undefined);

    const { result, rerender } = renderHook(
      ({ intended }: { intended: WindowManagerGlobalShortcutMap }) =>
        useGlobalShortcutReconciliation(intended),
      { initialProps: { intended: initial } }
    );
    await waitFor(() => expect(result.current.registrations[0]?.status).toBe("registered"));

    rerender({ intended: changed });

    await waitFor(() =>
      expect(result.current.registrations).toEqual([
        {
          command_id: "palette.summon.global",
          intended_chord: "meta+alt+Space",
          status: "unsupported",
          reason: "desktop shell synchronization failed",
        },
      ])
    );
    expect(warn).toHaveBeenCalledWith(
      "Desktop shell global hotkey synchronization failed",
      expect.any(Error)
    );
  });
});

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

  it("Should lease live data to the focused and most recent visible background window", () => {
    const shell = createShell();
    const third = windowFixture("window:third");
    const { result } = renderHook(
      () => ({
        primary: useWindowLiveDataEnabled("window:primary"),
        peer: useWindowLiveDataEnabled("window:peer"),
        third: useWindowLiveDataEnabled("window:third"),
      }),
      { wrapper: shell.wrapper }
    );

    expect(result.current).toEqual({ primary: true, peer: true, third: false });
    act(() =>
      shell.setRuntimeState({
        windows: { ...shell.state.windows, [third.id]: third },
        focusedId: third.id,
        client: {
          ...shell.state.client!,
          focusedWindowId: third.id,
          focusOrder: [third.id, "window:primary", "window:peer"],
        },
      })
    );
    expect(result.current).toEqual({ primary: true, peer: false, third: true });
    act(() => shell.setRuntimeState({ activeDesktopId: "desktop:other" }));
    expect(result.current).toEqual({ primary: false, peer: false, third: false });
  });

  it("Should fill the live budget by ID while excluding windows that are not shown", () => {
    const shell = createShell();
    const candidates = [
      windowFixture("window:z", { inSplit: true }),
      windowFixture("window:a", { inSplit: true }),
      windowFixture("window:lower", { inSplit: true }),
      windowFixture("window:collapsed", { inSplit: true, shown: false }),
      windowFixture("window:hidden-tab", { tabActive: false, shown: false }),
      windowFixture("window:other-desktop", { desktopId: "other" }),
    ];
    shell.setRuntimeState({
      windows: Object.fromEntries(candidates.map(window => [window.id, window])),
      focusedId: null,
      client: { ...shell.state.client!, focusOrder: [] },
    });
    const { result } = renderHook(
      () => ({
        a: useWindowLiveDataEnabled("window:a"),
        z: useWindowLiveDataEnabled("window:z"),
        lower: useWindowLiveDataEnabled("window:lower"),
        // Behind a collapsed split's switcher.
        collapsed: useWindowLiveDataEnabled("window:collapsed"),
        hidden: useWindowLiveDataEnabled("window:hidden-tab"),
        other: useWindowLiveDataEnabled("window:other-desktop"),
      }),
      { wrapper: shell.wrapper }
    );
    expect(result.current).toEqual({
      a: true,
      z: false,
      lower: true,
      collapsed: false,
      hidden: false,
      other: false,
    });
    act(() =>
      shell.setRuntimeState({
        focusedId: "window:lower",
        client: { ...shell.state.client!, focusOrder: ["window:hidden-tab", "window:lower"] },
      })
    );
    expect(result.current).toEqual({
      a: true,
      z: false,
      lower: true,
      collapsed: false,
      hidden: false,
      other: false,
    });
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

describe("useOsShortcuts", () => {
  it("Should cycle a frozen visible-session order with wrap and calm 0/1 no-ops [UT-065]", () => {
    const frozen = Object.freeze([{ id: "session-a" }, { id: "session-b" }, { id: "session-c" }]);

    expect(adjacentShortcutItem(frozen, "session-b", "next")?.id).toBe("session-c");
    expect(adjacentShortcutItem(frozen, "session-a", "previous")?.id).toBe("session-c");
    expect(adjacentShortcutItem([], null, "next")).toBeNull();
    expect(adjacentShortcutItem([{ id: "session-a" }], "session-a", "next")).toBeNull();
  });

  it("Should keep an empty attention jump as a calm no-op [UT-074]", () => {
    expect(shortcutAttentionTarget({ needsYou: [], finished: [] })).toBeNull();
  });

  it("Should dispatch the registry command id a chord is bound to", () => {
    const run = vi.fn();
    renderHook(() => useOsShortcuts(shortcutRegistry(), run, { onEscape: vi.fn() }));

    fireEvent.keyDown(document, { key: "k", code: "KeyK", metaKey: true });
    fireEvent.keyDown(document, { key: "n", code: "KeyN", metaKey: true });
    fireEvent.keyDown(document, { key: "o", code: "KeyO", metaKey: true, shiftKey: true });

    expect(run.mock.calls.map(([id]) => id)).toEqual([
      "palette.open",
      "session.new",
      "workspace.picker",
    ]);
  });

  it("Should honour an alternate chord bound to the same command", () => {
    const run = vi.fn();
    renderHook(() => useOsShortcuts(shortcutRegistry(), run, { onEscape: vi.fn() }));

    fireEvent.keyDown(document, { key: "p", code: "KeyP", metaKey: true, shiftKey: true });

    expect(run).toHaveBeenCalledExactlyOnceWith("palette.open");
  });

  it("Should route Escape to the shell and leave a prevented Escape to the nested control", () => {
    const onEscape = vi.fn();
    renderHook(() => useOsShortcuts(shortcutRegistry(), vi.fn(), { onEscape }));

    fireEvent.keyDown(document, { key: "Escape", code: "Escape" });
    expect(onEscape).toHaveBeenCalledOnce();

    const event = new KeyboardEvent("keydown", {
      key: "Escape",
      code: "Escape",
      bubbles: true,
      cancelable: true,
    });
    event.preventDefault();
    document.dispatchEvent(event);
    expect(onEscape).toHaveBeenCalledOnce();
  });

  it("Should protect editor-owned chords while keeping tab lifecycle shortcuts global", () => {
    const run = vi.fn();
    renderHook(() => useOsShortcuts(shortcutRegistry(), run, { onEscape: vi.fn() }));
    const input = document.createElement("input");
    document.body.append(input);

    // Layout undo belongs to the editor while it has focus (WCAG 2.1.4).
    fireEvent.keyDown(input, { key: "z", code: "KeyZ", metaKey: true });
    expect(run).not.toHaveBeenCalled();
    // Window and tab lifecycle stay global — the operator expects ⌘W mid-typing.
    fireEvent.keyDown(input, { key: "w", code: "KeyW", metaKey: true });
    fireEvent.keyDown(input, { key: "k", code: "KeyK", metaKey: true });
    fireEvent.keyDown(input, { key: "n", code: "KeyN", metaKey: true });
    expect(run.mock.calls.map(([id]) => id)).toEqual([
      "window.close",
      "palette.open",
      "session.new",
    ]);

    // The bare `?` cheatsheet chord yields to typing; the ⌘ one does not.
    run.mockClear();
    fireEvent.keyDown(input, { key: "?", code: "Slash", shiftKey: true });
    expect(run).not.toHaveBeenCalled();
    fireEvent.keyDown(input, { key: "/", code: "Slash", metaKey: true });
    expect(run).toHaveBeenCalledExactlyOnceWith("shortcuts.cheatsheet");

    run.mockClear();
    fireEvent.keyDown(document, { key: "z", code: "KeyZ", metaKey: true });
    expect(run).toHaveBeenCalledExactlyOnceWith("layout.undo");
    input.remove();
  });

  it("Should honor live keymap overrides and ignore the shipped chord [UT-051]", () => {
    const run = vi.fn();
    const rebound = shortcutRegistry({ "window.tab.next": ["meta+KeyL"] });
    renderHook(() => useOsShortcuts(rebound, run, { onEscape: vi.fn() }));

    fireEvent.keyDown(document, { key: "Tab", code: "Tab", ctrlKey: true });
    expect(run).not.toHaveBeenCalled();

    fireEvent.keyDown(document, { key: "l", code: "KeyL", metaKey: true });
    expect(run).toHaveBeenCalledExactlyOnceWith("window.tab.next");
  });

  it("Should swallow the chord of an unavailable command instead of acting on it", () => {
    const run = vi.fn();
    const registry = shortcutRegistry({}, { "window.close": false });
    renderHook(() => useOsShortcuts(registry, run, { onEscape: vi.fn() }));

    const event = new KeyboardEvent("keydown", {
      key: "w",
      code: "KeyW",
      metaKey: true,
      bubbles: true,
      cancelable: true,
    });
    document.dispatchEvent(event);

    // Bound but unavailable: the keystroke must not fall through to the browser,
    // and the seam must not run a command the runtime just refused.
    expect(event.defaultPrevented).toBe(true);
    expect(run).not.toHaveBeenCalled();
  });

  it("Should keep an availability-exempt command working while the daemon is away", () => {
    const run = vi.fn();
    const registry = shortcutRegistry({}, { "shortcuts.cheatsheet": false });
    renderHook(() => useOsShortcuts(registry, run, { onEscape: vi.fn() }));

    fireEvent.keyDown(document, { key: "/", code: "Slash", metaKey: true });

    expect(run).toHaveBeenCalledExactlyOnceWith("shortcuts.cheatsheet");
  });

  it("Should dispatch the primary shortcut with Control on non-Apple platforms", () => {
    vi.spyOn(window.navigator, "platform", "get").mockReturnValue("Linux x86_64");
    const run = vi.fn();
    renderHook(() => useOsShortcuts(shortcutRegistry(), run, { onEscape: vi.fn() }));

    fireEvent.keyDown(document, { key: "t", code: "KeyT", ctrlKey: true });
    fireEvent.keyDown(document, { key: "[", code: "BracketLeft", ctrlKey: true });

    expect(run.mock.calls.map(([id]) => id)).toEqual(["window.tab.new", "window.nav.back"]);
  });

  it("Should leave editable descendants and repeated chords untouched [UT-099]", () => {
    const run = vi.fn();
    renderHook(() => useOsShortcuts(shortcutRegistry(), run, { onEscape: vi.fn() }));
    const editable = document.createElement("div");
    editable.setAttribute("contenteditable", "true");
    const nestedTarget = document.createElement("span");
    editable.append(nestedTarget);
    document.body.append(editable);

    fireEvent.keyDown(nestedTarget, { key: "z", code: "KeyZ", metaKey: true });
    fireEvent.keyDown(document, { key: "z", code: "KeyZ", metaKey: true, repeat: true });

    expect(run).not.toHaveBeenCalled();
    editable.remove();
  });

  it("Should dispatch nothing while disabled, so first-run setup blocks the whole set", () => {
    const run = vi.fn();
    const onEscape = vi.fn();
    renderHook(() => useOsShortcuts(shortcutRegistry(), run, { enabled: false, onEscape }));

    fireEvent.keyDown(document, { key: "k", code: "KeyK", metaKey: true });
    fireEvent.keyDown(document, { key: "z", code: "KeyZ", metaKey: true });
    fireEvent.keyDown(document, { key: "Escape", code: "Escape" });

    expect(run).not.toHaveBeenCalled();
    expect(onEscape).not.toHaveBeenCalled();
  });
});

describe("useOsPaletteExecution keyboard dispatch [UT-129]", () => {
  const contentRef = { current: null } as RefObject<HTMLElement | null>;

  function renderExecution(
    options: {
      registry?: PaletteRegistry;
      selected?: PaletteRowSubject | null;
      open?: boolean;
    } = {}
  ) {
    const runAction = vi.fn();
    const runCommand = vi.fn();
    const registry = options.registry ?? shortcutRegistry();
    const selected =
      options.selected === undefined
        ? ({
            kind: "command",
            command: registry.byId.get("window.tab.new") as ResolvedPaletteCommand,
          } as PaletteRowSubject)
        : options.selected;
    const view = renderHook(() =>
      useOsPaletteExecution({
        open: options.open ?? true,
        registry,
        pins: [],
        selected,
        contentRef,
        onPanelOpen: vi.fn(),
        runAction,
        runCommand,
      })
    );
    return { ...view, runAction, runCommand };
  }

  function press(init: KeyboardEventInit): KeyboardEvent {
    const event = new KeyboardEvent("keydown", { bubbles: true, cancelable: true, ...init });
    // The listener drives React state, so the dispatch has to be flushed before
    // the assertion reads the hook back.
    act(() => {
      document.dispatchEvent(event);
    });
    return event;
  }

  it("Should toggle the action panel on the palette chord instead of letting it close the palette", () => {
    const { result } = renderExecution();

    const opened = press({ key: "k", code: "KeyK", metaKey: true });
    expect(result.current.panel.open).toBe(true);
    expect(opened.defaultPrevented).toBe(true);

    press({ key: "k", code: "KeyK", metaKey: true });
    expect(result.current.panel.open).toBe(false);
  });

  it("Should fire the selected row's action chord wherever focus drifted in the palette", () => {
    // The selected row is `window.tab.new`, whose primary action carries that
    // command's own chord — pressing it acts on the row, not on the shell.
    const { result, runAction } = renderExecution();
    const fired = press({ key: "t", code: "KeyT", metaKey: true });

    expect(runAction).toHaveBeenCalledOnce();
    expect(runAction.mock.calls[0]?.[0]).toMatchObject({
      primary: true,
      intent: { kind: "run-command", commandId: "window.tab.new" },
    });
    expect(fired.defaultPrevented).toBe(true);
    expect(result.current.panel.open).toBe(false);
  });

  it("Should ignore a held chord so one press cannot fire an action twice", () => {
    const { runAction } = renderExecution();

    press({ key: "t", code: "KeyT", metaKey: true, repeat: true });
    expect(runAction).not.toHaveBeenCalled();

    press({ key: "k", code: "KeyK", metaKey: true, repeat: true });
    expect(runAction).not.toHaveBeenCalled();
  });

  it("Should leave a chord that belongs to no action on the selected row alone", () => {
    // `window.nav.back` is bound in the keymap but is not one of this row's
    // actions, so the shell's own listener must still get the keystroke.
    const { runAction } = renderExecution();
    const untouched = press({ key: "[", code: "BracketLeft", metaKey: true });

    expect(runAction).not.toHaveBeenCalled();
    expect(untouched.defaultPrevented).toBe(false);
  });

  it("Should listen only while the palette is open", () => {
    const { result, runAction } = renderExecution({ open: false });

    press({ key: "k", code: "KeyK", metaKey: true });
    press({ key: "t", code: "KeyT", metaKey: true });

    expect(result.current.panel.open).toBe(false);
    expect(runAction).not.toHaveBeenCalled();
  });

  it("Should have nothing to act on when no row is selected", () => {
    const { result, runAction } = renderExecution({ selected: null });

    press({ key: "k", code: "KeyK", metaKey: true });
    press({ key: "t", code: "KeyT", metaKey: true });

    expect(result.current.panel.open).toBe(false);
    expect(result.current.panel.model).toBeNull();
    expect(runAction).not.toHaveBeenCalled();
  });
});

function DeskLayerHarness() {
  const { layerRef } = useOsDeskLayer();
  return <div ref={layerRef} data-testid="desk-layer" />;
}

describe("useOsDeskLayer", () => {
  it("Should wait for layout configuration before exposing restored desktops", () => {
    const shell = createShell();
    const desktop = {
      id: "desktop:main",
      name: "Main",
      order: 0,
      tabs: [
        {
          id: "tab:window:primary",
          pinned: false,
          root: { id: "leaf:window:primary", kind: "leaf" as const, windowId: "window:primary" },
        },
      ],
      activeTabId: "tab:window:primary",
    };
    shell.setRuntimeState({
      desktops: [desktop],
      windowManagerConfig: null,
      hydration: "pending",
      activeTabIds: {},
    });
    const { result } = renderHook(() => useOsDeskLayer(), { wrapper: shell.wrapper });

    expect(result.current.desktops).toEqual([]);

    act(() => {
      shell.setRuntimeState({
        windowManagerConfig: CONFIG,
        hydration: "live",
        activeTabIds: { [desktop.id]: "tab:window:primary" },
      });
    });

    expect(result.current.desktops).toEqual([
      { desktop, active: true, activeTabId: "tab:window:primary" },
    ]);
  });

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
    render(<DeskLayerHarness />, { wrapper: shell.wrapper });
    const layer = screen.getByTestId("desk-layer");
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
      viewportWidth: window.innerWidth,
      width: 1280,
      height: 700,
      origin: { x: 24, y: 52 },
    });
  });
});

// Invariant: a desktop keeps every tab it has shown mounted, in bar order, and drops closed ones.
// Owning layer: the desk layer's tab retention; canonical suite: OS interaction hooks.
describe("useVisitedTabs", () => {
  it("Should keep visited tabs in bar order and forget closed tabs", () => {
    const { result, rerender } = renderHook(
      ({ tabIds, active }: { tabIds: string[]; active: string | null }) =>
        useVisitedTabs(tabIds, active),
      { initialProps: { tabIds: ["a", "b", "c"], active: "b" as string | null } }
    );
    expect(result.current).toEqual(["b"]);

    rerender({ tabIds: ["a", "b", "c"], active: "a" });
    expect(result.current).toEqual(["a", "b"]);

    rerender({ tabIds: ["a", "c"], active: "c" });
    expect(result.current).toEqual(["a", "c"]);
  });
});

describe("useDesktopCardDrag", () => {
  /** A grid of three 100px cards in a row, a at 0, b at 100, c at 200. */
  function cardGrid(order: readonly string[]) {
    const grid = document.createElement("div");
    grid.dataset.slot = "desktops-overview-grid";
    grid.getBoundingClientRect = () => new DOMRect(0, 0, 300, 100);
    const cards = new Map<string, HTMLElement>();
    order.forEach((id, position) => {
      const card = document.createElement("div");
      card.dataset.slot = "desktop-card";
      card.dataset.cardDesktopId = id;
      card.getBoundingClientRect = () => new DOMRect(position * 100, 0, 100, 100);
      grid.append(card);
      cards.set(id, card);
    });
    return cards;
  }

  function pointer(type: "pointermove" | "pointerup", clientX: number) {
    const event = new MouseEvent(type, { bubbles: true, clientX, clientY: 50 });
    Object.defineProperty(event, "pointerId", { value: 1 });
    act(() => {
      window.dispatchEvent(event);
    });
  }

  function press(
    start: (id: string, event: ReactPointerEvent<HTMLElement>) => void,
    card: HTMLElement
  ) {
    act(() =>
      start(card.dataset.cardDesktopId!, {
        button: 0,
        pointerId: 1,
        clientX: 50,
        clientY: 50,
        currentTarget: card,
        target: card,
      } as unknown as ReactPointerEvent<HTMLElement>)
    );
  }

  it("Should preview the landing gap and send one reorder on release", () => {
    const onReorder = vi.fn();
    const { result } = renderHook(() => useDesktopCardDrag(["a", "b", "c"], onReorder));
    const cards = cardGrid(["a", "b", "c"]);

    press(result.current.start, cards.get("a")!);
    pointer("pointermove", 52);
    expect(result.current.drag).toBeNull();
    pointer("pointermove", 250);
    expect(result.current.drag).toMatchObject({ desktopId: "a", index: 2, width: 100 });
    expect(result.current.previewOrder).toEqual(["b", "c", "a"]);
    pointer("pointerup", 250);

    expect(onReorder).toHaveBeenCalledExactlyOnceWith("a", 2);
    expect(result.current.drag).toBeNull();
    expect(result.current.previewOrder).toEqual(["a", "b", "c"]);
  });

  it.each([
    { gesture: "drag", travelTo: 250 },
    { gesture: "press below the drag threshold", travelTo: 52 },
  ])(
    "Should cancel a $gesture on Escape without sending a reorder or closing the overview",
    ({ travelTo }) => {
      const onReorder = vi.fn();
      const { result } = renderHook(() => useDesktopCardDrag(["a", "b", "c"], onReorder));
      const cards = cardGrid(["a", "b", "c"]);
      // Where the overview's dismissal listens: past the window, Escape must not arrive.
      const overviewEscape = vi.fn();
      document.addEventListener("keydown", overviewEscape);

      try {
        press(result.current.start, cards.get("a")!);
        pointer("pointermove", travelTo);
        const escape = new KeyboardEvent("keydown", {
          key: "Escape",
          bubbles: true,
          cancelable: true,
        });
        act(() => {
          document.body.dispatchEvent(escape);
        });

        expect(escape.defaultPrevented).toBe(true);
        expect(overviewEscape).not.toHaveBeenCalled();
        expect(result.current.drag).toBeNull();
        pointer("pointermove", 250);
        pointer("pointerup", 250);
        expect(result.current.drag).toBeNull();
        expect(onReorder).not.toHaveBeenCalled();
      } finally {
        document.removeEventListener("keydown", overviewEscape);
      }
    }
  );

  it("Should cancel when the desktop order changes under the drag", () => {
    const onReorder = vi.fn();
    const { result, rerender } = renderHook(
      ({ order }: { order: readonly string[] }) => useDesktopCardDrag(order, onReorder),
      { initialProps: { order: ["a", "b", "c"] as readonly string[] } }
    );
    const cards = cardGrid(["a", "b", "c"]);

    press(result.current.start, cards.get("a")!);
    pointer("pointermove", 250);
    expect(result.current.drag).not.toBeNull();
    rerender({ order: ["b", "a", "c"] });

    expect(result.current.drag).toBeNull();
    expect(result.current.previewOrder).toEqual(["b", "a", "c"]);
    pointer("pointermove", 250);
    pointer("pointerup", 250);
    expect(result.current.drag).toBeNull();
    expect(onReorder).not.toHaveBeenCalled();
  });
});

function leafNode(windowId: string): LayoutNode {
  return { id: `leaf:${windowId}`, kind: "leaf", windowId };
}

function desktopWith(tabs: Array<{ id: string; root: LayoutNode }>) {
  return {
    id: "desktop:main",
    name: "Main",
    order: 0,
    tabs: tabs.map(tab => ({ ...tab, pinned: false })),
    activeTabId: tabs[0]!.id,
  };
}

/** jsdom has no pointer capture: the element records it instead. */
function stubPointerCapture(element: HTMLElement) {
  const captured = new Set<number>();
  element.setPointerCapture = vi.fn((pointerId: number) => void captured.add(pointerId));
  element.hasPointerCapture = vi.fn((pointerId: number) => captured.has(pointerId));
  element.releasePointerCapture = vi.fn((pointerId: number) => void captured.delete(pointerId));
  return element;
}

function TabBarDragHarness({ tabIds }: { tabIds: string[] }) {
  const { startTabDrag } = useOsTabDragStart();
  return (
    <div data-slot="os-tab-bar" data-testid="bar">
      {tabIds.map(tabId => (
        <button
          key={tabId}
          data-slot="os-tab"
          data-tab-id={tabId}
          type="button"
          onPointerDown={event => startTabDrag(tabId, event)}
        >
          {tabId}
        </button>
      ))}
    </div>
  );
}

function HeadDragHarness({ windowId, onBack }: { windowId: string; onBack: () => void }) {
  const { startHeadDrag } = useOsTabDragStart();
  return (
    <header data-testid="head" onPointerDown={event => startHeadDrag(windowId, event)}>
      <button type="button" onClick={onBack}>
        Back
      </button>
    </header>
  );
}

// Invariant: a tab or window-head drag sends its one command on release and always tears down,
// even when that command throws; a press that stays below the drag threshold, or starts on a
// control inside the head, is a plain click on that control.
// Owning layer: the pointer-to-drag-store wiring in useOsTabDragStart; the gesture rules
// themselves are owned by the drag store suite.
describe("useOsTabDragStart pointer gestures", () => {
  afterEach(() => {
    act(() => {
      osTabDragStore.trigger.cancelled();
      osTabDragStore.trigger.settled();
    });
  });

  it("Should dispatch the reorder a released tab drag emits, fenced to the press revision", async () => {
    const shell = createShell();
    const windows = Object.fromEntries(
      ["a", "b", "c"].map(name => [
        `window:${name}`,
        windowFixture(`window:${name}`, { tabId: `tab:${name}`, nodeId: `leaf:window:${name}` }),
      ])
    );
    shell.setRuntimeState({
      windows,
      desktops: [
        desktopWith(
          ["a", "b", "c"].map(name => ({ id: `tab:${name}`, root: leafNode(`window:${name}`) }))
        ),
      ],
      activeTabIds: { "desktop:main": "tab:a" },
    });
    render(<TabBarDragHarness tabIds={["tab:a", "tab:b", "tab:c"]} />, { wrapper: shell.wrapper });
    screen.getByTestId("bar").getBoundingClientRect = () => new DOMRect(0, 0, 300, 30);
    const tabs = ["tab:a", "tab:b", "tab:c"].map((tabId, index) => {
      const tab = stubPointerCapture(screen.getByRole("button", { name: tabId }));
      tab.getBoundingClientRect = () => new DOMRect(index * 100, 0, 100, 30);
      return tab;
    });

    fireEvent.pointerDown(tabs[0]!, { button: 0, pointerId: 1, clientX: 50, clientY: 15 });
    fireEvent.pointerMove(window, { pointerId: 1, clientX: 260, clientY: 15 });
    expect(osTabDragStore.getSnapshot().context.phase).toBe("dragging");
    fireEvent.pointerUp(window, { pointerId: 1, clientX: 260, clientY: 15 });

    // Moved past the last tab: the tab lands at the end of the bar, in one command.
    expect(shell.controller.reorderTab).toHaveBeenCalledExactlyOnceWith("window:a", 2, {
      expectedRevision: SNAPSHOT.revision,
      rebase: { windowId: "window:a" },
    });
    expect(osTabDragStore.getSnapshot().context.phase).toBe("idle");
    expect(tabs[0]!.setPointerCapture).toHaveBeenCalledOnce();
    // The reorder settled, so the bar shows the daemon's order again.
    await waitFor(() => expect(osTabDragStore.getSnapshot().context.settling).toBeNull());
  });

  it("Should not capture a head press that never becomes a drag", () => {
    const shell = renderSplitHead();
    const head = stubPointerCapture(screen.getByTestId("head"));

    fireEvent.pointerDown(head, { button: 0, pointerId: 3, clientX: 10, clientY: 10 });
    expect(osTabDragStore.getSnapshot().context.phase).toBe("pressed");
    fireEvent.pointerUp(window, { pointerId: 3, clientX: 12, clientY: 10 });

    // Capturing at press would route the click to the head instead of what is under it.
    expect(head.setPointerCapture).not.toHaveBeenCalled();
    expect(shell.controller.moveWindow).not.toHaveBeenCalled();
    expect(osTabDragStore.getSnapshot().context.phase).toBe("idle");
  });

  it("Should never arm a drag from a control inside the head, even when the press moves", () => {
    const shell = renderSplitHead();
    const head = stubPointerCapture(screen.getByTestId("head"));
    const back = screen.getByRole("button", { name: "Back" });

    fireEvent.pointerDown(back, { button: 0, pointerId: 3, clientX: 10, clientY: 10 });
    expect(osTabDragStore.getSnapshot().context.phase).toBe("idle");
    fireEvent.pointerMove(window, { pointerId: 3, clientX: 40, clientY: 10 });
    fireEvent.pointerUp(window, { pointerId: 3, clientX: 40, clientY: 10 });
    fireEvent.click(back);

    expect(onHeadBack).toHaveBeenCalledOnce();
    expect(head.setPointerCapture).not.toHaveBeenCalled();
    expect(shell.controller.moveWindow).not.toHaveBeenCalled();
  });

  it("Should leave the next gesture clean when a drop's dispatch throws", () => {
    const { shell, tabs } = renderTabBar();
    vi.mocked(shell.controller.reorderTab).mockImplementationOnce(() => {
      throw new Error("dispatch failed");
    });
    // jsdom reports a listener's exception as a window error; the test owns that report.
    const onError = (event: ErrorEvent) => event.preventDefault();
    window.addEventListener("error", onError);

    fireEvent.pointerDown(tabs[0]!, { button: 0, pointerId: 1, clientX: 50, clientY: 15 });
    fireEvent.pointerMove(window, { pointerId: 1, clientX: 260, clientY: 15 });
    expect(document.documentElement.style.userSelect).toBe("none");
    fireEvent.pointerUp(window, { pointerId: 1, clientX: 260, clientY: 15 });
    window.removeEventListener("error", onError);

    expect(shell.controller.reorderTab).toHaveBeenCalledOnce();
    expect(document.documentElement.style.userSelect).toBe("");
    expect(tabs[0]!.hasPointerCapture(1)).toBe(false);
    expect(osTabDragStore.getSnapshot().context).toMatchObject({ phase: "idle", settling: null });

    // The listeners of the failed gesture are gone: one more drag sends exactly one more command.
    fireEvent.pointerDown(tabs[1]!, { button: 0, pointerId: 2, clientX: 150, clientY: 15 });
    fireEvent.pointerMove(window, { pointerId: 2, clientX: 10, clientY: 15 });
    fireEvent.pointerUp(window, { pointerId: 2, clientX: 10, clientY: 15 });

    expect(shell.controller.reorderTab).toHaveBeenCalledTimes(2);
    expect(shell.controller.reorderTab).toHaveBeenLastCalledWith("window:b", 0, {
      expectedRevision: SNAPSHOT.revision,
      rebase: { windowId: "window:b" },
    });
  });

  const onHeadBack = vi.fn();

  /** One split tab whose primary window's head is rendered with a Back control. */
  function renderSplitHead() {
    onHeadBack.mockClear();
    const shell = createShell();
    shell.setRuntimeState({
      desktops: [
        desktopWith([
          {
            id: "tab:main",
            root: {
              id: "split:main",
              kind: "split",
              axis: "horizontal",
              weights: [0.5, 0.5],
              children: [leafNode("window:primary"), leafNode("window:peer")],
            },
          },
        ]),
      ],
      activeTabIds: { "desktop:main": "tab:main" },
    });
    render(<HeadDragHarness windowId="window:primary" onBack={onHeadBack} />, {
      wrapper: shell.wrapper,
    });
    return shell;
  }

  /** Three single-window tabs on a 300px bar, each 100px wide. */
  function renderTabBar() {
    const shell = createShell();
    const windows = Object.fromEntries(
      ["a", "b", "c"].map(name => [
        `window:${name}`,
        windowFixture(`window:${name}`, { tabId: `tab:${name}`, nodeId: `leaf:window:${name}` }),
      ])
    );
    shell.setRuntimeState({
      windows,
      desktops: [
        desktopWith(
          ["a", "b", "c"].map(name => ({ id: `tab:${name}`, root: leafNode(`window:${name}`) }))
        ),
      ],
      activeTabIds: { "desktop:main": "tab:a" },
    });
    render(<TabBarDragHarness tabIds={["tab:a", "tab:b", "tab:c"]} />, { wrapper: shell.wrapper });
    screen.getByTestId("bar").getBoundingClientRect = () => new DOMRect(0, 0, 300, 30);
    const tabs = ["tab:a", "tab:b", "tab:c"].map((tabId, index) => {
      const tab = stubPointerCapture(screen.getByRole("button", { name: tabId }));
      tab.getBoundingClientRect = () => new DOMRect(index * 100, 0, 100, 30);
      return tab;
    });
    return { shell, tabs };
  }
});

const SEAM: ProjectedSeam = {
  id: "split:main:0",
  splitId: "split:main",
  boundaryIndex: 0,
  orientation: "vertical",
  rect: { x: 496, y: 0, w: 8, h: 700 },
  value: 500,
  minValue: 300,
  maxValue: 700,
  axisSpan: 1000,
  leadingWeight: 0.5,
  trailingWeight: 0.5,
  leadingWindowIds: ["window:primary"],
  trailingWindowIds: ["window:peer"],
};

function SeamHarness({
  seam,
  memberCount = 2,
}: {
  seam: ProjectedSeam | undefined;
  memberCount?: number;
}) {
  const { state, onPointerDown, onKeyDown, onDoubleClick } = useOsSplitSeam(seam, memberCount);
  return (
    <div
      data-state={state}
      data-testid="seam"
      tabIndex={0}
      onDoubleClick={onDoubleClick}
      onKeyDown={onKeyDown}
      onPointerDown={onPointerDown}
    />
  );
}

/** The authoritative snapshot with one split tab whose split carries these weights. */
function snapshotWithSplit(weights: number[], revision = SNAPSHOT.revision): WindowManagerSnapshot {
  return {
    ...SNAPSHOT,
    revision,
    desktops: [
      desktopWith([
        {
          id: "tab:main",
          root: {
            id: "split:main",
            kind: "split",
            axis: "horizontal",
            weights,
            children: [leafNode("window:primary"), leafNode("window:peer")],
          },
        },
      ]),
    ],
  };
}

// Invariant: a seam drag commits one `layout.resize` fenced to the revision and split it started
// from; Escape, a cancelled or lost pointer, unmounting and a change of the context it was drawn
// in — including the split's weights — (US-009.EC-2) cancel it, restore the shares and send
// nothing. Keyboard and pair-balance resizes carry the same fence.
// Owning layer: the seam gesture hook over the shared seam drag store (hooks/seam-drag-store).
describe("useOsSplitSeam gesture lifecycle", () => {
  let frames: FrameRequestCallback[] = [];

  function flushFrames() {
    const pending = frames;
    frames = [];
    act(() => {
      for (const frame of pending) frame(16);
    });
  }

  function startSeamDrag(snapshot: WindowManagerSnapshot = SNAPSHOT) {
    const shell = createShell();
    shell.setRuntimeState({ snapshot, activeTabIds: { "desktop:main": "tab:main" } });
    frames = [];
    vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => {
      frames.push(callback);
      return frames.length;
    });
    vi.stubGlobal("cancelAnimationFrame", vi.fn());
    const { unmount } = render(<SeamHarness seam={SEAM} />, { wrapper: shell.wrapper });
    const seam = stubPointerCapture(screen.getByTestId("seam"));
    fireEvent.pointerDown(seam, { button: 0, pointerId: 4, clientX: 500, clientY: 300 });
    fireEvent.pointerMove(seam, { pointerId: 4, clientX: 560, clientY: 300 });
    flushFrames();
    expect(windowManagerStore.getSnapshot().context.seamPreview).toEqual({
      splitId: "split:main",
      boundaryIndex: 0,
      deltaPx: 60,
    });
    expect(seam).toHaveAttribute("data-state", "drag");
    return { shell, seam, unmount };
  }

  /** The drag ended without a commit: the shares are back and a late release sends nothing. */
  function expectCancelled(shell: ReturnType<typeof createShell>, seam: HTMLElement) {
    fireEvent.pointerUp(seam, { pointerId: 4, clientX: 560, clientY: 300 });
    expect(windowManagerStore.getSnapshot().context.seamPreview).toBeNull();
    expect(shell.controller.resizeLayout).not.toHaveBeenCalled();
  }

  afterEach(() => {
    act(() => windowManagerStore.trigger.seamPreviewCleared());
  });

  it("Should cancel on Escape: the shares come back and nothing is sent", () => {
    const { shell, seam } = startSeamDrag();

    fireEvent.keyDown(window, { key: "Escape" });
    fireEvent.pointerUp(seam, { pointerId: 4, clientX: 560, clientY: 300 });

    expect(windowManagerStore.getSnapshot().context.seamPreview).toBeNull();
    expect(seam).not.toHaveAttribute("data-state");
    expect(shell.controller.resizeLayout).not.toHaveBeenCalled();
  });

  it("Should cancel when another tab is shown mid-drag", () => {
    const { shell, seam } = startSeamDrag();

    act(() => shell.setRuntimeState({ activeTabIds: { "desktop:main": "tab:other" } }));
    fireEvent.pointerUp(seam, { pointerId: 4, clientX: 560, clientY: 300 });

    expect(windowManagerStore.getSnapshot().context.seamPreview).toBeNull();
    expect(shell.controller.resizeLayout).not.toHaveBeenCalled();
  });

  it("Should commit one resize fenced to the revision the drag started from", async () => {
    const { shell, seam } = startSeamDrag();

    // Another client changes the layout mid-drag: the commit still names the press revision and
    // the seam's split, so the daemon applies it only while that split is still there.
    act(() => shell.setRuntimeState({ snapshot: { ...SNAPSHOT, revision: 9 } }));
    fireEvent.pointerUp(seam, { pointerId: 4, clientX: 560, clientY: 300 });

    expect(shell.controller.resizeLayout).toHaveBeenCalledExactlyOnceWith(
      "split:main",
      0,
      expect.closeTo(0.06),
      { expectedRevision: SNAPSHOT.revision, rebase: { splitId: "split:main", boundaryIndex: 0 } }
    );
    await waitFor(() => expect(windowManagerStore.getSnapshot().context.seamPreview).toBeNull());
  });
  it("Should cancel when the pointer is cancelled", () => {
    const { shell, seam } = startSeamDrag();

    fireEvent.pointerCancel(seam, { pointerId: 4 });

    expectCancelled(shell, seam);
  });

  it("Should cancel when the seam loses pointer capture", () => {
    const { shell, seam } = startSeamDrag();

    fireEvent(seam, new Event("lostpointercapture", { bubbles: true }));

    expectCancelled(shell, seam);
  });

  it("Should cancel when the seam unmounts mid-drag", () => {
    const { shell, seam, unmount } = startSeamDrag();

    unmount();

    expectCancelled(shell, seam);
  });

  it("Should cancel when someone else changes the split's weights mid-drag", () => {
    const { shell, seam } = startSeamDrag(snapshotWithSplit([0.5, 0.5]));

    // The preview showed 50/50 moving; committing its delta onto 30/70 would land elsewhere.
    act(() => shell.setRuntimeState({ snapshot: snapshotWithSplit([0.3, 0.7], 8) }));

    expectCancelled(shell, seam);
  });

  it("Should fence an arrow-key resize and a pair balance like a drag", () => {
    const shell = createShell();
    const wide: ProjectedSeam = { ...SEAM, leadingWeight: 0.3, trailingWeight: 0.5 };
    render(<SeamHarness memberCount={3} seam={wide} />, { wrapper: shell.wrapper });
    const seam = screen.getByTestId("seam");
    const fence = {
      expectedRevision: SNAPSHOT.revision,
      rebase: { splitId: "split:main", boundaryIndex: 0 },
    };

    fireEvent.keyDown(seam, { key: "ArrowRight" });
    fireEvent.doubleClick(seam);

    expect(shell.controller.resizeLayout).toHaveBeenNthCalledWith(
      1,
      "split:main",
      0,
      expect.closeTo(0.02),
      fence
    );
    // A three-member split evens only this pair: 0.3 and 0.5 become 0.4 each.
    expect(shell.controller.resizeLayout).toHaveBeenNthCalledWith(
      2,
      "split:main",
      0,
      expect.closeTo(0.1),
      fence
    );
    expect(shell.controller.balanceLayout).not.toHaveBeenCalled();
  });
});
