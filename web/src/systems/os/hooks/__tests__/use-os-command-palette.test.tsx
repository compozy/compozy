// Suite: OS command palette
// Invariant: palette commands preserve the selected tab's identity and route by
// delegating each lifecycle decision to the routing coordinator exactly once,
// and a pushed view owns the surface completely — its own results, its own
// filters, and a keyboard selection that survives the catalog moving.
// Owning layer: command-palette hook and presentation boundary.
// Boundary OUT: keyboard action dispatch (window-manager-action-dispatch),
// overlay lifetime and stack reset (use-desktop-overlays), stack and filter
// mechanics (palette-view-stack, palette-session-filters), and daemon command
// transport.
import { act, render, renderHook, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { LayoutDesktop } from "../../lib/window-manager-types";
import type { OsDesktopRuntimeStore, OsWindow } from "../../lib/os-types";
import { OsCommandPalette } from "../../components/os-command-palette";
import { useOsCommandPalette } from "../use-os-command-palette";
import { PALETTE_SESSION_ROW_LIMIT } from "../use-os-palette-sessions-view";

type PaletteSessionFixture = {
  id: string;
  name: string | null;
  agent_name: string;
  badge: string;
  workspace_id?: string;
  updated_at: string;
  attention_changed_at?: string;
};

// The view path is the one piece of palette state a click has to move, so the
// mock is a real subscribable store rather than a value: pushing a view in a
// test re-renders the palette exactly as it does in the shell.
const paletteMocks = vi.hoisted(() => {
  const listeners = new Set<() => void>();
  let viewStack: readonly { viewId: "sessions" }[] = [];
  return {
    activeWorkspaceId: "workspace:alpha" as string | null,
    closeWindow: vi.fn(async () => true),
    coordinator: {
      userActivateWindow: vi.fn(async () => true),
      userOpen: vi.fn(async () => "window:new-tab" as string | null),
    },
    desktop: null as OsDesktopRuntimeStore | null,
    isWaiting: vi.fn<(sessionId: string) => boolean>(() => false),
    jumpToSession: vi.fn(),
    openForAgent: vi.fn(),
    pending: false,
    paletteIntent: null as { kind: "destination"; windowId: string } | null,
    paletteIntentCleared: vi.fn(),
    paletteIntentRequested: vi.fn(),
    sessionListScope: "recent" as "recent" | "all" | "all-workspaces",
    sessionListSaving: false,
    sessions: [] as PaletteSessionFixture[],
    sessionsLoading: false,
    sessionsWorkspaceId: vi.fn(),
    setSessionListScope: vi.fn(),
    toggleLocked: false,
    windowCommands: {
      arrangeCommands: [],
      commandsAvailable: true,
      dispatchArrange: vi.fn(),
      dispatchPlacement: vi.fn(),
      focusedWindowActions: null,
      placementCommands: [],
      shortcutLabels: {},
    },
    windowSlots: new Map<string, { crumb?: string }>(),
    workspaceGroups: [] as Array<{
      workspaceId: string;
      workspaceName: string;
      sessions: PaletteSessionFixture[];
      total: number;
      loading: boolean;
      failed: boolean;
      retry: () => void;
    }>,
    workspaces: [
      { id: "workspace:alpha", name: "Alpha" },
      { id: "workspace:beta", name: "Beta" },
    ],
    readViewStack: () => viewStack,
    subscribeViewStack: (listener: () => void) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    writeViewStack: (next: readonly { viewId: "sessions" }[]) => {
      viewStack = next;
      listeners.forEach(listener => listener());
    },
  };
});

vi.mock("@/systems/session/lib/session-display-title", () => ({
  getSessionDisplayTitle: (session: { id: string; name: string | null }) =>
    session.name ?? session.id,
}));

vi.mock("@/systems/session/hooks/use-session-create", () => ({
  useSessionCreateActions: () => ({ openForAgent: paletteMocks.openForAgent }),
}));

vi.mock("@/systems/workspace/hooks/use-worktrees", () => ({
  useWorktrees: () => ({ data: undefined }),
}));
vi.mock("@/systems/workspace/hooks/use-active-worktree", () => ({
  useScopedWorktreeFilter: () => ({ worktreeId: undefined, resolved: true }),
  useActiveWorktree: () => ({ selectedWorktreeId: null, activeWorktree: null, fallback: null }),
}));
vi.mock("@/systems/session/hooks/use-sessions", () => ({
  useSessions: (workspaceId: string | null) => {
    paletteMocks.sessionsWorkspaceId(workspaceId);
    return { data: paletteMocks.sessions, isLoading: paletteMocks.sessionsLoading };
  },
}));
vi.mock("@/systems/session/hooks/use-session-list-preferences", () => ({
  useSessionListPreferences: () => ({
    sort: "last_activity" as const,
    scope: paletteMocks.sessionListScope,
    setSort: vi.fn(),
    setScope: paletteMocks.setSessionListScope,
    loading: false,
    saving: paletteMocks.sessionListSaving,
  }),
}));
vi.mock("@/systems/session/hooks/use-workspace-session-groups", () => ({
  useWorkspaceSessionGroups: () => paletteMocks.workspaceGroups,
}));
vi.mock("../use-attention-jump", () => ({
  useAttentionJump: () => paletteMocks.jumpToSession,
}));

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({
    activeWorkspaceId: paletteMocks.activeWorkspaceId,
    runtimeWorkspaceId: paletteMocks.activeWorkspaceId,
    setActiveWorkspaceId: vi.fn(),
    workspaces: paletteMocks.workspaces,
    scope: "workspace" as const,
    pending: paletteMocks.pending,
    toggleLocked: paletteMocks.toggleLocked,
    canDisableGlobal: true,
    toggleGlobalScope: vi.fn(),
  }),
}));

vi.mock("../../hooks/use-desktop", () => ({
  useDesktop: <T,>(selector: (state: OsDesktopRuntimeStore) => T) => {
    if (paletteMocks.desktop === null) throw new Error("Desktop fixture was not configured.");
    return selector(paletteMocks.desktop);
  },
}));

vi.mock("../../hooks/use-os-shell", () => ({
  useOsShell: () => ({
    coordinator: paletteMocks.coordinator,
    manager: {
      closeWindow: paletteMocks.closeWindow,
      getState: desktopState,
      groupWindows: vi.fn(),
      reopenWindow: vi.fn(),
      toggleFloating: vi.fn(),
    },
  }),
}));

vi.mock("../../hooks/use-os-window-commands", () => ({
  useOsWindowCommands: () => paletteMocks.windowCommands,
}));

vi.mock("../../hooks/use-window-manager-store", async () => {
  const { useSyncExternalStore } = await import("react");
  return {
    useWindowPaletteIntent: () => paletteMocks.paletteIntent,
    useWindowPaletteViewStack: () =>
      useSyncExternalStore(
        paletteMocks.subscribeViewStack,
        paletteMocks.readViewStack,
        paletteMocks.readViewStack
      ),
  };
});

vi.mock("../../lib/attention-model", () => ({
  isNeedsYouSession: (session: { id: string }) => paletteMocks.isWaiting(session.id),
}));

vi.mock("../../lib/window-slot-registry", () => ({
  pruneWindowSlotStores: vi.fn(),
  subscribeWindowSlotRegistry: () => () => undefined,
  windowSlotRegistryVersion: () => 0,
  windowSlotSnapshot: (windowId: string) => paletteMocks.windowSlots.get(windowId) ?? null,
}));

vi.mock("../../stores/window-manager-store", async () => {
  const { popPaletteViewFrame, pushPaletteViewFrame } =
    await import("../../lib/palette-view-stack");
  return {
    windowManagerStore: {
      trigger: {
        paletteIntentCleared: paletteMocks.paletteIntentCleared,
        paletteIntentRequested: paletteMocks.paletteIntentRequested,
        paletteViewPushed: ({ viewId }: { viewId: "sessions" }) =>
          paletteMocks.writeViewStack(
            pushPaletteViewFrame(paletteMocks.readViewStack(), viewId) as Array<{
              viewId: "sessions";
            }>
          ),
        paletteViewPopped: () =>
          paletteMocks.writeViewStack(
            popPaletteViewFrame(paletteMocks.readViewStack()) as Array<{ viewId: "sessions" }>
          ),
        paletteViewStackSet: ({ stack }: { stack: readonly { viewId: "sessions" }[] }) =>
          paletteMocks.writeViewStack(stack),
      },
    },
  };
});

function windowFixture(overrides: Partial<OsWindow> = {}): OsWindow {
  return {
    app: "tasks",
    desktopId: "desktop:alpha",
    groupId: null,
    id: "window:tasks",
    instanceKey: null,
    layer: 1,
    minimized: false,
    navStack: [],
    nodeId: null,
    parentAxis: null,
    pinned: false,
    placement: "floating",
    rect: { h: 400, w: 600, x: 20, y: 20 },
    route: { pathname: "/tasks", search: {} },
    stackActive: true,
    stackId: null,
    ...overrides,
  };
}

const DESKTOPS: readonly LayoutDesktop[] = [
  {
    floating: [],
    floatingStacks: [],
    focusOwner: null,
    groups: [],
    id: "desktop:alpha",
    name: "Alpha",
    order: 0,
    purpose: "standard",
  },
  {
    floating: [],
    floatingStacks: [],
    focusOwner: null,
    groups: [],
    id: "desktop:beta",
    name: "Beta",
    order: 1,
    purpose: "standard",
  },
];

function desktopState(): OsDesktopRuntimeStore {
  if (paletteMocks.desktop === null) throw new Error("Desktop fixture was not configured.");
  return paletteMocks.desktop;
}

function desktopFixture(
  windows: Record<string, OsWindow>,
  focusedId: string | null
): OsDesktopRuntimeStore {
  return {
    activeDesktopId: "desktop:alpha",
    client: null,
    connectionStatus: "connected",
    desktopBounds: null,
    desktops: DESKTOPS,
    dockMagnify: false,
    focusedId,
    frames: {},
    hydration: "live",
    presentation: "floating",
    projections: {},
    railCollapsedAgentIds: [],
    reduceMotion: false,
    snapshot: null,
    viewportState: "ready",
    wallpaper: "ember",
    windowManagerConfig: null,
    windows,
  };
}

function PaletteHarness({ children }: { children: ReactNode }) {
  return <>{children}</>;
}

describe("useOsCommandPalette", () => {
  beforeEach(() => {
    paletteMocks.activeWorkspaceId = "workspace:alpha";
    paletteMocks.closeWindow.mockClear();
    paletteMocks.coordinator.userActivateWindow.mockClear();
    paletteMocks.coordinator.userOpen.mockClear();
    paletteMocks.coordinator.userOpen.mockResolvedValue("window:new-tab");
    paletteMocks.isWaiting.mockReset();
    paletteMocks.isWaiting.mockReturnValue(false);
    paletteMocks.paletteIntent = null;
    paletteMocks.paletteIntentCleared.mockClear();
    paletteMocks.paletteIntentRequested.mockClear();
    paletteMocks.pending = false;
    paletteMocks.sessions = [];
    paletteMocks.sessionsLoading = false;
    paletteMocks.sessionsWorkspaceId.mockClear();
    paletteMocks.sessionListScope = "recent";
    paletteMocks.sessionListSaving = false;
    paletteMocks.setSessionListScope.mockClear();
    paletteMocks.jumpToSession.mockClear();
    paletteMocks.workspaceGroups = [];
    paletteMocks.writeViewStack([]);
    paletteMocks.toggleLocked = false;
    paletteMocks.windowSlots.clear();
    paletteMocks.windowCommands.commandsAvailable = true;
    paletteMocks.desktop = desktopFixture({ "window:tasks": windowFixture() }, "window:tasks");
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("Should create each new tab inside the focused frame and scope its picker [UT-080]", async () => {
    const onOpenChange = vi.fn();
    const { result } = renderHook(() => useOsCommandPalette(true, onOpenChange), {
      wrapper: PaletteHarness,
    });

    await act(async () => {
      result.current.newTab();
      result.current.newTab();
    });

    await waitFor(() => {
      expect(paletteMocks.coordinator.userOpen).toHaveBeenCalledTimes(2);
    });
    expect(paletteMocks.coordinator.userOpen).toHaveBeenNthCalledWith(1, {
      app: "new-tab",
      stackTargetWindowId: "window:tasks",
    });
    expect(paletteMocks.coordinator.userOpen).toHaveBeenNthCalledWith(2, {
      app: "new-tab",
      stackTargetWindowId: "window:tasks",
    });
    expect(paletteMocks.paletteIntentRequested).toHaveBeenCalledTimes(2);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("Should keep a dismissed destination tab intact and open standalone without a focus [UT-081, UT-082]", () => {
    paletteMocks.paletteIntent = { kind: "destination", windowId: "window:new-tab" };
    paletteMocks.desktop = desktopFixture({}, null);
    const onOpenChange = vi.fn();
    const { result, rerender } = renderHook(({ open }) => useOsCommandPalette(open, onOpenChange), {
      initialProps: { open: true },
      wrapper: PaletteHarness,
    });

    expect(result.current.destinationWindowId).toBe("window:new-tab");
    rerender({ open: false });
    expect(result.current.destinationWindowId).toBeNull();
    expect(paletteMocks.closeWindow).not.toHaveBeenCalled();

    act(() => {
      result.current.newTab();
    });
    expect(paletteMocks.coordinator.userOpen).toHaveBeenCalledWith({ app: "new-tab" });
  });

  it("Should preserve the selected tab route by activating its existing window [UT-090]", async () => {
    paletteMocks.desktop = desktopFixture(
      {
        "window:deep-task": windowFixture({
          id: "window:deep-task",
          navStack: [{ pathname: "/tasks", search: {} }],
          route: { pathname: "/tasks/task-42", search: { pane: "activity" } },
        }),
      },
      "window:deep-task"
    );
    const { result } = renderHook(() => useOsCommandPalette(true, vi.fn()), {
      wrapper: PaletteHarness,
    });

    await act(async () => {
      result.current.goToTab("window:deep-task");
    });

    expect(paletteMocks.coordinator.userActivateWindow).toHaveBeenCalledWith("window:deep-task");
  });

  it("Should use the runtime workspace and lock the Global toggle while scope resolves", () => {
    paletteMocks.pending = true;

    render(<OsCommandPalette open onOpenChange={vi.fn()} />);

    expect(paletteMocks.sessionsWorkspaceId).toHaveBeenCalledWith("workspace:alpha");
    expect(screen.getByTestId("os-palette-toggle-global-scope")).toHaveAttribute(
      "data-disabled",
      "true"
    );
  });

  it("Should render contextual Go to tab results, attention, and no empty group [UT-100]", () => {
    const alpha = windowFixture({ id: "window:alpha" });
    const beta = windowFixture({ desktopId: "desktop:beta", id: "window:beta" });
    const waiting = windowFixture({
      app: "session",
      desktopId: "desktop:beta",
      id: "window:waiting",
      instanceKey: "session:waiting",
      route: { pathname: "/agents/codex/sessions/session%3Awaiting", search: {} },
    });
    paletteMocks.sessions = [
      {
        agent_name: "codex",
        badge: "running",
        id: "session:waiting",
        name: "Needs input",
        updated_at: "2026-08-16T10:00:00Z",
      },
    ];
    paletteMocks.isWaiting.mockImplementation(
      (sessionId: string) => sessionId === "session:waiting"
    );
    paletteMocks.desktop = desktopFixture(
      { [alpha.id]: alpha, [beta.id]: beta, [waiting.id]: waiting },
      alpha.id
    );

    const rendered = render(<OsCommandPalette open onOpenChange={vi.fn()} />);

    expect(screen.getByText("Go to tab")).toBeInTheDocument();
    expect(screen.getByTestId("os-palette-tab-window:alpha")).toHaveTextContent("TasksAlpha");
    expect(screen.getByTestId("os-palette-tab-window:beta")).toHaveTextContent("TasksBeta");
    expect(screen.getByTestId("os-palette-tab-window:waiting")).toHaveTextContent(
      "Needs input1Beta"
    );

    paletteMocks.desktop = desktopFixture({}, null);
    rendered.rerender(<OsCommandPalette open onOpenChange={vi.fn()} />);
    expect(screen.queryByText("Go to tab")).not.toBeInTheDocument();
  });

  it("Should restore a minimized tab on its desktop through the single activation action [UT-101]", async () => {
    paletteMocks.desktop = desktopFixture(
      {
        "window:minimized": windowFixture({
          desktopId: "desktop:beta",
          id: "window:minimized",
          minimized: true,
        }),
      },
      null
    );
    const { result } = renderHook(() => useOsCommandPalette(true, vi.fn()), {
      wrapper: PaletteHarness,
    });

    await act(async () => {
      result.current.goToTab("window:minimized");
    });

    expect(paletteMocks.coordinator.userActivateWindow).toHaveBeenCalledWith("window:minimized");
  });
});

function paletteSession(
  overrides: Partial<PaletteSessionFixture> & { id: string }
): PaletteSessionFixture {
  return {
    name: null,
    agent_name: "claude",
    badge: "idle",
    workspace_id: "workspace:alpha",
    updated_at: "2026-08-16T10:00:00Z",
    ...overrides,
  };
}

function workspaceGroup(
  workspaceId: string,
  name: string,
  sessions: PaletteSessionFixture[],
  overrides: Partial<{ loading: boolean; failed: boolean }> = {}
) {
  return {
    workspaceId,
    workspaceName: name,
    sessions,
    total: sessions.length,
    loading: false,
    failed: false,
    retry: () => undefined,
    ...overrides,
  };
}

async function pushSessionsView(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByTestId("os-palette-view-sessions"));
  return screen.getByPlaceholderText("Search sessions…");
}

describe("palette nested views", () => {
  beforeEach(() => {
    paletteMocks.desktop = desktopFixture({ "window:tasks": windowFixture() }, "window:tasks");
    paletteMocks.writeViewStack([]);
    paletteMocks.sessions = [];
    paletteMocks.sessionListScope = "recent";
    paletteMocks.setSessionListScope.mockClear();
    paletteMocks.jumpToSession.mockClear();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("Should push a view, keep editing text, and pop only on an empty query [UT-059]", async () => {
    const user = userEvent.setup();
    paletteMocks.sessions = [paletteSession({ id: "s-1", name: "Refactor session store" })];
    render(<OsCommandPalette open onOpenChange={vi.fn()} />);
    expect(screen.queryByTestId("os-palette-breadcrumb")).not.toBeInTheDocument();

    const input = await pushSessionsView(user);
    expect(screen.getByTestId("os-palette-breadcrumb")).toHaveTextContent("Sessions");

    await user.type(input, "ref");
    expect(input).toHaveValue("ref");
    await user.keyboard("{Backspace}");
    expect(input).toHaveValue("re");
    expect(screen.getByTestId("os-palette-breadcrumb")).toBeInTheDocument();

    await user.clear(input);
    await user.keyboard("{Backspace}");
    expect(screen.queryByTestId("os-palette-breadcrumb")).not.toBeInTheDocument();
    expect(screen.getByTestId("os-palette-view-sessions")).toBeInTheDocument();
  });

  it("Should render only the active view's results and name its own empty state [UT-060]", async () => {
    const user = userEvent.setup();
    paletteMocks.sessions = [paletteSession({ id: "s-1", name: "Refactor session store" })];
    const rendered = render(<OsCommandPalette open onOpenChange={vi.fn()} />);
    expect(screen.getByTestId("os-palette-app-tasks")).toBeInTheDocument();

    await pushSessionsView(user);
    expect(screen.getByTestId("os-palette-session-view-s-1")).toBeInTheDocument();
    expect(screen.queryByTestId("os-palette-app-tasks")).not.toBeInTheDocument();
    expect(screen.queryByTestId("os-palette-toggle-global-scope")).not.toBeInTheDocument();
    expect(screen.queryByTestId("os-palette-view-sessions")).not.toBeInTheDocument();

    paletteMocks.sessions = [];
    rendered.rerender(<OsCommandPalette open onOpenChange={vi.fn()} />);
    expect(screen.getByText("No sessions in this workspace yet.")).toBeInTheDocument();
    expect(screen.getByTestId("os-palette-breadcrumb")).toBeInTheDocument();
  });

  it("Should narrow by chip, name a zero match, and clear it in one keystroke [UT-061]", async () => {
    const user = userEvent.setup();
    paletteMocks.sessions = [
      paletteSession({ id: "s-needs", name: "Chargeback flow", badge: "waiting-for-input" }),
      paletteSession({ id: "s-running", name: "Fix payment retries", badge: "running" }),
    ];
    render(<OsCommandPalette open onOpenChange={vi.fn()} />);
    const input = await pushSessionsView(user);

    expect(screen.getByTestId("os-palette-session-filter-all")).toHaveTextContent("All2");
    expect(screen.getByTestId("os-palette-session-filter-finished")).toHaveTextContent("Finished0");

    await user.click(screen.getByTestId("os-palette-session-filter-needs-you"));
    expect(screen.getByTestId("os-palette-session-view-s-needs")).toBeInTheDocument();
    expect(screen.queryByTestId("os-palette-session-view-s-running")).not.toBeInTheDocument();

    await user.click(screen.getByTestId("os-palette-session-filter-finished"));
    expect(screen.getByText("Nothing finished right now.")).toBeInTheDocument();

    await user.click(input);
    await user.keyboard("{Backspace}");
    expect(screen.getByTestId("os-palette-session-view-s-needs")).toBeInTheDocument();
    expect(screen.getByTestId("os-palette-session-view-s-running")).toBeInTheDocument();
    expect(screen.getByTestId("os-palette-breadcrumb")).toBeInTheDocument();
  });

  it("Should widen through the operator's persisted session-list scope [UT-061]", async () => {
    const user = userEvent.setup();
    paletteMocks.sessions = [paletteSession({ id: "s-alpha", name: "Refactor session store" })];
    const rendered = render(<OsCommandPalette open onOpenChange={vi.fn()} />);
    await pushSessionsView(user);

    const scope = screen.getByTestId("os-palette-session-scope");
    expect(scope).toHaveAttribute("aria-pressed", "false");
    await user.click(scope);
    expect(paletteMocks.setSessionListScope).toHaveBeenCalledWith("all-workspaces");

    paletteMocks.sessionListScope = "all-workspaces";
    paletteMocks.workspaceGroups = [
      workspaceGroup("workspace:alpha", "Alpha", [
        paletteSession({ id: "s-alpha", name: "Refactor session store" }),
      ]),
      workspaceGroup("workspace:beta", "Beta", [
        paletteSession({
          id: "s-beta",
          name: "Chargeback flow",
          workspace_id: "workspace:beta",
          badge: "waiting-for-input",
        }),
      ]),
      workspaceGroup("workspace:gamma", "Gamma", [], { failed: true }),
    ];
    rendered.rerender(<OsCommandPalette open onOpenChange={vi.fn()} />);

    expect(screen.getByTestId("os-palette-session-scope")).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByTestId("os-palette-session-view-s-beta")).toHaveTextContent("Beta");
    expect(screen.getByText("Gamma couldn't be loaded.")).toBeInTheDocument();
    // Attention-first survives the union: the needs-you row from the foreign
    // workspace outranks the idle row from this one.
    const rows = screen.getAllByTestId(/^os-palette-session-view-/);
    expect(rows.map(row => row.dataset.testid)).toEqual([
      "os-palette-session-view-s-beta",
      "os-palette-session-view-s-alpha",
    ]);

    await user.click(screen.getByTestId("os-palette-session-scope"));
    expect(paletteMocks.setSessionListScope).toHaveBeenLastCalledWith("all");

    paletteMocks.workspaceGroups = [];
    rendered.rerender(<OsCommandPalette open onOpenChange={vi.fn()} />);
    expect(screen.getByText("No sessions across workspaces yet.")).toBeInTheDocument();
  });

  it("Should land on a session through the shared attention jump [UT-060]", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    paletteMocks.sessions = [
      paletteSession({ id: "s-1", name: "Refactor session store", agent_name: "codex" }),
    ];
    render(<OsCommandPalette open onOpenChange={onOpenChange} />);
    await pushSessionsView(user);

    await user.click(screen.getByTestId("os-palette-session-view-s-1"));

    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(paletteMocks.jumpToSession).toHaveBeenCalledWith({
      sessionId: "s-1",
      agentName: "codex",
      workspaceId: "workspace:alpha",
    });
    expect(paletteMocks.coordinator.userOpen).not.toHaveBeenCalled();
  });

  it("Should keep the keyboard selection while the catalog moves underneath it [UT-076]", async () => {
    const user = userEvent.setup();
    const stamped = (id: string, minute: string) =>
      paletteSession({ id, name: id, attention_changed_at: `2026-08-16T10:0${minute}:00Z` });
    paletteMocks.sessions = [stamped("s-a", "3"), stamped("s-b", "2"), stamped("s-c", "1")];
    const rendered = render(<OsCommandPalette open onOpenChange={vi.fn()} />);
    await pushSessionsView(user);

    await waitFor(() => {
      expect(screen.getByTestId("os-palette-session-view-s-a")).toHaveAttribute(
        "aria-selected",
        "true"
      );
    });
    await user.keyboard("{ArrowDown}");
    expect(screen.getByTestId("os-palette-session-view-s-b")).toHaveAttribute(
      "aria-selected",
      "true"
    );

    paletteMocks.sessions = [
      stamped("s-new", "4"),
      stamped("s-a", "3"),
      stamped("s-b", "2"),
      stamped("s-c", "1"),
    ];
    rendered.rerender(<OsCommandPalette open onOpenChange={vi.fn()} />);
    expect(screen.getByTestId("os-palette-session-view-s-b")).toHaveAttribute(
      "aria-selected",
      "true"
    );

    paletteMocks.sessions = [stamped("s-new", "4"), stamped("s-a", "3"), stamped("s-c", "1")];
    rendered.rerender(<OsCommandPalette open onOpenChange={vi.fn()} />);
    expect(screen.getByTestId("os-palette-session-view-s-c")).toHaveAttribute(
      "aria-selected",
      "true"
    );
  });

  it("Should bound the rendered rows and say how many matches went unrendered", async () => {
    const user = userEvent.setup();
    paletteMocks.sessions = Array.from({ length: PALETTE_SESSION_ROW_LIMIT + 12 }, (_, index) =>
      paletteSession({ id: `s-${index}`, name: `Session ${index}` })
    );
    render(<OsCommandPalette open onOpenChange={vi.fn()} />);
    await pushSessionsView(user);

    expect(screen.getAllByTestId(/^os-palette-session-view-/)).toHaveLength(
      PALETTE_SESSION_ROW_LIMIT
    );
    const list = screen.getByTestId("os-command-palette");
    expect(
      within(list).getByText(
        `Showing ${PALETTE_SESSION_ROW_LIMIT} of ${PALETTE_SESSION_ROW_LIMIT + 12} — keep typing to narrow.`,
        { exact: false }
      )
    ).toBeInTheDocument();
  });
});
