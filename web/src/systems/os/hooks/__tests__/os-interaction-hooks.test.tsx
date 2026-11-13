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
  useRef,
} from "react";
import { createPortal } from "react-dom";
import { afterEach, describe, expect, it, onTestFinished, vi } from "vitest";

import { OsShellContext, type OsShellHandle } from "../../contexts/os-shell-context";
import { RoutingCoordinator } from "../../lib/routing-coordinator";
import type {
  OsDesktopRuntimeStore,
  OsWindow,
  OsWindowRoute,
  WindowManagerController,
  WindowManagerCommandOutcome,
} from "../../lib/os-types";
import type {
  LayoutNode,
  LayoutTab,
  ProjectedSeam,
  WindowManagerConfig,
  WindowManagerSnapshot,
} from "../../lib/window-manager-types";
import type { WindowManagerGlobalShortcutMap } from "../../lib/window-manager-shortcut-types";
import { SCROLL_OWNER_ATTRIBUTE, SCROLL_RESTORE_EVENT } from "@/lib/scroll-owner";
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
import { KeyboardFocusHandoff } from "../../lib/keyboard-focus-handoff";
import { WindowPaneHosts } from "../../lib/window-pane-hosts";
import { useOsShortcuts } from "../use-os-shortcuts";
import { WINDOW_MANAGER_COMMAND_DEADLINE_MS } from "../../runtime/window-manager-runtime-helpers";
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
import { TAB_DRAG_EXIT_PX } from "../../lib/drop-edge";
import { createWindowManagerProjectionAtom } from "../../lib/window-manager-projection";
import { applySeamPreviewToTab, seamWeightDelta } from "../../lib/seam-preview";
import { admitSeamShift, projectTab } from "../../lib/tab-projection";
import { authoritativeSeam, buildWindowManagerMinimums } from "../../lib/window-manager-view";
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
    balanceLayout: vi.fn(() => acceptedOutcome()),
    balanceTab: vi.fn(),
    commandsInFlight: vi.fn(() => false),
    commandsSettled: vi.fn(async () => {}),
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
    swapDirection: vi.fn(),
    directionalFocusId: vi.fn(() => null),
    directionalCandidates: vi.fn(() => []),
    undoLayout: vi.fn(),
    redoLayout: vi.fn(),
    balanceFocusedLayout: vi.fn(),
    clearConflict: vi.fn(),
    refreshSnapshot: vi.fn(async () => true),
  };
  const history = { navigate: vi.fn(), replace: vi.fn() };
  const coordinator = new RoutingCoordinator(controller, history);
  const shell: OsShellHandle = { projection, manager: controller, coordinator };
  const wrapper = ({ children }: { children: ReactNode }) => (
    <OsShellContext.Provider value={shell}>{children}</OsShellContext.Provider>
  );
  return {
    controller,
    coordinator,
    history,
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

  // A shown window moved into a new tab that is not shown keeps its pane mounted (#1654).
  it("Should keep a tab that received a mounted window, though it was never shown", () => {
    const { result, rerender } = renderHook(
      ({ tabIds, adopted }: { tabIds: string[]; adopted: string[] }) =>
        useVisitedTabs(tabIds, "a", adopted),
      { initialProps: { tabIds: ["a", "b"], adopted: [] as string[] } }
    );
    expect(result.current).toEqual(["a"]);

    rerender({ tabIds: ["a", "b"], adopted: ["b"] });
    expect(result.current).toEqual(["a", "b"]);

    // Kept once adopted, as a shown tab is; closing it still drops it.
    rerender({ tabIds: ["a", "b"], adopted: [] });
    expect(result.current).toEqual(["a", "b"]);
    rerender({ tabIds: ["a"], adopted: [] });
    expect(result.current).toEqual(["a"]);
  });
});

// Invariant: a pane keeps the scroll offsets of its scroll containers and its focused element
// across a slot change and across being hidden and shown, unless focus was given elsewhere; DOM
// focus returns only into the window that holds the client's authoritative focus, so a restore
// never reads as the operator choosing a window [AS-F-20, R13-02].
// Owning layer: the pane hosts the desk layer's slots place; canonical suite: OS interaction hooks.
// jsdom does not lay out or reset offsets: each case removes what the browser would lose and
// asserts what the hosts write back.
describe("WindowPaneHosts", () => {
  function setup(handoff: KeyboardFocusHandoff | null = null) {
    /** Every live resize observer and what it watches. */
    const observers = new Map<
      ResizeObserver,
      { callback: ResizeObserverCallback; on: Set<Element> }
    >();
    /** What any live resize observer watches. */
    const sized = new Set<Element>();
    class ResizeObserverMock implements ResizeObserver {
      constructor(callback: ResizeObserverCallback) {
        observers.set(this, { callback, on: new Set() });
      }

      observe(target: Element) {
        observers.get(this)?.on.add(target);
        sized.add(target);
      }
      unobserve(target: Element) {
        observers.get(this)?.on.delete(target);
        sized.delete(target);
      }
      disconnect() {
        for (const target of observers.get(this)?.on ?? []) sized.delete(target);
        observers.get(this)?.on.clear();
      }
    }
    vi.stubGlobal("ResizeObserver", ResizeObserverMock);
    const focus = { windowId: "w" };
    const content = { route: { pathname: "/files", search: {} } as OsWindowRoute };
    const hosts = new WindowPaneHosts(
      {
        isFocused: windowId => windowId === focus.windowId,
        routeOf: () => content.route,
      },
      handoff
    );
    const host = hosts.hostOf("w");
    const surface = document.createElement("section");
    const scroller = document.createElement("div");
    const field = document.createElement("textarea");
    surface.append(scroller, field);
    host.append(surface);
    const [first, second] = [document.createElement("div"), document.createElement("div")];
    document.body.append(first, second);
    const scrollTo = (top: number) => {
      scroller.scrollTop = top;
      scroller.dispatchEvent(new Event("scroll"));
    };
    /** What the browser loses when the subtree leaves the document or stops rendering. */
    const loseLayoutState = () => {
      scroller.scrollTop = 0;
      if (document.activeElement === field) field.blur();
    };
    /**
     * The tab stops showing. The browser drops the offsets and, by its focus fixup, takes focus
     * from a focused field inside with a focusout that fires once the surface no longer renders.
     * jsdom's `blur()` is a no-op under `hidden`, so the surface is hidden from the first
     * listener of that focusout (window, capture phase), before the hosts read it.
     */
    const stopRendering = () => {
      scroller.scrollTop = 0;
      if (document.activeElement !== field) {
        surface.hidden = true;
        return;
      }
      window.addEventListener(
        "focusout",
        () => {
          surface.hidden = true;
        },
        { capture: true, once: true }
      );
      field.blur();
    };
    /** The browser reports `target`'s new size to the observers watching it. */
    const resize = (target: Element, width: number, height: number) => {
      for (const [instance, { callback, on }] of observers) {
        if (!on.has(target)) continue;
        const entry = {
          target,
          contentRect: new DOMRectReadOnly(0, 0, width, height),
          borderBoxSize: [],
          contentBoxSize: [],
          devicePixelContentBoxSize: [],
        };
        callback([entry], instance);
      }
    };
    const box = (width: number, height: number) => resize(surface, width, height);
    const cleanup = () => {
      hosts.dispose();
      first.remove();
      second.remove();
    };
    // Also when an assertion fails first: a host left listening, or a stray desk layer, would
    // change what later suites in this file hit-test.
    onTestFinished(cleanup);
    return {
      hosts,
      focus,
      content,
      sized,
      surface,
      scroller,
      field,
      first,
      second,
      scrollTo,
      loseLayoutState,
      stopRendering,
      resize,
      box,
      cleanup,
    };
  }

  it("Should restore scroll offsets and focus when a pane moves to another slot", () => {
    const { hosts, scroller, field, first, second, scrollTo, loseLayoutState, cleanup } = setup();
    const leave = hosts.place("w", first);
    scrollTo(120);
    field.focus();

    leave();
    loseLayoutState();
    expect(document.activeElement).toBe(document.body);
    hosts.place("w", second);

    expect(hosts.hostOf("w").parentElement).toBe(second);
    expect(scroller.scrollTop).toBe(120);
    expect(document.activeElement).toBe(field);
    cleanup();
  });

  it("Should leave focus where the move gave it, and still restore scroll", () => {
    const { hosts, scroller, field, first, second, scrollTo, loseLayoutState, cleanup } = setup();
    const dragSource = document.createElement("button");
    document.body.append(dragSource);
    const leave = hosts.place("w", first);
    scrollTo(64);
    field.focus();

    leave();
    loseLayoutState();
    dragSource.focus();
    hosts.place("w", second);

    expect(scroller.scrollTop).toBe(64);
    expect(document.activeElement).toBe(dragSource);
    dragSource.remove();
    cleanup();
  });

  it("Should restore scroll offsets and focus when a hidden pane is shown again", () => {
    const { hosts, surface, scroller, field, first, scrollTo, stopRendering, box, cleanup } =
      setup();
    hosts.place("w", first);
    box(640, 480);
    scrollTo(80);
    field.focus();
    const refocus = vi.spyOn(field, "focus");

    stopRendering();
    box(0, 0);
    expect(scroller.scrollTop).toBe(0);
    expect(document.activeElement).toBe(document.body);
    surface.hidden = false;
    box(640, 480);

    expect(scroller.scrollTop).toBe(80);
    expect(refocus).toHaveBeenCalledExactlyOnceWith({ preventScroll: true });
    expect(document.activeElement).toBe(field);
    cleanup();
  });

  it("Should not give focus back after the operator moved it away from the pane", () => {
    const { hosts, surface, field, first, loseLayoutState, box, cleanup } = setup();
    hosts.place("w", first);
    box(640, 480);
    field.focus();
    // A click on an empty area blurs the field while it still renders.
    field.blur();

    surface.hidden = true;
    loseLayoutState();
    box(0, 0);
    surface.hidden = false;
    box(640, 480);

    expect(document.activeElement).toBe(document.body);
    cleanup();
  });

  it("Should restore scroll but not focus into a pane whose window lost focus meanwhile", () => {
    const { hosts, focus, surface, scroller, field, first, scrollTo, stopRendering, box, cleanup } =
      setup();
    hosts.place("w", first);
    box(640, 480);
    scrollTo(90);
    field.focus();
    const refocus = vi.spyOn(field, "focus");

    stopRendering();
    box(0, 0);
    expect(document.activeElement).toBe(document.body);
    // A directional focus, an open beside it or a peer's move gave another window the focus.
    focus.windowId = "other";
    surface.hidden = false;
    box(640, 480);

    expect(scroller.scrollTop).toBe(90);
    expect(refocus).not.toHaveBeenCalled();
    expect(document.activeElement).toBe(document.body);

    // Kept for when the window has focus again.
    surface.hidden = true;
    box(0, 0);
    focus.windowId = "w";
    surface.hidden = false;
    box(640, 480);
    expect(refocus).toHaveBeenCalledExactlyOnceWith({ preventScroll: true });
    expect(document.activeElement).toBe(field);
    cleanup();
  });

  describe("keyboard handoff [#1655]", () => {
    /** The caret in another window's composer, where the operator types before the change. */
    function elsewhere() {
      const composer = document.createElement("textarea");
      document.body.append(composer);
      onTestFinished(() => composer.remove());
      return composer;
    }

    it("Should carry the keyboard to the element the window last held, without scrolling", () => {
      const handoff = new KeyboardFocusHandoff();
      const { hosts, focus, field, first, box } = setup(handoff);
      hosts.place("w", first);
      box(640, 480);
      field.focus();
      const composer = elsewhere();
      focus.windowId = "other";
      composer.focus();
      const refocus = vi.spyOn(field, "focus");

      // A focus-left chord moved this client's focus back to the window.
      focus.windowId = "w";
      handoff.request("w", handoff.begin());

      expect(refocus).toHaveBeenCalledExactlyOnceWith({ preventScroll: true });
      expect(document.activeElement).toBe(field);
    });

    it("Should land on the window itself when nothing in it held focus", () => {
      const handoff = new KeyboardFocusHandoff();
      const { hosts, surface, first, box } = setup(handoff);
      hosts.place("w", first);
      box(640, 480);
      const composer = elsewhere();
      composer.focus();

      handoff.request("w", handoff.begin());

      expect(document.activeElement).toBe(surface);
      // Focusable for the handoff only: a later click on an empty area does not focus it.
      composer.focus();
      expect(surface.hasAttribute("tabindex")).toBe(false);
    });

    it("Should land on the window itself when the element it last held cannot take focus", () => {
      const handoff = new KeyboardFocusHandoff();
      const { hosts, focus, surface, field, first, box } = setup(handoff);
      hosts.place("w", first);
      box(640, 480);
      field.focus();
      const composer = elsewhere();
      focus.windowId = "other";
      composer.focus();
      // A send button disabled after sending, still mounted.
      field.disabled = true;

      focus.windowId = "w";
      handoff.request("w", handoff.begin());

      expect(document.activeElement).toBe(surface);
    });

    it("Should wait for a window in a tab that shows a frame later", () => {
      const handoff = new KeyboardFocusHandoff();
      const { hosts, surface, field, first, box } = setup(handoff);
      hosts.place("w", first);
      box(640, 480);
      field.focus();
      const composer = elsewhere();
      composer.focus();
      surface.hidden = true;
      box(0, 0);

      // Tab next: the window manager focused the window before its tab renders.
      handoff.request("w", handoff.begin());
      expect(document.activeElement).toBe(composer);
      surface.hidden = false;
      box(640, 480);

      expect(document.activeElement).toBe(field);
    });

    it("Should leave the caret where it is for a focus another client or the CLI sent", () => {
      const handoff = new KeyboardFocusHandoff();
      const { hosts, focus, surface, field, first, box } = setup(handoff);
      hosts.place("w", first);
      box(640, 480);
      field.focus();
      const composer = elsewhere();
      focus.windowId = "other";
      composer.focus();

      // `compozy window focus --client`: the marker moves and the tab shows, nothing is requested.
      focus.windowId = "w";
      surface.hidden = true;
      box(0, 0);
      surface.hidden = false;
      box(640, 480);

      expect(document.activeElement).toBe(composer);
    });

    it("Should not take the keyboard out of an overlay, and land once the overlay gives it back", () => {
      const handoff = new KeyboardFocusHandoff();
      const { hosts, field, first, box } = setup(handoff);
      hosts.place("w", first);
      box(640, 480);
      field.focus();
      const composer = elsewhere();
      const palette = document.createElement("div");
      palette.setAttribute("role", "dialog");
      const input = document.createElement("input");
      palette.append(input);
      document.body.append(palette);
      onTestFinished(() => palette.remove());
      input.focus();

      handoff.request("w", handoff.begin());
      expect(document.activeElement).toBe(input);

      // The palette closes and returns focus to where it opened from.
      palette.remove();
      composer.focus();
      expect(document.activeElement).toBe(field);
    });

    it("Should not take the caret from a field the operator clicked while the focus change was in flight", () => {
      const handoff = new KeyboardFocusHandoff();
      const { hosts, field, first, box } = setup(handoff);
      hosts.place("w", first);
      box(640, 480);
      field.focus();
      const composer = elsewhere();
      composer.focus();

      // A directional focus starts; before the daemon answers, the operator clicks a field.
      const since = handoff.begin();
      composer.dispatchEvent(new Event("pointerdown", { bubbles: true }));
      handoff.request("w", since);

      expect(document.activeElement).toBe(composer);
    });

    it("Should drop a waiting handoff when the operator presses a pointer", () => {
      const handoff = new KeyboardFocusHandoff();
      const { hosts, surface, field, first, box } = setup(handoff);
      hosts.place("w", first);
      box(640, 480);
      field.focus();
      const composer = elsewhere();
      composer.focus();
      surface.hidden = true;
      box(0, 0);

      handoff.request("w", handoff.begin());
      composer.dispatchEvent(new Event("pointerdown", { bubbles: true }));
      surface.hidden = false;
      box(640, 480);

      expect(document.activeElement).toBe(composer);
    });
  });

  describe("through the pane's focus capture", () => {
    function renderPanes() {
      vi.stubGlobal(
        "ResizeObserver",
        class {
          observe() {}
          unobserve() {}
          disconnect() {}
        }
      );
      const shell = createShell();
      const queryClient = new QueryClient();
      queryClient.setQueryData(workspaceKeys.list(), []);
      queryClient.setQueryData(statusKeys.current(), { daemon: {} });
      const hosts = new WindowPaneHosts({
        isFocused: windowId => shell.controller.getState().focusedId === windowId,
        routeOf: windowId => shell.controller.getState().windows[windowId]?.route ?? null,
      });
      const userFocus = vi.spyOn(shell.coordinator, "userFocus");
      const ShellProvider = shell.wrapper;
      function Pane({ windowId }: { windowId: string }) {
        const member = useOsWindowMember(windowId);
        return (
          <section onFocusCapture={member.handleFocusCapture}>
            <textarea aria-label={`${windowId} composer`} />
            {/* An overlay the pane renders on the shell, as a hidden editor's question is. */}
            {createPortal(<button type="button">{`${windowId} overlay`}</button>, document.body)}
          </section>
        );
      }
      function Slot({ windowId }: { windowId: string }) {
        const slotRef = useRef<HTMLDivElement>(null);
        useLayoutEffect(() => hosts.place(windowId, slotRef.current!), [windowId]);
        return <div ref={slotRef} />;
      }
      /** `nested`: the open turned the tab into a split, so the window's slot remounts. */
      function Desk({ nested }: { nested: boolean }) {
        return (
          <>
            {nested ? (
              <div>
                <Slot key="nested" windowId="window:primary" />
              </div>
            ) : (
              <Slot key="single" windowId="window:primary" />
            )}
            {createPortal(<Pane windowId="window:primary" />, hosts.hostOf("window:primary"))}
          </>
        );
      }
      const view = render(
        <QueryClientProvider client={queryClient}>
          <ShellProvider>
            <Desk nested={false} />
          </ShellProvider>
        </QueryClientProvider>
      );
      const composer = screen.getByRole("textbox", { name: "window:primary composer" });
      const remount = () =>
        view.rerender(
          <QueryClientProvider client={queryClient}>
            <ShellProvider>
              <Desk nested />
            </ShellProvider>
          </QueryClientProvider>
        );
      const cleanup = () => {
        view.unmount();
        hosts.dispose();
        queryClient.clear();
      };
      return { shell, composer, remount, userFocus, cleanup };
    }

    it("Should leave focus on the window an open beside a focused input gave it [AS-F-20]", () => {
      const { shell, composer, remount, userFocus, cleanup } = renderPanes();
      composer.focus();
      expect(userFocus).not.toHaveBeenCalled();
      const refocus = vi.spyOn(composer, "focus");

      // The open lands beside A and the authoritative focus moves to it, B.
      act(() => shell.setRuntimeState({ focusedId: "window:peer" }));
      act(() => remount());

      expect(refocus).not.toHaveBeenCalled();
      expect(userFocus).not.toHaveBeenCalled();
      expect(shell.controller.focusWindow).not.toHaveBeenCalled();
      expect(shell.history.navigate).not.toHaveBeenCalled();
      cleanup();
    });

    // Invariant (AS-F-21): focus landing in an overlay portaled outside the pane's DOM does not
    // send window focus or write history, though React propagates it to the pane's handler.
    it("Should not focus the window for focus in an overlay it portals outside the pane [AS-F-21]", () => {
      const { shell, composer, userFocus, cleanup } = renderPanes();
      act(() => shell.setRuntimeState({ focusedId: "window:peer" }));

      act(() => screen.getByRole("button", { name: "window:primary overlay" }).focus());

      expect(userFocus).not.toHaveBeenCalled();
      expect(shell.controller.focusWindow).not.toHaveBeenCalled();
      expect(shell.history.navigate).not.toHaveBeenCalled();

      // Focus inside the pane is still the operator choosing the window.
      act(() => composer.focus());
      expect(userFocus).toHaveBeenCalledOnce();
      cleanup();
    });

    it("Should return the caret in the focused window without a focus command or history entry", () => {
      const { shell, composer, remount, userFocus, cleanup } = renderPanes();
      composer.focus();
      const refocus = vi.spyOn(composer, "focus");

      act(() => remount());

      expect(refocus).toHaveBeenCalledOnce();
      expect(document.activeElement).toBe(composer);
      expect(userFocus).not.toHaveBeenCalled();
      expect(shell.controller.focusWindow).not.toHaveBeenCalled();
      expect(shell.history.navigate).not.toHaveBeenCalled();
      cleanup();
    });
  });

  it("Should restore scroll and focus when a keyed reorder re-inserts the pane's layer [R13-01]", async () => {
    const { hosts, scroller, field, scrollTo, cleanup } = setup();
    const deskLayer = document.createElement("div");
    deskLayer.dataset.slot = "os-desk-layer";
    const [shown, other] = [document.createElement("div"), document.createElement("div")];
    const slot = document.createElement("div");
    shown.append(slot);
    deskLayer.append(shown, other);
    document.body.append(deskLayer);
    onTestFinished(() => deskLayer.remove());
    hosts.place("w", slot);
    scrollTo(120);
    field.focus();

    // The shown tab is dragged right past the other: React re-inserts its layer, no slot remounts.
    deskLayer.appendChild(shown);
    scroller.scrollTop = 0;
    expect(document.activeElement).toBe(document.body);

    await waitFor(() => expect(scroller.scrollTop).toBe(120));
    expect(document.activeElement).toBe(field);
    deskLayer.remove();
    cleanup();
  });

  it("Should restore once when a slot mounts and adopts its pane in the same commit [R14-05]", async () => {
    const { hosts, surface, cleanup } = setup();
    const owner = document.createElement("div");
    owner.setAttribute(SCROLL_OWNER_ATTRIBUTE, "");
    surface.append(owner);
    const deskLayer = document.createElement("div");
    deskLayer.dataset.slot = "os-desk-layer";
    const split = document.createElement("div");
    const [before, after] = [document.createElement("div"), document.createElement("div")];
    split.append(before);
    deskLayer.append(split);
    document.body.append(deskLayer);
    onTestFinished(() => deskLayer.remove());
    hosts.place("w", before);
    const told = vi.fn();
    owner.addEventListener(SCROLL_RESTORE_EVENT, told);

    // A drag into a split: the new slot mounts in the observed split, then adopts the host.
    split.append(after);
    hosts.place("w", after);
    await new Promise(resolve => setTimeout(resolve, 0));

    expect(told).toHaveBeenCalledOnce();
    deskLayer.remove();
    cleanup();
  });

  it("Should size-observe a pane root that mounts after placing, and release it by name [R13-04]", async () => {
    const { hosts, sized, first, cleanup } = setup();
    const late = hosts.hostOf("late");
    hosts.place("late", first);
    const root = document.createElement("section");

    late.append(root);
    await waitFor(() => expect(sized.has(root)).toBe(true));

    // The portal's children leave in the same commit that drops the window.
    root.remove();
    hosts.retain(new Set(["w"]));
    expect(sized.has(root)).toBe(false);
    cleanup();
  });

  it("Should not write an offset onto content that replaced it while the pane was hidden [R13-05]", () => {
    const { hosts, content, surface, scroller, first, scrollTo, box, cleanup } = setup();
    hosts.place("w", first);
    box(640, 480);
    scrollTo(140);

    surface.hidden = true;
    scroller.scrollTop = 0;
    box(0, 0);
    // A `window navigate` on the hidden pane: same container, new content.
    content.route = { pathname: "/files", search: { path: "docs" } };
    surface.hidden = false;
    box(640, 480);

    expect(scroller.scrollTop).toBe(0);
    cleanup();
  });

  it("Should keep an offset across a route change its container outlived in view (#1654)", () => {
    const { hosts, content, surface, scroller, first, scrollTo, box, cleanup } = setup();
    hosts.place("w", first);
    box(640, 480);
    scrollTo(140);

    // A detail opens beside the list while the pane renders: the list keeps its content.
    content.route = { pathname: "/files", search: { path: "docs/readme.md" } };
    hosts.syncRoutes();
    surface.hidden = true;
    scroller.scrollTop = 0;
    box(0, 0);
    surface.hidden = false;
    box(640, 480);

    expect(scroller.scrollTop).toBe(140);
    cleanup();
  });

  it("Should drop a hidden container's offset across a route change in a shown pane (#1654)", () => {
    const { hosts, content, surface, first, box, cleanup } = setup();
    const drawer = document.createElement("div");
    surface.append(drawer);
    hosts.place("w", first);
    box(640, 480);
    drawer.scrollTop = 260;
    drawer.dispatchEvent(new Event("scroll"));

    // The drawer is collapsed when the window navigates: its new content never scrolled.
    drawer.hidden = true;
    drawer.scrollTop = 0;
    content.route = { pathname: "/files", search: { path: "src" } };
    hosts.syncRoutes();
    drawer.hidden = false;
    surface.hidden = true;
    box(0, 0);
    surface.hidden = false;
    box(640, 480);

    expect(drawer.scrollTop).toBe(0);
    cleanup();
  });

  it("Should not record the offset a container reads while its pane is hidden (#1654)", () => {
    const { hosts, surface, scroller, first, scrollTo, box, cleanup } = setup();
    hosts.place("w", first);
    box(640, 480);
    scrollTo(220);

    // Hidden, the container reads 0 and a scroll still reaches the host (a list re-reading it).
    surface.hidden = true;
    scrollTo(0);
    box(0, 0);
    surface.hidden = false;
    box(640, 480);

    expect(scroller.scrollTop).toBe(220);
    cleanup();
  });

  it("Should restore once when a hidden pane is placed into a rendered slot (#1654)", () => {
    const { hosts, surface, first, second, box, cleanup } = setup();
    const owner = document.createElement("div");
    owner.setAttribute(SCROLL_OWNER_ATTRIBUTE, "");
    surface.append(owner);
    const told = vi.fn();
    owner.addEventListener(SCROLL_RESTORE_EVENT, told);
    first.hidden = true;
    hosts.place("w", first);
    box(0, 0);
    expect(told).not.toHaveBeenCalled();

    // Placing shows it: the box its root reports next is not a second show.
    hosts.place("w", second);
    box(640, 480);

    expect(told).toHaveBeenCalledOnce();
    first.hidden = false;
    cleanup();
  });

  /**
   * A 200px scroller over content that can be emptied and grown again, writes clamped to the
   * scrollable range the way the browser clamps them.
   */
  function clampingContent(scroller: HTMLElement) {
    const rows = document.createElement("div");
    scroller.append(rows);
    const layout = { height: 1_000, top: 0 };
    Object.defineProperty(scroller, "clientHeight", { configurable: true, get: () => 200 });
    Object.defineProperty(scroller, "scrollHeight", {
      configurable: true,
      get: () => Math.max(200, layout.height),
    });
    Object.defineProperty(scroller, "scrollTop", {
      configurable: true,
      get: () => layout.top,
      set: (value: number) => {
        layout.top = Math.min(Math.max(0, value), Math.max(0, layout.height - 200));
      },
    });
    return { rows, layout };
  }

  it("Should leave a scroll owner's offset to it and tell it the pane shows again [A-1]", () => {
    const { hosts, surface, first, box, cleanup } = setup();
    const owner = document.createElement("div");
    owner.setAttribute(SCROLL_OWNER_ATTRIBUTE, "");
    surface.append(owner);
    const told = vi.fn();
    owner.addEventListener(SCROLL_RESTORE_EVENT, told);
    hosts.place("w", first);
    box(640, 480);
    expect(told).toHaveBeenCalledOnce();
    owner.scrollTop = 300;
    owner.dispatchEvent(new Event("scroll"));

    surface.hidden = true;
    owner.scrollTop = 0;
    box(0, 0);
    surface.hidden = false;
    box(640, 480);

    // One writer: the owner restores itself on the event; the host never wrote its offset.
    expect(told).toHaveBeenCalledTimes(2);
    expect(owner.scrollTop).toBe(0);
    cleanup();
  });

  it("Should write an offset the content could not hold once the content grows [A-3]", () => {
    const { hosts, surface, scroller, first, scrollTo, resize, box, cleanup } = setup();
    const { rows, layout } = clampingContent(scroller);
    hosts.place("w", first);
    box(640, 480);
    scrollTo(600);

    // Hidden, the virtualized list unmounts its rows: shown again, the write clamps to 0.
    surface.hidden = true;
    layout.height = 0;
    scroller.scrollTop = 0;
    box(0, 0);
    surface.hidden = false;
    box(640, 480);
    expect(scroller.scrollTop).toBe(0);

    // The rows mount again in a later frame; the offset lands as soon as they can hold it.
    layout.height = 1_000;
    resize(rows, 640, 1_000);
    expect(scroller.scrollTop).toBe(600);
    cleanup();
  });

  it("Should write a restored offset its container dropped back to 0 until the operator scrolls [A-3]", () => {
    const { hosts, surface, scroller, first, scrollTo, box, cleanup } = setup();
    clampingContent(scroller);
    hosts.place("w", first);
    box(640, 480);
    scrollTo(600);
    surface.hidden = true;
    scroller.scrollTop = 0;
    box(0, 0);
    surface.hidden = false;
    box(640, 480);
    expect(scroller.scrollTop).toBe(600);

    // The container's own logic, re-reading what it saw while hidden, moves back to the top.
    scrollTo(0);
    expect(scroller.scrollTop).toBe(600);

    // The operator's gesture ends it: their scroll to the top stays and is the new offset.
    scroller.dispatchEvent(new Event("wheel"));
    scrollTo(0);
    expect(scroller.scrollTop).toBe(0);
    surface.hidden = true;
    box(0, 0);
    surface.hidden = false;
    box(640, 480);
    expect(scroller.scrollTop).toBe(0);
    cleanup();
  });

  it("Should keep a reset to 0 an in-window navigation makes right after a show (#1654)", () => {
    const { hosts, content, surface, scroller, first, scrollTo, box, cleanup } = setup();
    clampingContent(scroller);
    hosts.place("w", first);
    box(640, 480);
    scrollTo(600);
    surface.hidden = true;
    scroller.scrollTop = 0;
    box(0, 0);
    surface.hidden = false;
    box(640, 480);
    expect(scroller.scrollTop).toBe(600);

    // The palette or the sidebar navigates the window; the app resets the container to the top.
    content.route = { pathname: "/files", search: { path: "src" } };
    hosts.syncRoutes();
    scrollTo(0);

    expect(scroller.scrollTop).toBe(0);
    cleanup();
  });

  it("Should sweep scroll containers an in-window navigation removed [R13-04]", () => {
    const { hosts, scroller, surface, first, scrollTo, cleanup } = setup();
    hosts.place("w", first);
    scrollTo(30);
    // Page after page of new content, each scrolled once, while the pane stays shown.
    const pages: HTMLElement[] = [];
    for (let index = 0; index < 40; index += 1) {
      const page = document.createElement("div");
      surface.append(page);
      page.scrollTop = 10 + index;
      page.dispatchEvent(new Event("scroll"));
      page.remove();
      pages.push(page);
    }
    const retained = (
      hosts as unknown as { hosts: Map<string, { offsets: Map<Element, unknown> }> }
    ).hosts.get("w")!.offsets;

    // At most twice what the pane still holds (the floor of 16), never one entry per page.
    expect(retained.size).toBeLessThanOrEqual(17);
    expect(retained.has(scroller)).toBe(true);
    cleanup();
  });

  it("Should not write offsets back while a shown pane only resizes", () => {
    const { hosts, scroller, first, scrollTo, box, cleanup } = setup();
    hosts.place("w", first);
    box(640, 480);
    scrollTo(50);
    // Scroll anchoring moved the container during a seam drag; its scroll event is still queued.
    scroller.scrollTop = 70;

    box(600, 480);
    box(560, 480);

    expect(scroller.scrollTop).toBe(70);
    cleanup();
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
      {/* The window title: a full-title popover trigger, and still the head's grab surface. */}
      <h1 data-slot="topbar-title">
        <button type="button">Agents</button>
      </h1>
      {/* A published head control that is no native control, like the board's "Select all". */}
      <span aria-checked="false" aria-label="Select all" role="checkbox" tabIndex={0} />
      {/* A head picker's popup renders in a portal, outside the head's DOM. */}
      {createPortal(<input aria-label="Picker filter" />, document.body)}
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

  it("UT-035: a tab stays a bar reorder within the exit band and leaves the bar past it", () => {
    const { shell, tabs } = renderTabBar();
    // The content under the bar: a pointer that leaves the band lands here.
    const layer = document.createElement("div");
    layer.dataset.slot = "os-desk-layer";
    layer.getBoundingClientRect = () => new DOMRect(0, 30, 300, 500);
    document.body.append(layer);
    const barBottom = 30;

    fireEvent.pointerDown(tabs[0]!, { button: 0, pointerId: 5, clientX: 50, clientY: 15 });
    fireEvent.pointerMove(window, {
      pointerId: 5,
      clientX: 260,
      clientY: barBottom + TAB_DRAG_EXIT_PX,
    });
    expect(osTabDragStore.getSnapshot().context).toMatchObject({
      phase: "dragging",
      mode: "reorder",
      insertionIndex: 3,
      target: { kind: "reorder", index: 2 },
      contentTabId: "tab:a",
    });

    fireEvent.pointerMove(window, {
      pointerId: 5,
      clientX: 260,
      clientY: barBottom + TAB_DRAG_EXIT_PX + 1,
    });
    // Past the band the dragged active tab previews the neighbour it would split into.
    expect(osTabDragStore.getSnapshot().context).toMatchObject({
      phase: "dragging",
      mode: "split",
      insertionIndex: null,
      contentTabId: "tab:b",
    });

    fireEvent.pointerUp(window, {
      pointerId: 5,
      clientX: 260,
      clientY: barBottom + TAB_DRAG_EXIT_PX + 1,
    });
    // No reorder leaves once the tab left the bar.
    expect(shell.controller.reorderTab).not.toHaveBeenCalled();
    layer.remove();
  });

  it("Should drag a window head by its title, a popover trigger, past the threshold", () => {
    renderSplitHead();
    const head = stubPointerCapture(screen.getByTestId("head"));
    const title = screen.getByRole("button", { name: "Agents" });

    fireEvent.pointerDown(title, { button: 0, pointerId: 6, clientX: 100, clientY: 10 });
    expect(osTabDragStore.getSnapshot().context.phase).toBe("pressed");
    fireEvent.pointerMove(window, { pointerId: 6, clientX: 140, clientY: 10 });

    expect(osTabDragStore.getSnapshot().context).toMatchObject({
      phase: "dragging",
      session: { source: { kind: "head", windowId: "window:primary" } },
    });
    expect(head.setPointerCapture).toHaveBeenCalledOnce();
    fireEvent.keyDown(window, { key: "Escape" });
    expect(osTabDragStore.getSnapshot().context.phase).toBe("idle");
  });

  it("Should never arm a drag from a press on a role=checkbox span published into the head", () => {
    const shell = renderSplitHead();
    const selectAll = screen.getByRole("checkbox", { name: "Select all" });

    fireEvent.pointerDown(selectAll, { button: 0, pointerId: 7, clientX: 10, clientY: 300 });
    expect(osTabDragStore.getSnapshot().context.phase).toBe("idle");
    fireEvent.pointerMove(window, { pointerId: 7, clientX: 80, clientY: 300 });
    fireEvent.pointerUp(window, { pointerId: 7, clientX: 80, clientY: 300 });

    expect(osTabDragStore.getSnapshot().context.phase).toBe("idle");
    expect(shell.controller.moveWindow).not.toHaveBeenCalled();
  });

  it("Should never arm a drag from a press inside a popup portaled out of the head", () => {
    const shell = renderSplitHead();
    const filter = screen.getByRole("textbox", { name: "Picker filter" });

    // React still delivers this press to the head's handler, though it is not in the head.
    fireEvent.pointerDown(filter, { button: 0, pointerId: 7, clientX: 10, clientY: 300 });
    expect(osTabDragStore.getSnapshot().context.phase).toBe("idle");
    fireEvent.pointerMove(window, { pointerId: 7, clientX: 80, clientY: 300 });
    fireEvent.pointerUp(window, { pointerId: 7, clientX: 80, clientY: 300 });

    expect(osTabDragStore.getSnapshot().context.phase).toBe("idle");
    expect(shell.controller.moveWindow).not.toHaveBeenCalled();
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
  testId = "seam",
}: {
  seam: ProjectedSeam | undefined;
  testId?: string;
}) {
  const { state, onPointerDown, onKeyDown, onDoubleClick } = useOsSplitSeam(seam);
  return (
    <div
      data-state={state}
      data-testid={testId}
      tabIndex={0}
      onDoubleClick={onDoubleClick}
      onKeyDown={onKeyDown}
      onPointerDown={onPointerDown}
    />
  );
}

/**
 * The authoritative snapshot with one split tab whose split carries these weights, one member
 * window per weight. The windows are of no catalog app, so each keeps the conservative 220px floor.
 */
function snapshotWithSplit(weights: number[], revision = SNAPSHOT.revision): WindowManagerSnapshot {
  const windowIds = ["window:primary", "window:peer", "window:third"].slice(0, weights.length);
  return snapshotWithRoot(
    {
      id: "split:main",
      kind: "split",
      axis: "horizontal",
      weights,
      children: windowIds.map(id => leafNode(id)),
    },
    windowIds,
    revision
  );
}

/** The authoritative snapshot with one tab over this root, its windows of no catalog app. */
function snapshotWithRoot(
  root: LayoutNode,
  windowIds: readonly string[],
  revision: number
): WindowManagerSnapshot {
  return {
    ...SNAPSHOT,
    revision,
    windows: Object.fromEntries(
      windowIds.map(id => [
        id,
        {
          id,
          app: "uncatalogued",
          instanceKey: null,
          route: { pathname: "/", search: {} },
          navStack: [],
          desktopId: "desktop:main",
          tabId: "tab:main",
        },
      ])
    ),
    desktops: [desktopWith([{ id: "tab:main", root }])],
  };
}

/** `SEAM` as a 1000px work area with no gaps lays it out over 220px floors: 1px is 0.001. */
const EXACT_SEAM: ProjectedSeam = {
  ...SEAM,
  rect: { x: 500, y: 0, w: 0, h: 700 },
  minValue: 220,
  maxValue: 780,
};

// Invariant: a seam drag commits one `layout.resize` fenced to the revision and split it is based
// on; Escape, a cancelled or lost pointer, unmounting and a change of the context it was drawn in
// — including the seam's layout (own, ancestor or minimums) changed by someone else — (US-009.EC-2) cancel it, restore the
// shares and send nothing. While this client has a seam command in flight on any split, the drag
// is re-based on the latest layout instead and its release waits for that command, so what it
// sends always equals what its preview showed, clamp included (US-009.EC-1). Keyboard and
// pair-balance resizes carry the same fence; every seam command carries its input time on the
// monotonic command clock, from which the runtime counts its deadline. Arrow steps waiting back to
// back on one seam merge into one resize; an own command of any kind in flight re-bases a drag
// instead of cancelling it; the limit state shows with the frame that reaches the clamp.
// Owning layer: the seam gesture hook over the shared seam drag store (hooks/seam-drag-store).
describe("useOsSplitSeam gesture lifecycle", () => {
  let frames: FrameRequestCallback[] = [];

  /** A seam command's fence: its revision and seam, and the time of the input that made it. */
  const seamFence = (
    splitId: string,
    boundaryIndex: number,
    expectedRevision: number,
    inputAt: unknown = expect.any(Number)
  ) => ({ expectedRevision, rebase: { splitId, boundaryIndex }, inputAt });
  const fenceAt = (expectedRevision: number) => seamFence("split:main", 0, expectedRevision);

  function flushFrames() {
    const pending = frames;
    frames = [];
    act(() => {
      for (const frame of pending) frame(16);
    });
  }

  /** A drag on the seam the snapshot really lays out, 60px past its 500px place. */
  function startSeamDrag(snapshot: WindowManagerSnapshot = snapshotWithSplit([0.5, 0.5])) {
    const shell = exactShell(snapshot);
    const { unmount } = render(<SeamHarness seam={EXACT_SEAM} />, { wrapper: shell.wrapper });
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
    act(() => {
      windowManagerStore.trigger.seamPreviewCleared();
      windowManagerStore.trigger.bindingUnbound({});
    });
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

    // Another client changes something that leaves this seam where it was: the drag carries on,
    // and its commit still names the press revision and the seam's split, so the daemon applies
    // it only while that split is still there.
    act(() => shell.setRuntimeState({ snapshot: snapshotWithSplit([0.5, 0.5], 9) }));
    expect(seam).toHaveAttribute("data-state", "drag");
    fireEvent.pointerUp(seam, { pointerId: 4, clientX: 560, clientY: 300 });

    expect(shell.controller.resizeLayout).toHaveBeenCalledExactlyOnceWith(
      "split:main",
      0,
      expect.closeTo(0.06),
      fenceAt(SNAPSHOT.revision)
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

  it("Should re-base a drag on this client's own non-seam command and hold its release until that command settles", async () => {
    const { shell, seam } = startSeamDrag(snapshotWithSplit([0.5, 0.5]));
    let settleBalance!: () => void;
    vi.mocked(shell.controller.commandsInFlight).mockReturnValue(true);
    vi.mocked(shell.controller.commandsSettled).mockReturnValueOnce(
      new Promise<void>(resolve => {
        settleBalance = resolve;
      })
    );

    // Released while a palette balance of this client is in flight: nothing is sent yet.
    fireEvent.pointerUp(seam, { pointerId: 4, clientX: 560, clientY: 300 });
    expect(shell.controller.resizeLayout).not.toHaveBeenCalled();
    expect(seam).toHaveAttribute("data-state", "drag");
    // An arrow pressed after the release waits behind it.
    fireEvent.keyDown(seam, { key: "ArrowRight" });
    expect(shell.controller.resizeLayout).not.toHaveBeenCalled();

    // Its result lands: the held drag is re-based, the seam shown at 560 kept under the pointer,
    // 260px from where 30/70 lays it out.
    act(() => shell.setRuntimeState({ snapshot: snapshotWithSplit([0.3, 0.7], 8) }));
    expect(seam).toHaveAttribute("data-state", "drag");
    expect(windowManagerStore.getSnapshot().context.seamPreview?.deltaPx).toBeCloseTo(260);
    vi.mocked(shell.controller.commandsInFlight).mockReturnValue(false);
    await act(async () => settleBalance());

    // Authored against the balanced layout: +0.26 from 30/70, never +0.06 on top of it; the
    // arrow goes out after it.
    await waitFor(() => expect(shell.controller.resizeLayout).toHaveBeenCalledTimes(2));
    expect(shell.controller.resizeLayout).toHaveBeenNthCalledWith(
      1,
      "split:main",
      0,
      expect.closeTo(0.26),
      fenceAt(8)
    );
    expect(shell.controller.resizeLayout).toHaveBeenNthCalledWith(
      2,
      "split:main",
      0,
      expect.closeTo(0.02),
      fenceAt(8)
    );
  });

  it("Should fence an arrow-key resize and a pair balance like a drag", async () => {
    // Commands are authored from the layout: a 30/50/20 split, 1200px wide so 20% keeps its floor.
    const shell = exactShell(snapshotWithSplit([0.3, 0.5, 0.2]), 1200);
    const wide: ProjectedSeam = { ...EXACT_SEAM, leadingWeight: 0.3, trailingWeight: 0.5 };
    render(<SeamHarness seam={wide} />, { wrapper: shell.wrapper });
    const seam = screen.getByTestId("seam");
    const fence = fenceAt(SNAPSHOT.revision);

    fireEvent.keyDown(seam, { key: "ArrowRight" });
    // The balance waits behind a resize in flight on the split; let the arrow's settle first.
    await act(async () => {});
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
  /** A 1000×700 work area with no gaps, so seams sit at weight × 1000 px as in `EXACT_SEAM`. */
  function exactShell(snapshot: WindowManagerSnapshot, width = 1000, height = 700) {
    act(() =>
      windowManagerStore.trigger.workAreaMeasured({
        workArea: {
          rect: { x: 0, y: 0, w: width, h: height },
          origin: { x: 0, y: 0 },
          viewportWidth: width,
        },
      })
    );
    const shell = createShell();
    shell.setRuntimeState({
      snapshot,
      windowManagerConfig: { ...CONFIG, gaps: { inner: 0, top: 0, right: 0, bottom: 0, left: 0 } },
      activeTabIds: { "desktop:main": "tab:main" },
    });
    frames = [];
    vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => {
      frames.push(callback);
      return frames.length;
    });
    vi.stubGlobal("cancelAnimationFrame", vi.fn());
    return shell;
  }

  /** The next resize stays in flight until the returned function settles it. */
  function deferResize(shell: ReturnType<typeof createShell>) {
    let settle!: (applied: boolean) => void;
    vi.mocked(shell.controller.resizeLayout).mockImplementationOnce(() => ({
      accepted: true,
      completion: new Promise<boolean>(resolve => {
        settle = resolve;
      }),
    }));
    return async () => act(async () => settle(true));
  }

  function previewDelta() {
    return windowManagerStore.getSnapshot().context.seamPreview?.deltaPx;
  }

  it("Should keep a drag re-based when this seam's own earlier resize lands, and cancel on someone else's", async () => {
    const shell = exactShell(snapshotWithSplit([0.5, 0.5]));
    const { rerender } = render(<SeamHarness seam={EXACT_SEAM} />, { wrapper: shell.wrapper });
    const seam = stubPointerCapture(screen.getByTestId("seam"));
    const settle = deferResize(shell);
    fireEvent.pointerDown(seam, { button: 0, pointerId: 4, clientX: 500, clientY: 300 });
    fireEvent.pointerMove(seam, { pointerId: 4, clientX: 560, clientY: 300 });
    fireEvent.pointerUp(seam, { pointerId: 4, clientX: 560, clientY: 300 });
    expect(shell.controller.resizeLayout).toHaveBeenCalledExactlyOnceWith(
      "split:main",
      0,
      expect.closeTo(0.06),
      fenceAt(7)
    );

    // Grab it again where the first resize's preview shows it, before that resize is acknowledged.
    rerender(
      <SeamHarness
        seam={{ ...EXACT_SEAM, value: 560, leadingWeight: 0.56, trailingWeight: 0.44 }}
      />
    );
    fireEvent.pointerDown(seam, { button: 0, pointerId: 5, clientX: 560, clientY: 300 });
    fireEvent.pointerMove(seam, { pointerId: 5, clientX: 600, clientY: 300 });
    flushFrames();
    // Based on the unpreviewed 500 the preview is clamped against, the seam shows at 600.
    expect(previewDelta()).toBeCloseTo(100);

    // The first resize's own echo moves the split: the drag carries on from 560, still at 600.
    act(() => shell.setRuntimeState({ snapshot: snapshotWithSplit([0.56, 0.44], 8) }));
    await settle();
    expect(seam).toHaveAttribute("data-state", "drag");
    expect(previewDelta()).toBeCloseTo(40);

    // Another client's change afterwards still cancels it.
    act(() => shell.setRuntimeState({ snapshot: snapshotWithSplit([0.7, 0.3], 9) }));
    expect(seam).not.toHaveAttribute("data-state");
    expect(windowManagerStore.getSnapshot().context.seamPreview).toBeNull();
    fireEvent.pointerUp(seam, { pointerId: 5, clientX: 600, clientY: 300 });
    expect(shell.controller.resizeLayout).toHaveBeenCalledOnce();
  });

  it("Should send what the preview shows when an own resize lands mid-drag, at the moved minimum (AS-F-12)", async () => {
    const shell = exactShell(snapshotWithSplit([0.5, 0.5]));
    render(<SeamHarness seam={EXACT_SEAM} />, { wrapper: shell.wrapper });
    const seam = stubPointerCapture(screen.getByTestId("seam"));
    const settle = deferResize(shell);
    fireEvent.keyDown(seam, { key: "ArrowRight", shiftKey: true });
    fireEvent.pointerDown(seam, { button: 0, pointerId: 4, clientX: 500, clientY: 300 });

    // The +0.1 lands with the pointer down: the split is 60/40 and the seam stays under the pointer.
    act(() => shell.setRuntimeState({ snapshot: snapshotWithSplit([0.6, 0.4], 8) }));
    await settle();
    expect(previewDelta()).toBeCloseTo(-100);
    fireEvent.pointerMove(seam, { pointerId: 4, clientX: 800, clientY: 300 });
    // The clamped frame is not on screen yet, so neither is its limit state.
    expect(seam).toHaveAttribute("data-state", "drag");
    flushFrames();
    // Past 780 the trailing side would drop under its floor: the preview stops at 78/22.
    expect(previewDelta()).toBeCloseTo(200);
    expect(seam).toHaveAttribute("data-state", "limit");
    fireEvent.pointerUp(seam, { pointerId: 4, clientX: 800, clientY: 300 });

    // +0.18 from 60/40, at the revision it lands on; the press-time seam would have allowed +0.28.
    expect(shell.controller.resizeLayout).toHaveBeenCalledTimes(2);
    expect(shell.controller.resizeLayout).toHaveBeenLastCalledWith(
      "split:main",
      0,
      expect.closeTo(0.18),
      fenceAt(8)
    );
  });

  it("Should re-base on another client's change while an own resize is in flight, and send after it", async () => {
    const shell = exactShell(snapshotWithSplit([0.5, 0.5]));
    render(<SeamHarness seam={EXACT_SEAM} />, { wrapper: shell.wrapper });
    const seam = stubPointerCapture(screen.getByTestId("seam"));
    const settle = deferResize(shell);
    fireEvent.keyDown(seam, { key: "ArrowRight", shiftKey: true });
    fireEvent.pointerDown(seam, { button: 0, pointerId: 4, clientX: 500, clientY: 300 });
    fireEvent.pointerMove(seam, { pointerId: 4, clientX: 560, clientY: 300 });

    // Someone else's 30/70 lands first: the drag is re-based on it, the seam still at 560.
    act(() => shell.setRuntimeState({ snapshot: snapshotWithSplit([0.3, 0.7], 8) }));
    expect(seam).toHaveAttribute("data-state", "drag");
    expect(previewDelta()).toBeCloseTo(260);

    // Released while the own +0.1 is in flight: nothing is sent against a layout about to move.
    fireEvent.pointerUp(seam, { pointerId: 4, clientX: 560, clientY: 300 });
    expect(shell.controller.resizeLayout).toHaveBeenCalledOnce();
    expect(seam).toHaveAttribute("data-state", "drag");

    // The own +0.1 lands on top (40/60) and settles: the release goes out against it.
    act(() => shell.setRuntimeState({ snapshot: snapshotWithSplit([0.4, 0.6], 9) }));
    expect(previewDelta()).toBeCloseTo(160);
    await settle();
    expect(shell.controller.resizeLayout).toHaveBeenLastCalledWith(
      "split:main",
      0,
      expect.closeTo(0.16),
      fenceAt(9)
    );
    expect(seam).not.toHaveAttribute("data-state");
  });

  /** The two seams of a 30/40/30 split at 300 and 700; the second one's test id is "sibling". */
  function renderThirds(shell: ReturnType<typeof createShell>) {
    const first: ProjectedSeam = {
      ...EXACT_SEAM,
      rect: { x: 300, y: 0, w: 0, h: 700 },
      value: 300,
      maxValue: 480,
      leadingWeight: 0.3,
      trailingWeight: 0.4,
    };
    const sibling: ProjectedSeam = {
      ...EXACT_SEAM,
      id: "split:main:1",
      boundaryIndex: 1,
      rect: { x: 700, y: 0, w: 0, h: 700 },
      value: 700,
      minValue: 520,
      maxValue: 780,
      leadingWeight: 0.4,
      trailingWeight: 0.3,
      leadingWindowIds: ["window:peer"],
      trailingWindowIds: ["window:third"],
    };
    const view = render(
      <>
        <SeamHarness seam={first} />
        <SeamHarness seam={sibling} testId="sibling" />
      </>,
      { wrapper: shell.wrapper }
    );
    return { ...view, first, sibling };
  }

  function dragSeam(element: HTMLElement, pointerId: number, from: number, to: number) {
    fireEvent.pointerDown(element, { button: 0, pointerId, clientX: from, clientY: 300 });
    fireEvent.pointerMove(element, { pointerId, clientX: to, clientY: 300 });
  }

  /** The shares the sent resizes leave, applied in order onto `start`. */
  function sharesAfterResizes(
    shell: ReturnType<typeof createShell>,
    start: number[],
    from: number
  ) {
    return vi
      .mocked(shell.controller.resizeLayout)
      .mock.calls.slice(from)
      .reduce((weights, [, boundary, delta]) => {
        return weights.map((weight, index) => {
          if (index === boundary) return weight + delta;
          return index === boundary + 1 ? weight - delta : weight;
        });
      }, start);
  }

  const fenceOn = (boundaryIndex: number, expectedRevision: number) =>
    seamFence("split:main", boundaryIndex, expectedRevision);

  it("Should keep a drag through a sibling seam's own resize and send it against the moved bounds", async () => {
    const shell = exactShell(snapshotWithSplit([0.3, 0.4, 0.3]));
    renderThirds(shell);
    const settle = deferResize(shell);
    fireEvent.keyDown(screen.getByTestId("sibling"), { key: "ArrowLeft" });
    const seam = stubPointerCapture(screen.getByTestId("seam"));
    fireEvent.pointerDown(seam, { button: 0, pointerId: 4, clientX: 300, clientY: 300 });

    // The sibling's -0.02 lands (30/38/32): this drag carries on, its trailing side now 380px.
    act(() => shell.setRuntimeState({ snapshot: snapshotWithSplit([0.3, 0.38, 0.32], 8) }));
    expect(seam).toHaveAttribute("data-state", "drag");
    fireEvent.pointerMove(seam, { pointerId: 4, clientX: 500, clientY: 300 });
    flushFrames();
    expect(seam).toHaveAttribute("data-state", "limit");
    fireEvent.pointerUp(seam, { pointerId: 4, clientX: 500, clientY: 300 });
    await settle();

    // Stopped at 460, the moved floor, not the press-time 480.
    expect(shell.controller.resizeLayout).toHaveBeenLastCalledWith(
      "split:main",
      0,
      expect.closeTo(0.16),
      fenceAt(8)
    );
  });

  it("Should send held sibling releases one at a time, each against the layout the one before leaves (AS-F-12)", async () => {
    const shell = exactShell(snapshotWithSplit([0.3, 0.4, 0.3]));
    renderThirds(shell);
    const settleArrow = deferResize(shell);
    const settleLeft = deferResize(shell);
    const settleRight = deferResize(shell);
    const left = stubPointerCapture(screen.getByTestId("seam"));
    const right = stubPointerCapture(screen.getByTestId("sibling"));

    // +0.02 in flight; both seams are dragged and released behind it.
    fireEvent.keyDown(left, { key: "ArrowRight" });
    dragSeam(left, 4, 300, 460);
    fireEvent.pointerUp(left, { pointerId: 4, clientX: 460, clientY: 300 });
    dragSeam(right, 5, 700, 550);
    fireEvent.pointerUp(right, { pointerId: 5, clientX: 550, clientY: 300 });
    expect(shell.controller.resizeLayout).toHaveBeenCalledOnce();

    // The arrow lands (32/38/30) and settles: only the first held release goes out.
    act(() => shell.setRuntimeState({ snapshot: snapshotWithSplit([0.32, 0.38, 0.3], 8) }));
    await settleArrow();
    expect(shell.controller.resizeLayout).toHaveBeenCalledTimes(2);
    expect(shell.controller.resizeLayout).toHaveBeenNthCalledWith(
      2,
      "split:main",
      0,
      expect.closeTo(0.14),
      fenceOn(0, 8)
    );
    expect(right).toHaveAttribute("data-state", "drag");

    // Its result (46/24/30) puts the second seam's floor at 680: the held preview stops there.
    act(() => shell.setRuntimeState({ snapshot: snapshotWithSplit([0.46, 0.24, 0.3], 9) }));
    expect(right).toHaveAttribute("data-state", "limit");
    await settleLeft();

    // Sent against that layout: -0.02, not the -0.15 computed against 32/38/30.
    expect(shell.controller.resizeLayout).toHaveBeenCalledTimes(3);
    expect(shell.controller.resizeLayout).toHaveBeenNthCalledWith(
      3,
      "split:main",
      1,
      expect.closeTo(-0.02),
      fenceOn(1, 9)
    );
    expect(right).not.toHaveAttribute("data-state");
    // 46/22/32: what the last preview showed, every member at or above its 220px floor.
    const shares = sharesAfterResizes(shell, [0.32, 0.38, 0.3], 1);
    expect(shares).toEqual([expect.closeTo(0.46), expect.closeTo(0.22), expect.closeTo(0.32)]);
    for (const share of shares) expect(share).toBeGreaterThanOrEqual(0.22 - 1e-9);
    await settleRight();
  });

  it("Should hold a sibling's release made while a resumed release is in flight, then send it against its result", async () => {
    const shell = exactShell(snapshotWithSplit([0.3, 0.4, 0.3]));
    renderThirds(shell);
    const settleArrow = deferResize(shell);
    const settleLeft = deferResize(shell);
    const left = stubPointerCapture(screen.getByTestId("seam"));
    const right = stubPointerCapture(screen.getByTestId("sibling"));
    fireEvent.keyDown(left, { key: "ArrowRight" });
    dragSeam(left, 4, 300, 460);
    fireEvent.pointerUp(left, { pointerId: 4, clientX: 460, clientY: 300 });
    // The second seam is still being dragged when the first release resumes.
    dragSeam(right, 5, 700, 550);

    act(() => shell.setRuntimeState({ snapshot: snapshotWithSplit([0.32, 0.38, 0.3], 8) }));
    await settleArrow();
    expect(shell.controller.resizeLayout).toHaveBeenNthCalledWith(
      2,
      "split:main",
      0,
      expect.closeTo(0.14),
      fenceOn(0, 8)
    );

    // Released before that resize lands: it waits behind it instead of using 32/38/30.
    fireEvent.pointerUp(right, { pointerId: 5, clientX: 550, clientY: 300 });
    expect(shell.controller.resizeLayout).toHaveBeenCalledTimes(2);
    act(() => shell.setRuntimeState({ snapshot: snapshotWithSplit([0.46, 0.24, 0.3], 9) }));
    expect(right).toHaveAttribute("data-state", "limit");
    await settleLeft();

    expect(shell.controller.resizeLayout).toHaveBeenNthCalledWith(
      3,
      "split:main",
      1,
      expect.closeTo(-0.02),
      fenceOn(1, 9)
    );
    const shares = sharesAfterResizes(shell, [0.32, 0.38, 0.3], 1);
    for (const share of shares) expect(share).toBeGreaterThanOrEqual(0.22 - 1e-9);
  });

  it("Should merge arrow steps waiting on one seam into one resize, so key repeat stops on release", async () => {
    const shell = exactShell(snapshotWithSplit([0.5, 0.5]));
    render(<SeamHarness seam={EXACT_SEAM} />, { wrapper: shell.wrapper });
    const seam = screen.getByTestId("seam");
    const settle = deferResize(shell);

    // Key repeat against a slow daemon: the first step is in flight and four more wait behind it.
    for (const key of ["ArrowRight", "ArrowRight", "ArrowRight", "ArrowLeft", "ArrowRight"]) {
      fireEvent.keyDown(seam, { key });
    }
    expect(shell.controller.resizeLayout).toHaveBeenCalledExactlyOnceWith(
      "split:main",
      0,
      expect.closeTo(0.02),
      fenceAt(7)
    );

    // Once it lands, the waiting steps go out as one resize of their sum: +0.04, then nothing.
    act(() => shell.setRuntimeState({ snapshot: snapshotWithSplit([0.52, 0.48], 8) }));
    await settle();
    expect(shell.controller.resizeLayout).toHaveBeenCalledTimes(2);
    expect(shell.controller.resizeLayout).toHaveBeenLastCalledWith(
      "split:main",
      0,
      expect.closeTo(0.04),
      fenceAt(8)
    );
  });

  it("Should hold an arrow step pressed while this client's own non-seam command is in flight and author it against its result", async () => {
    const shell = exactShell(snapshotWithSplit([0.5, 0.5]));
    render(<SeamHarness seam={EXACT_SEAM} />, { wrapper: shell.wrapper });
    const seam = screen.getByTestId("seam");
    let settleBalance!: () => void;
    vi.mocked(shell.controller.commandsInFlight).mockReturnValue(true);
    vi.mocked(shell.controller.commandsSettled).mockReturnValueOnce(
      new Promise<void>(resolve => {
        settleBalance = resolve;
      })
    );

    // A palette balance of this client is in flight: the step waits for its result.
    fireEvent.keyDown(seam, { key: "ArrowRight" });
    expect(shell.controller.resizeLayout).not.toHaveBeenCalled();

    act(() => shell.setRuntimeState({ snapshot: snapshotWithSplit([0.3, 0.7], 8) }));
    vi.mocked(shell.controller.commandsInFlight).mockReturnValue(false);
    await act(async () => settleBalance());

    await waitFor(() =>
      expect(shell.controller.resizeLayout).toHaveBeenCalledExactlyOnceWith(
        "split:main",
        0,
        expect.closeTo(0.02),
        fenceAt(8)
      )
    );
  });

  it("Should keep seam input behind a release that still waits on an own non-seam command once the seam command ahead of it settles", async () => {
    const shell = exactShell(snapshotWithSplit([0.5, 0.5]));
    render(<SeamHarness seam={EXACT_SEAM} />, { wrapper: shell.wrapper });
    const seam = stubPointerCapture(screen.getByTestId("seam"));
    const settleArrow = deferResize(shell);
    let settleBalance!: () => void;
    vi.mocked(shell.controller.commandsSettled).mockReturnValueOnce(
      new Promise<void>(resolve => {
        settleBalance = resolve;
      })
    );
    fireEvent.keyDown(seam, { key: "ArrowRight" });
    // A palette balance of this client is in flight as well.
    vi.mocked(shell.controller.commandsInFlight).mockReturnValue(true);
    dragSeam(seam, 4, 500, 560);
    flushFrames();
    fireEvent.pointerUp(seam, { pointerId: 4, clientX: 560, clientY: 300 });

    // The arrow's resize settles; the release now waits on the balance, and an arrow pressed
    // meanwhile waits behind it instead of going out first.
    await settleArrow();
    fireEvent.keyDown(seam, { key: "ArrowLeft" });
    expect(shell.controller.resizeLayout).toHaveBeenCalledOnce();

    vi.mocked(shell.controller.commandsInFlight).mockReturnValue(false);
    await act(async () => settleBalance());
    await waitFor(() => expect(shell.controller.resizeLayout).toHaveBeenCalledTimes(3));
    expect(shell.controller.resizeLayout).toHaveBeenNthCalledWith(
      2,
      "split:main",
      0,
      expect.closeTo(0.06),
      fenceAt(7)
    );
    expect(shell.controller.resizeLayout).toHaveBeenNthCalledWith(
      3,
      "split:main",
      0,
      expect.closeTo(-0.02),
      fenceAt(7)
    );
  });

  it("Should compute an arrow press made while a resize is in flight against that resize's result", async () => {
    const shell = exactShell(snapshotWithSplit([0.7, 0.3]));
    const near: ProjectedSeam = {
      ...EXACT_SEAM,
      rect: { x: 700, y: 0, w: 0, h: 700 },
      value: 700,
      leadingWeight: 0.7,
      trailingWeight: 0.3,
    };
    render(<SeamHarness seam={near} />, { wrapper: shell.wrapper });
    const seam = screen.getByTestId("seam");
    const settle = deferResize(shell);

    // Two quick Shift+Right presses 80px from the floor: the first takes all 80px.
    fireEvent.keyDown(seam, { key: "ArrowRight", shiftKey: true });
    fireEvent.keyDown(seam, { key: "ArrowRight", shiftKey: true });
    expect(shell.controller.resizeLayout).toHaveBeenCalledExactlyOnceWith(
      "split:main",
      0,
      expect.closeTo(0.08),
      fenceAt(7)
    );

    // The second waits for 78/22 and finds the seam at its floor: it sends nothing, so it can
    // never push the trailing side under 220px.
    act(() => shell.setRuntimeState({ snapshot: snapshotWithSplit([0.78, 0.22], 8) }));
    await settle();
    expect(shell.controller.resizeLayout).toHaveBeenCalledOnce();
  });

  it("Should cancel when someone else moves an ancestor of the dragged seam, and keep a drag the layout leaves in place", () => {
    // [[a | b] | c] on one axis: the inner seam's place and bounds follow the outer weights too.
    const nested = (outer: number[], revision: number) =>
      snapshotWithRoot(
        {
          id: "split:outer",
          kind: "split",
          axis: "horizontal",
          weights: outer,
          children: [
            {
              id: "split:inner",
              kind: "split",
              axis: "horizontal",
              weights: [0.5, 0.5],
              children: [leafNode("window:a"), leafNode("window:b")],
            },
            leafNode("window:c"),
          ],
        },
        ["window:a", "window:b", "window:c"],
        revision
      );
    const shell = exactShell(nested([0.6, 0.4], SNAPSHOT.revision));
    const inner: ProjectedSeam = {
      ...EXACT_SEAM,
      id: "split:inner:0",
      splitId: "split:inner",
      rect: { x: 300, y: 0, w: 0, h: 700 },
      value: 300,
      minValue: 220,
      maxValue: 380,
      axisSpan: 600,
      leadingWindowIds: ["window:a"],
      trailingWindowIds: ["window:b"],
    };
    render(<SeamHarness seam={inner} />, { wrapper: shell.wrapper });
    const seam = stubPointerCapture(screen.getByTestId("seam"));
    fireEvent.pointerDown(seam, { button: 0, pointerId: 4, clientX: 300, clientY: 300 });
    fireEvent.pointerMove(seam, { pointerId: 4, clientX: 340, clientY: 300 });
    flushFrames();

    // A new revision that lays the seam out exactly where it was is no reason to stop.
    act(() => shell.setRuntimeState({ snapshot: nested([0.6, 0.4], 8) }));
    expect(seam).toHaveAttribute("data-state", "drag");

    // Someone else widens the outer branch to 70/30: the inner seam now sits at 350 with room to
    // 480, which the drag never previewed. Nothing of this client is in flight: it is cancelled.
    act(() => shell.setRuntimeState({ snapshot: nested([0.7, 0.3], 9) }));
    expect(seam).not.toHaveAttribute("data-state");
    expect(windowManagerStore.getSnapshot().context.seamPreview).toBeNull();
    fireEvent.pointerUp(seam, { pointerId: 4, clientX: 340, clientY: 300 });
    expect(shell.controller.resizeLayout).not.toHaveBeenCalled();
  });

  /** [a | [b | c]] on one axis over 1000px: the inner seam's place and bounds follow the outer. */
  function renderNested(outer: number[]) {
    const nestedAt = (weights: number[], revision: number) =>
      snapshotWithRoot(
        {
          id: "split:outer",
          kind: "split",
          axis: "horizontal",
          weights,
          children: [
            leafNode("window:a"),
            {
              id: "split:inner",
              kind: "split",
              axis: "horizontal",
              weights: [0.5, 0.5],
              children: [leafNode("window:b"), leafNode("window:c")],
            },
          ],
        },
        ["window:a", "window:b", "window:c"],
        revision
      );
    const shell = exactShell(nestedAt(outer, SNAPSHOT.revision));
    const outerSeam: ProjectedSeam = {
      ...EXACT_SEAM,
      id: "split:outer:0",
      splitId: "split:outer",
      rect: { x: 400, y: 0, w: 0, h: 700 },
      value: 400,
      maxValue: 560,
      leadingWeight: 0.4,
      trailingWeight: 0.6,
      leadingWindowIds: ["window:a"],
      trailingWindowIds: ["window:b", "window:c"],
    };
    const innerSeam: ProjectedSeam = {
      ...EXACT_SEAM,
      id: "split:inner:0",
      splitId: "split:inner",
      rect: { x: 700, y: 0, w: 0, h: 700 },
      value: 700,
      minValue: 620,
      maxValue: 780,
      axisSpan: 600,
      leadingWindowIds: ["window:b"],
      trailingWindowIds: ["window:c"],
    };
    render(
      <>
        <SeamHarness seam={outerSeam} testId="outer" />
        <SeamHarness seam={innerSeam} />
      </>,
      { wrapper: shell.wrapper }
    );
    return { shell, nestedAt };
  }

  it("Should author a nested seam's keystroke after an ancestor resize in flight lands (R7-01)", async () => {
    const { shell, nestedAt } = renderNested([0.4, 0.6]);
    const settle = deferResize(shell);
    fireEvent.keyDown(screen.getByTestId("outer"), { key: "ArrowRight", shiftKey: true });
    // Authored now, on the inner seam's 600px, this would be -0.1 and leave c at 200px on 500px.
    fireEvent.keyDown(screen.getByTestId("seam"), { key: "ArrowLeft", shiftKey: true });
    expect(shell.controller.resizeLayout).toHaveBeenCalledExactlyOnceWith(
      "split:outer",
      0,
      expect.closeTo(0.1),
      seamFence("split:outer", 0, 7)
    );

    // The outer resize lands (50/50): the inner seam sits at 750 on 500px, its floor at 720.
    act(() => shell.setRuntimeState({ snapshot: nestedAt([0.5, 0.5], 8) }));
    await settle();

    expect(shell.controller.resizeLayout).toHaveBeenCalledTimes(2);
    expect(shell.controller.resizeLayout).toHaveBeenLastCalledWith(
      "split:inner",
      0,
      expect.closeTo(-0.06),
      seamFence("split:inner", 0, 8)
    );
  });

  it("Should re-base a nested drag when this client's ancestor resize lands, and hold its release behind it (R7-01)", async () => {
    const { shell, nestedAt } = renderNested([0.4, 0.6]);
    const settle = deferResize(shell);
    fireEvent.keyDown(screen.getByTestId("outer"), { key: "ArrowRight", shiftKey: true });
    const seam = stubPointerCapture(screen.getByTestId("seam"));
    dragSeam(seam, 4, 700, 760);
    flushFrames();

    // The outer 50/50 lands while it is in flight: the inner seam moves to 750 and the drag is
    // re-based on it, the seam kept under the pointer at 760.
    act(() => shell.setRuntimeState({ snapshot: nestedAt([0.5, 0.5], 8) }));
    expect(seam).toHaveAttribute("data-state", "drag");
    expect(previewDelta()).toBeCloseTo(10);
    fireEvent.pointerUp(seam, { pointerId: 4, clientX: 760, clientY: 300 });
    expect(shell.controller.resizeLayout).toHaveBeenCalledOnce();
    await settle();

    // 10px of the inner 500px.
    expect(shell.controller.resizeLayout).toHaveBeenLastCalledWith(
      "split:inner",
      0,
      expect.closeTo(0.02),
      seamFence("split:inner", 0, 8)
    );
  });

  const BINDING_A = {
    workspaceId: "workspace:a",
    profileId: "profile:test",
    clientId: "client:web",
  };

  it("Should drop a keystroke queued in one workspace once the client is bound to another with the same split (AS-F-13)", async () => {
    act(() => windowManagerStore.trigger.bindingBound({ binding: BINDING_A }));
    const shell = exactShell(snapshotWithSplit([0.5, 0.5]));
    render(<SeamHarness seam={EXACT_SEAM} />, { wrapper: shell.wrapper });
    const seam = screen.getByTestId("seam");
    const settle = deferResize(shell);
    fireEvent.keyDown(seam, { key: "ArrowRight" });
    fireEvent.keyDown(seam, { key: "ArrowRight" });

    // Workspace B carries the same imported layout: same split id, same seam.
    act(() =>
      windowManagerStore.trigger.bindingBound({
        binding: { ...BINDING_A, workspaceId: "workspace:b" },
      })
    );
    await settle();

    expect(shell.controller.resizeLayout).toHaveBeenCalledOnce();
  });

  it("Should send nothing for a keystroke or balance queued on a seam that unmounted", async () => {
    const shell = exactShell(snapshotWithSplit([0.5, 0.5]));
    const { unmount } = render(<SeamHarness seam={EXACT_SEAM} />, { wrapper: shell.wrapper });
    const seam = screen.getByTestId("seam");
    const settle = deferResize(shell);
    fireEvent.keyDown(seam, { key: "ArrowRight" });
    fireEvent.keyDown(seam, { key: "ArrowRight" });
    fireEvent.doubleClick(seam);

    unmount();
    await settle();

    expect(shell.controller.resizeLayout).toHaveBeenCalledOnce();
    expect(shell.controller.balanceLayout).not.toHaveBeenCalled();
  });

  it("Should drop a queued keystroke older than the command deadline, counted from the key press", async () => {
    vi.useFakeTimers({ toFake: ["performance"] });
    const shell = exactShell(snapshotWithSplit([0.5, 0.5]));
    render(<SeamHarness seam={EXACT_SEAM} />, { wrapper: shell.wrapper });
    const seam = screen.getByTestId("seam");
    const settle = deferResize(shell);
    fireEvent.keyDown(seam, { key: "ArrowRight" });
    fireEvent.keyDown(seam, { key: "ArrowRight" });

    vi.advanceTimersByTime(WINDOW_MANAGER_COMMAND_DEADLINE_MS + 1);
    await settle();

    expect(shell.controller.resizeLayout).toHaveBeenCalledOnce();
  });

  it("Should keep a queued keystroke through a wall-clock step: its deadline runs on a monotonic clock", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    const shell = exactShell(snapshotWithSplit([0.5, 0.5]));
    render(<SeamHarness seam={EXACT_SEAM} />, { wrapper: shell.wrapper });
    const seam = screen.getByTestId("seam");
    const settle = deferResize(shell);
    fireEvent.keyDown(seam, { key: "ArrowRight" });
    fireEvent.keyDown(seam, { key: "ArrowRight" });

    // The system clock jumps a minute ahead while the second key waits; no time has passed.
    vi.setSystemTime(Date.now() + 60_000);
    await settle();

    expect(shell.controller.resizeLayout).toHaveBeenCalledTimes(2);
  });

  it("Should send a keystroke resumed near its deadline with the time of the key press (AS-F-13)", async () => {
    vi.useFakeTimers({ toFake: ["performance"] });
    const shell = exactShell(snapshotWithSplit([0.5, 0.5]));
    render(<SeamHarness seam={EXACT_SEAM} />, { wrapper: shell.wrapper });
    const seam = screen.getByTestId("seam");
    const settle = deferResize(shell);
    fireEvent.keyDown(seam, { key: "ArrowRight" });
    const pressedAt = performance.now();
    fireEvent.keyDown(seam, { key: "ArrowRight" });

    // Resumed 7.5s after the press: it goes out still carrying the press time, so the runtime
    // gives it what is left of the same 8s, not a new deadline behind other commands.
    vi.advanceTimersByTime(7_500);
    await settle();

    expect(shell.controller.resizeLayout).toHaveBeenCalledTimes(2);
    expect(shell.controller.resizeLayout).toHaveBeenLastCalledWith(
      "split:main",
      0,
      expect.closeTo(0.02),
      seamFence("split:main", 0, SNAPSHOT.revision, pressedAt)
    );
  });

  it("Should cancel a held release older than the command deadline and restore the shares", async () => {
    vi.useFakeTimers({ toFake: ["performance"] });
    const shell = exactShell(snapshotWithSplit([0.5, 0.5]));
    render(<SeamHarness seam={EXACT_SEAM} />, { wrapper: shell.wrapper });
    const seam = stubPointerCapture(screen.getByTestId("seam"));
    const settle = deferResize(shell);
    fireEvent.keyDown(seam, { key: "ArrowRight" });
    dragSeam(seam, 4, 500, 560);
    flushFrames();
    fireEvent.pointerUp(seam, { pointerId: 4, clientX: 560, clientY: 300 });
    expect(seam).toHaveAttribute("data-state", "drag");

    vi.advanceTimersByTime(WINDOW_MANAGER_COMMAND_DEADLINE_MS + 1);
    await settle();

    expect(seam).not.toHaveAttribute("data-state");
    expect(windowManagerStore.getSnapshot().context.seamPreview).toBeNull();
    expect(shell.controller.resizeLayout).toHaveBeenCalledOnce();
  });

  it("Should cancel a drag whose seam a config-only publication moved, never sending the stale delta (R5-01)", () => {
    const { shell, seam } = startSeamDrag();
    fireEvent.pointerMove(seam, { pointerId: 4, clientX: 780, clientY: 300 });
    flushFrames();

    // A 32px inner gap with the same snapshot: the span is 968 now, and +280px from the 1000px
    // seam would leave the trailing side under its 220px floor.
    act(() =>
      shell.setRuntimeState({
        windowManagerConfig: {
          ...CONFIG,
          gaps: { inner: 32, top: 0, right: 0, bottom: 0, left: 0 },
        },
      })
    );
    expect(seam).not.toHaveAttribute("data-state");
    expect(windowManagerStore.getSnapshot().context.seamPreview).toBeNull();

    fireEvent.pointerUp(seam, { pointerId: 4, clientX: 780, clientY: 300 });
    expect(shell.controller.resizeLayout).not.toHaveBeenCalled();
  });

  it("Should balance only the pair when a balance queued at two members resumes at three (AS-F-14)", async () => {
    const shell = exactShell(snapshotWithSplit([0.5, 0.5]));
    render(<SeamHarness seam={EXACT_SEAM} />, { wrapper: shell.wrapper });
    const seam = screen.getByTestId("seam");
    const settle = deferResize(shell);
    fireEvent.keyDown(seam, { key: "ArrowRight" });
    fireEvent.doubleClick(seam);

    // The resize lands with a closed member reopened into the split at 30%: 43.4/26.6/30.
    act(() => shell.setRuntimeState({ snapshot: snapshotWithSplit([0.434, 0.266, 0.3], 8) }));
    await settle();

    // 35/35/30: the third member keeps its 30%.
    expect(shell.controller.balanceLayout).not.toHaveBeenCalled();
    expect(shell.controller.resizeLayout).toHaveBeenLastCalledWith(
      "split:main",
      0,
      expect.closeTo(-0.084),
      fenceAt(8)
    );
  });

  it("Should author an arrow pressed during a live drag from the stored layout, not the preview (R6-01)", () => {
    const shell = exactShell(snapshotWithSplit([0.25, 0.75]));
    const quarter: ProjectedSeam = {
      ...EXACT_SEAM,
      rect: { x: 250, y: 0, w: 0, h: 700 },
      value: 250,
      leadingWeight: 0.25,
      trailingWeight: 0.75,
    };
    const { rerender } = render(<SeamHarness seam={quarter} />, { wrapper: shell.wrapper });
    const seam = stubPointerCapture(screen.getByTestId("seam"));
    dragSeam(seam, 4, 250, 650);
    flushFrames();
    // The rendered seam now carries the preview: 65/35.
    rerender(
      <SeamHarness seam={{ ...quarter, value: 650, leadingWeight: 0.65, trailingWeight: 0.35 }} />
    );

    fireEvent.keyDown(seam, { key: "ArrowLeft", shiftKey: true });

    // From the stored 25/75 the step stops at the leading side's 220px floor: -0.03, not -0.1.
    expect(shell.controller.resizeLayout).toHaveBeenCalledExactlyOnceWith(
      "split:main",
      0,
      expect.closeTo(-0.03),
      fenceAt(7)
    );
  });

  it("Should base a second pointer's drag on a sibling seam on the stored layout, not the first drag's preview (R6-01)", () => {
    const shell = exactShell(snapshotWithSplit([0.3, 0.4, 0.3]));
    const { rerender, first, sibling } = renderThirds(shell);
    const left = stubPointerCapture(screen.getByTestId("seam"));
    dragSeam(left, 4, 300, 460);
    flushFrames();
    // As rendered under that preview (46/24/30) the sibling's floor is at 680.
    rerender(
      <>
        <SeamHarness seam={first} />
        <SeamHarness seam={{ ...sibling, minValue: 680, leadingWeight: 0.24 }} testId="sibling" />
      </>
    );
    const right = stubPointerCapture(screen.getByTestId("sibling"));

    dragSeam(right, 5, 700, 550);
    // The stored 30/40/30 puts that floor at 520: 550 is within it.
    expect(right).toHaveAttribute("data-state", "drag");
    fireEvent.pointerUp(right, { pointerId: 5, clientX: 550, clientY: 300 });

    expect(shell.controller.resizeLayout).toHaveBeenCalledExactlyOnceWith(
      "split:main",
      1,
      expect.closeTo(-0.15),
      fenceOn(1, 7)
    );
  });

  it("Should keep resuming queued commands behind one that throws, and free the queue after", async () => {
    const errors = vi.spyOn(console, "error").mockImplementation(() => {});
    const shell = exactShell(snapshotWithSplit([0.5, 0.5]));
    render(<SeamHarness seam={EXACT_SEAM} />, { wrapper: shell.wrapper });
    const seam = screen.getByTestId("seam");
    const settle = deferResize(shell);
    vi.mocked(shell.controller.resizeLayout).mockImplementationOnce(() => {
      throw new Error("The transport failed.");
    });
    fireEvent.keyDown(seam, { key: "ArrowRight" });
    fireEvent.keyDown(seam, { key: "ArrowRight" });
    // A balance queued behind the second step, so the two waiting commands stay separate.
    fireEvent.doubleClick(seam);

    // The second step throws as it is sent; the balance behind it still goes out.
    await settle();
    expect(shell.controller.resizeLayout).toHaveBeenCalledTimes(2);
    expect(shell.controller.balanceLayout).toHaveBeenCalledOnce();
    expect(errors).toHaveBeenCalledOnce();

    // Nothing is left holding the queue: the next press goes out at once.
    await act(async () => {});
    fireEvent.keyDown(seam, { key: "ArrowRight" });
    expect(shell.controller.resizeLayout).toHaveBeenCalledTimes(3);
  });

  it("Should cancel a drag whose seam turns to the other axis while this client's resize is in flight (R7-02)", async () => {
    const shell = exactShell(snapshotWithSplit([0.5, 0.5]));
    render(<SeamHarness seam={EXACT_SEAM} />, { wrapper: shell.wrapper });
    const seam = stubPointerCapture(screen.getByTestId("seam"));
    const settle = deferResize(shell);
    fireEvent.keyDown(seam, { key: "ArrowRight" });
    dragSeam(seam, 4, 500, 560);
    flushFrames();

    // The same pair, now stacked: a re-base cannot carry a horizontal delta onto it.
    act(() =>
      shell.setRuntimeState({
        snapshot: snapshotWithRoot(
          {
            id: "split:main",
            kind: "split",
            axis: "vertical",
            weights: [0.5, 0.5],
            children: [leafNode("window:primary"), leafNode("window:peer")],
          },
          ["window:primary", "window:peer"],
          8
        ),
      })
    );
    expect(seam).not.toHaveAttribute("data-state");
    expect(windowManagerStore.getSnapshot().context.seamPreview).toBeNull();
    fireEvent.pointerUp(seam, { pointerId: 4, clientX: 560, clientY: 300 });
    await settle();

    expect(shell.controller.resizeLayout).toHaveBeenCalledOnce();
  });

  it("Should stop a horizontal seam over a collapsed branch at its minimum and commit there (F8)", () => {
    // 400×700: [a | b] on top cannot sit side by side over 220px floors, so it shows one member
    // at a time under its 36px switcher; c below. Stored 60/40: the seam sits at 420.
    const shell = exactShell(
      snapshotWithRoot(
        {
          id: "split:outer",
          kind: "split",
          axis: "vertical",
          weights: [0.6, 0.4],
          children: [
            {
              id: "split:top",
              kind: "split",
              axis: "horizontal",
              weights: [0.5, 0.5],
              children: [leafNode("window:a"), leafNode("window:b")],
            },
            leafNode("window:c"),
          ],
        },
        ["window:a", "window:b", "window:c"],
        SNAPSHOT.revision
      ),
      400
    );
    const outer: ProjectedSeam = {
      ...EXACT_SEAM,
      id: "split:outer:0",
      splitId: "split:outer",
      orientation: "horizontal",
      rect: { x: 0, y: 420, w: 400, h: 0 },
      value: 420,
      // 36px switcher + the 220px member floor; 220px for c below.
      minValue: 256,
      maxValue: 480,
      axisSpan: 700,
      leadingWeight: 0.6,
      trailingWeight: 0.4,
      leadingWindowIds: ["window:a", "window:b"],
      trailingWindowIds: ["window:c"],
    };
    render(<SeamHarness seam={outer} />, { wrapper: shell.wrapper });
    const seam = stubPointerCapture(screen.getByTestId("seam"));

    // 270px up, 106px past the minimum.
    fireEvent.pointerDown(seam, { button: 0, pointerId: 4, clientX: 200, clientY: 420 });
    fireEvent.pointerMove(seam, { pointerId: 4, clientX: 200, clientY: 150 });
    flushFrames();
    expect(seam).toHaveAttribute("data-state", "limit");
    expect(windowManagerStore.getSnapshot().context.seamPreview).toMatchObject({
      splitId: "split:outer",
      boundaryIndex: 0,
    });
    fireEvent.pointerUp(seam, { pointerId: 4, clientX: 200, clientY: 150 });

    expect(shell.controller.resizeLayout).toHaveBeenCalledExactlyOnceWith(
      "split:outer",
      0,
      expect.closeTo(-164 / 700),
      seamFence("split:outer", 0, SNAPSHOT.revision)
    );
  });

  it("Should drag a seam over a collapsed pair to its limit on half-pixel shares and commit one resize there (F8)", () => {
    // The QA geometry: a 290×907 column split evenly (453.5px per member), the collapsed pair on
    // top under its 36px switcher, 220px floors.
    const shell = exactShell(
      snapshotWithRoot(
        {
          id: "split:outer",
          kind: "split",
          axis: "vertical",
          weights: [0.5, 0.5],
          children: [
            {
              id: "split:top",
              kind: "split",
              axis: "horizontal",
              weights: [0.5, 0.5],
              children: [leafNode("window:a"), leafNode("window:b")],
            },
            leafNode("window:c"),
          ],
        },
        ["window:a", "window:b", "window:c"],
        SNAPSHOT.revision
      ),
      290,
      907
    );
    const outer = authoritativeSeam(
      shell.state,
      { x: 0, y: 0, w: 290, h: 907 },
      {
        splitId: "split:outer",
        boundaryIndex: 0,
      }
    );
    if (outer === null) throw new Error("expected the outer seam");
    render(<SeamHarness seam={outer} />, { wrapper: shell.wrapper });
    const seam = stubPointerCapture(screen.getByTestId("seam"));

    // Up to the top of the column, far past the limit.
    fireEvent.pointerDown(seam, { button: 0, pointerId: 4, clientX: 100, clientY: outer.value });
    fireEvent.pointerMove(seam, { pointerId: 4, clientX: 100, clientY: 0 });
    flushFrames();
    expect(seam).toHaveAttribute("data-state", "limit");
    expect(windowManagerStore.getSnapshot().context.seamPreview).toMatchObject({
      splitId: "split:outer",
    });
    fireEvent.pointerUp(seam, { pointerId: 4, clientX: 100, clientY: 0 });

    expect(shell.controller.resizeLayout).toHaveBeenCalledExactlyOnceWith(
      "split:outer",
      0,
      expect.closeTo((outer.minValue - outer.value) / outer.axisSpan),
      seamFence("split:outer", 0, SNAPSHOT.revision)
    );
    // The pair keeps 36px of switcher and 220px of member at the limit.
    expect(outer.minValue).toBeGreaterThanOrEqual(256);
  });

  it("Should keep a three-member split side by side at every frame and commit the shift of the last one (F8)", () => {
    // The row of the seam preview suite: 820px, raw shares 300.3 | 300.3 | 219.4 over 220px
    // floors, allocated 300 | 300 | 220; any half-pixel shift of the first seam hands the third
    // member's spare pixel to the other two and leaves it 219px wide.
    const snapshot = snapshotWithSplit([300.3 / 820, 300.3 / 820, 219.4 / 820]);
    const shell = exactShell(snapshot, 820, 700);
    const area = { x: 0, y: 0, w: 820, h: 700 };
    const stored = authoritativeSeam(shell.state, area, {
      splitId: "split:main",
      boundaryIndex: 0,
    });
    const tab = snapshot.desktops[0]?.tabs[0];
    if (stored === null || tab === undefined) throw new Error("expected the first row seam");
    const input = (layout: LayoutTab) => ({
      tab: layout,
      workArea: area,
      gaps: { inner: 0, top: 0, right: 0, bottom: 0, left: 0 },
      minimums: buildWindowManagerMinimums(snapshot),
    });
    // The frame the published preview renders, admitted as the runtime view admits it.
    const shownShift = () => {
      const admitted = admitSeamShift(input(tab), stored, previewDelta() ?? 0);
      if (admitted === null) throw new Error("expected the first row seam");
      return admitted.shiftPx;
    };
    // Rendered half a pixel past where the layout has it, as a preview frame may leave it: the
    // press re-anchors on the stored seam, so every whole-pixel pointer asks for k + 0.5px.
    render(<SeamHarness seam={{ ...stored, value: stored.value + 0.5 }} />, {
      wrapper: shell.wrapper,
    });
    const seam = stubPointerCapture(screen.getByTestId("seam"));
    fireEvent.pointerDown(seam, { button: 0, pointerId: 4, clientX: stored.value, clientY: 300 });

    // Past both bounds (80px either way): no frame shows the row one member at a time. Inside
    // them each frame is held half a pixel back, which is no limit; past them the bound holds it
    // back a pixel or more, and the seam reports its limit.
    for (let pointer = stored.value - 120; pointer <= stored.value + 120; pointer += 1) {
      fireEvent.pointerMove(seam, { pointerId: 4, clientX: pointer, clientY: 300 });
      flushFrames();
      const requestedPx = pointer - stored.value + 0.5;
      expect(previewDelta()).toBe(requestedPx);
      const frame = projectTab(input(applySeamPreviewToTab(tab, stored, shownShift())));
      expect(frame.collapsed).toEqual([]);
      expect(frame.seams.map(entry => entry.id)).toEqual(["split:main:0", "split:main:1"]);
      expect(seam).toHaveAttribute(
        "data-state",
        Math.abs(requestedPx) - 80 >= 1 ? "limit" : "drag"
      );
    }
    // Released asking for 40.5px: the last frame shows 40px, and that is what commits.
    fireEvent.pointerUp(seam, { pointerId: 4, clientX: stored.value + 40, clientY: 300 });

    expect(previewDelta()).toBe(40.5);
    expect(shownShift()).toBe(40);
    expect(shell.controller.resizeLayout).toHaveBeenCalledExactlyOnceWith(
      "split:main",
      0,
      seamWeightDelta(stored, 40),
      fenceAt(SNAPSHOT.revision)
    );
  });

  it("Should step a three-member split by arrow keys only to shifts it still lays out side by side at, up to its bound (AS-F-18)", async () => {
    // The row of the F8 drag case: 820px, raw shares 300.3 | 300.3 | 219.4 over 220px floors,
    // allocated 300 | 300 | 220. One ArrowRight asks for 0.02 × 820 = 16.4px: unadmitted, the
    // shares 316.7 | 283.9 | 219.4 floor to 818 and the two spare pixels go to .9 and .7, leaving
    // the third member 219px wide. Admitted, the step is 16px: 316.3 | 284.3 | 219.4 keeps the
    // fractions .3 | .3 | .4, so the spare pixel stays with the third member (220px). Every step
    // keeps them, so each moves 16px until the second member reaches 220px at +80px; the next
    // press is at the bound and sends nothing.
    let weights = [300.3 / 820, 300.3 / 820, 219.4 / 820];
    let revision = SNAPSHOT.revision;
    const shell = exactShell(snapshotWithSplit(weights, revision), 820, 700);
    const area = { x: 0, y: 0, w: 820, h: 700 };
    const target = { splitId: "split:main", boundaryIndex: 0 };
    const stored = authoritativeSeam(shell.state, area, target);
    if (stored === null) throw new Error("expected the first row seam");
    render(<SeamHarness seam={stored} />, { wrapper: shell.wrapper });
    const seam = screen.getByTestId("seam");
    act(() => seam.focus());

    for (let press = 1; press <= 6; press += 1) {
      fireEvent.keyDown(seam, { key: "ArrowRight" });
      await act(async () => {});
      if (press === 6) break;
      expect(shell.controller.resizeLayout).toHaveBeenCalledTimes(press);
      expect(shell.controller.resizeLayout).toHaveBeenLastCalledWith(
        "split:main",
        0,
        expect.closeTo(16 / 820, 9),
        fenceAt(revision)
      );
      const delta = vi.mocked(shell.controller.resizeLayout).mock.lastCall?.[2] ?? 0;
      weights = [(weights[0] ?? 0) + delta, (weights[1] ?? 0) - delta, weights[2] ?? 0];
      revision += 1;
      act(() => shell.setRuntimeState({ snapshot: snapshotWithSplit(weights, revision) }));
      const tab = shell.state.snapshot?.desktops[0]?.tabs[0];
      if (tab === undefined) throw new Error("expected the row's tab");
      const after = projectTab({
        tab,
        workArea: area,
        gaps: { inner: 0, top: 0, right: 0, bottom: 0, left: 0 },
        minimums: buildWindowManagerMinimums(snapshotWithSplit(weights, revision)),
      });
      expect(after.collapsed).toEqual([]);
      expect(after.seams.map(entry => entry.id)).toEqual(["split:main:0", "split:main:1"]);
      expect(seam).toHaveFocus();
    }
    // At the bound: no step is sent, and the seam keeps focus.
    expect(shell.controller.resizeLayout).toHaveBeenCalledTimes(5);
    expect(seam).toHaveFocus();
  });

  it("Should not arm a press on a seam the stored layout does not have (R7-02)", () => {
    const shell = exactShell(snapshotWithSplit([0.5, 0.5]));
    render(<SeamHarness seam={{ ...EXACT_SEAM, id: "split:gone:0", splitId: "split:gone" }} />, {
      wrapper: shell.wrapper,
    });
    const seam = stubPointerCapture(screen.getByTestId("seam"));

    dragSeam(seam, 4, 500, 560);
    flushFrames();
    fireEvent.pointerUp(seam, { pointerId: 4, clientX: 560, clientY: 300 });

    expect(seam).not.toHaveAttribute("data-state");
    expect(seam.setPointerCapture).not.toHaveBeenCalled();
    expect(windowManagerStore.getSnapshot().context.seamPreview).toBeNull();
    expect(shell.controller.resizeLayout).not.toHaveBeenCalled();
  });

  /**
   * A 1000px-wide column: a row of five windows over one window. Five 220px floors do not fit in
   * 1000px, so the row shows one member at a time under its 36px switcher (256px tall at least).
   */
  function rowOverWindow(weights: number[]): WindowManagerSnapshot {
    const row = ["window:r1", "window:r2", "window:r3", "window:r4", "window:r5"];
    return snapshotWithRoot(
      {
        id: "split:outer",
        kind: "split",
        axis: "vertical",
        weights,
        children: [
          {
            id: "split:row",
            kind: "split",
            axis: "horizontal",
            weights: row.map(() => 0.2),
            children: row.map(id => leafNode(id)),
          },
          leafNode("window:below"),
        ],
      },
      [...row, "window:below"],
      SNAPSHOT.revision
    );
  }

  function projectColumn(weights: number[], height: number) {
    const snapshot = rowOverWindow(weights);
    const tab = snapshot.desktops[0]?.tabs[0];
    if (tab === undefined) throw new Error("expected the column's tab");
    return projectTab({
      tab,
      workArea: { x: 0, y: 0, w: 1000, h: height },
      gaps: { inner: 0, top: 0, right: 0, bottom: 0, left: 0 },
      minimums: buildWindowManagerMinimums(snapshot),
    });
  }

  function outerSeam(shell: ReturnType<typeof exactShell>, height: number) {
    const outer = authoritativeSeam(
      shell.state,
      { x: 0, y: 0, w: 1000, h: height },
      { splitId: "split:outer", boundaryIndex: 0 }
    );
    if (outer === null) throw new Error("expected the column seam");
    return outer;
  }

  it("Should hold a balance back to the nearest shift its split still lays out side by side at", () => {
    // 1000×500: 260px for the collapsed row (≥ 256) and 240px for the window (≥ 220) both fit.
    // An even split gives each 250px, under the row's 256px, and would show the column one member
    // at a time; the balance stops at the row's 256px instead, a -4px shift.
    expect(projectColumn([0.5, 0.5], 500).collapsed.map(entry => entry.splitId)).toContain(
      "split:outer"
    );
    const shell = exactShell(rowOverWindow([0.52, 0.48]), 1000, 500);
    const outer = outerSeam(shell, 500);
    render(<SeamHarness seam={outer} />, { wrapper: shell.wrapper });

    fireEvent.doubleClick(screen.getByTestId("seam"));

    expect(shell.controller.balanceLayout).not.toHaveBeenCalled();
    expect(shell.controller.resizeLayout).toHaveBeenCalledExactlyOnceWith(
      "split:outer",
      0,
      expect.closeTo(-4 / 500, 9),
      seamFence("split:outer", 0, SNAPSHOT.revision)
    );
    const delta = vi.mocked(shell.controller.resizeLayout).mock.lastCall?.[2] ?? 0;
    const after = projectColumn([0.52 + delta, 0.48 - delta], 500);
    expect(after.collapsed.map(entry => entry.splitId)).toEqual(["split:row"]);
    expect(after.seams.map(entry => entry.id)).toEqual(["split:outer:0"]);
  });

  it("Should not balance an odd span with layout.balance when the exact tie leaves the trailing side under its minimum (R13-03)", () => {
    // 1000×511, a window (220px floor) over the collapsed row (256px floor: 220 + its switcher).
    // 256 is the larger half of 511, and `layout.balance` writes exactly 0.5 | 0.5: both shares
    // are 255.5, the tie gives the spare pixel to the first member, and the row gets 255. The
    // balance must test those exact weights, so it resizes by the admitted shift instead.
    // From 0.45 | 0.55 (230 | 281px) the evening shift asks for 25.55px, held to 25 (255 | 256).
    const row = ["window:r1", "window:r2", "window:r3", "window:r4", "window:r5"];
    const windowOverRow = (weights: number[]) =>
      snapshotWithRoot(
        {
          id: "split:outer",
          kind: "split",
          axis: "vertical",
          weights,
          children: [
            leafNode("window:above"),
            {
              id: "split:row",
              kind: "split",
              axis: "horizontal",
              weights: row.map(() => 0.2),
              children: row.map(id => leafNode(id)),
            },
          ],
        },
        ["window:above", ...row],
        SNAPSHOT.revision
      );
    const project = (weights: number[]) => {
      const snapshot = windowOverRow(weights);
      const tab = snapshot.desktops[0]?.tabs[0];
      if (tab === undefined) throw new Error("expected the column's tab");
      return projectTab({
        tab,
        workArea: { x: 0, y: 0, w: 1000, h: 511 },
        gaps: { inner: 0, top: 0, right: 0, bottom: 0, left: 0 },
        minimums: buildWindowManagerMinimums(snapshot),
      });
    };
    expect(project([0.5, 0.5]).collapsed.map(entry => entry.splitId)).toContain("split:outer");
    const shell = exactShell(windowOverRow([0.45, 0.55]), 1000, 511);
    render(<SeamHarness seam={outerSeam(shell, 511)} />, { wrapper: shell.wrapper });

    fireEvent.doubleClick(screen.getByTestId("seam"));

    expect(shell.controller.balanceLayout).not.toHaveBeenCalled();
    expect(shell.controller.resizeLayout).toHaveBeenCalledExactlyOnceWith(
      "split:outer",
      0,
      expect.closeTo(25 / 511, 9),
      seamFence("split:outer", 0, SNAPSHOT.revision)
    );
    const delta = vi.mocked(shell.controller.resizeLayout).mock.lastCall?.[2] ?? 0;
    const after = project([0.45 + delta, 0.55 - delta]);
    expect(after.collapsed.map(entry => entry.splitId)).toEqual(["split:row"]);
    expect(after.seams.map(entry => entry.id)).toEqual(["split:outer:0"]);
  });

  it("Should balance a two-member split with layout.balance when the even split lays out side by side", () => {
    // 1000×600: an even split gives the collapsed row 300px, over its 256px, and the window 300px.
    const shell = exactShell(rowOverWindow([0.6, 0.4]), 1000, 600);
    render(<SeamHarness seam={outerSeam(shell, 600)} />, { wrapper: shell.wrapper });

    fireEvent.doubleClick(screen.getByTestId("seam"));

    expect(shell.controller.resizeLayout).not.toHaveBeenCalled();
    expect(shell.controller.balanceLayout).toHaveBeenCalledExactlyOnceWith(
      { splitId: "split:outer" },
      seamFence("split:outer", 0, SNAPSHOT.revision)
    );
    expect(projectColumn([0.5, 0.5], 600).collapsed.map(entry => entry.splitId)).toEqual([
      "split:row",
    ]);
  });

  it("Should fence a two-member balance like a drag", () => {
    const shell = exactShell(snapshotWithSplit([0.5, 0.5]));
    render(<SeamHarness seam={EXACT_SEAM} />, { wrapper: shell.wrapper });

    fireEvent.doubleClick(screen.getByTestId("seam"));

    expect(shell.controller.balanceLayout).toHaveBeenCalledExactlyOnceWith(
      { splitId: "split:main" },
      fenceAt(SNAPSHOT.revision)
    );
  });
});
