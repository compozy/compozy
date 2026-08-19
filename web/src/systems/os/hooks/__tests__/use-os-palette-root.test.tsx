// Suite: OS command palette root
// Invariant: every row is a projection of the one client registry, entity rows
// preserve the selected tab's identity, session landing goes through the shared
// attention jump exactly once (BR-20), destination mode offers only navigable
// targets, and a pushed view owns the surface completely — its own results, its
// own filters, and a keyboard selection that survives the catalog moving.
// Owning layer: palette root view-model and presentation boundary.
// Boundary OUT: the dispatch seam and client-op table (cmd-palette-dispatch),
// availability evaluation (cmd-palette-availability), overlay lifetime
// (use-desktop-overlays), stack and filter mechanics (palette-view-stack,
// palette-session-filters), and daemon command transport.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { act, render, renderHook, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { LayoutDesktop } from "../../lib/window-manager-types";
import type { OsDesktopRuntimeStore, OsWindow } from "../../lib/os-types";
import { OsCommandPalette } from "../../components/os-command-palette";
import { useOsPaletteRoot } from "../use-os-palette-root";
import { CmdPaletteRegistryProvider } from "../../contexts/cmd-palette-registry-context";
import type {
  CmdPaletteRankSignals,
  PaletteRegistry,
  ResolvedPaletteCommand,
} from "../../lib/cmd-palette-types";
import { PALETTE_SESSION_ROW_LIMIT } from "../use-os-palette-sessions-view";
import {
  isPaletteDomainSearchEnabled,
  projectVaultRows,
  type OsPaletteDomainSection,
} from "../use-os-palette-domain-search";

const TEST_WEIGHTS = JSON.parse(
  readFileSync(
    resolve(process.cwd(), "../internal/cmdpalette/testdata/ranking_weights_v1.json"),
    "utf8"
  )
) as CmdPaletteRankSignals["weights"];

const TEST_RANK_SIGNALS: CmdPaletteRankSignals = {
  weights: TEST_WEIGHTS,
  usage: [],
  query_hits: [],
  pins: [],
  revision: "ps_test",
};

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
    sessionListScope: "workspace" as "workspace" | "all-workspaces",
    sessionListSaving: false,
    sessions: [] as PaletteSessionFixture[],
    sessionsLoading: false,
    sessionsWorkspaceId: vi.fn(),
    sessionsFilters: vi.fn(),
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
    rankSignals: null as CmdPaletteRankSignals | null,
    domainSections: [] as OsPaletteDomainSection[],
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
  useWorktreeListings: () => ({}),
}));
vi.mock("@/systems/workspace/hooks/use-active-worktree", () => ({
  useScopedWorktreeFilter: () => ({ worktreeId: undefined, resolved: true }),
  useActiveWorktree: () => ({ selectedWorktreeId: null, activeWorktree: null, fallback: null }),
}));
vi.mock("@/systems/session/hooks/use-sessions", () => ({
  useSessions: (workspaceId: string | null, options?: { filters?: Record<string, unknown> }) => {
    paletteMocks.sessionsWorkspaceId(workspaceId);
    paletteMocks.sessionsFilters(options?.filters ?? {});
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

vi.mock("../use-cmd-palette-rank-signals", () => ({
  useCmdPaletteRankSignals: () => ({
    data: paletteMocks.rankSignals,
    loading: false,
    failed: false,
  }),
}));

vi.mock("../use-os-palette-domain-search", async importOriginal => ({
  ...(await importOriginal<typeof import("../use-os-palette-domain-search")>()),
  useOsPaletteDomainSearch: () => paletteMocks.domainSections,
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
    reduceMotion: false,
    snapshot: null,
    viewportState: "ready",
    wallpaper: "ember",
    windowManagerConfig: null,
    windows,
  };
}

const PALETTE_REGISTRY: PaletteRegistry = (() => {
  const commands = [
    { id: "window.tab.new", title: "New tab", section: "Tabs" },
    { id: "app.open.tasks", title: "Open Tasks", section: "Apps", app: "tasks" },
    { id: "app.open.agents", title: "Open Agents", section: "Apps", app: "agents" },
    { id: "palette.view.sessions", title: "Sessions", section: "Views", view: "sessions" },
  ].map(entry => ({
    id: entry.id,
    title: entry.title,
    section: entry.section,
    icon: "command",
    source: "core",
    bindings: [],
    alias: null,
    destructive: false,
    availability_exempt: false,
    arguments: [],
    action: entry.app
      ? { kind: "navigate", app: entry.app }
      : entry.view
        ? { kind: "view", view: entry.view }
        : { kind: "client_op", op: entry.id },
    execution: { retry_safe: true, single_flight: false },
    visible: true,
    available: true,
    reason: "",
    chords: [],
  })) as unknown as ResolvedPaletteCommand[];
  return {
    commands,
    byId: new Map(commands.map(command => [command.id, command])),
    sources: [{ source: "core", status: "healthy" }],
    catalogRevision: "sha256:test",
    stale: false,
    daemonReachable: true,
  };
})();

function PaletteHarness({ children }: { children: ReactNode }) {
  return (
    <CmdPaletteRegistryProvider registry={PALETTE_REGISTRY}>{children}</CmdPaletteRegistryProvider>
  );
}

const paletteDispatch = {
  // Stands in for the seam: a `view` action pushes the stack, exactly as
  // `dispatchPaletteCommand` routes it through the shell's `pushView` port.
  run: vi.fn(
    async (
      command: ResolvedPaletteCommand,
      _args?: Readonly<Record<string, unknown>>,
      _query?: string
    ) => {
      const view = command.action.kind === "view" ? command.action.view : undefined;
      if (view) paletteMocks.writeViewStack([{ viewId: view as "sessions" }]);
      return { status: "ran" } as const;
    }
  ),
  runById: vi.fn(async (_commandId: string) => ({ status: "ran" }) as const),
  executeClientOp: vi.fn(async (_op: string, _payload: unknown) => undefined),
};

function renderPalette() {
  return render(
    <PaletteHarness>
      <OsCommandPalette open dispatch={paletteDispatch} onOpenChange={vi.fn()} />
    </PaletteHarness>
  );
}

function renderRoot(open = true, onOpenChange = vi.fn()) {
  return renderHook(
    (props: { open: boolean }) =>
      useOsPaletteRoot({
        open: props.open,
        onOpenChange,
        dispatch: (command, query) => void paletteDispatch.run(command, undefined, query),
      }),
    { initialProps: { open }, wrapper: PaletteHarness }
  );
}

describe("useOsPaletteRoot", () => {
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
    paletteMocks.rankSignals = null;
    paletteMocks.domainSections = [];
    paletteMocks.sessions = [];
    paletteMocks.sessionsLoading = false;
    paletteMocks.sessionsWorkspaceId.mockClear();
    paletteMocks.sessionListScope = "workspace";
    paletteMocks.sessionListSaving = false;
    paletteMocks.setSessionListScope.mockClear();
    paletteMocks.jumpToSession.mockClear();
    paletteMocks.workspaceGroups = [];
    paletteMocks.writeViewStack([]);
    paletteMocks.toggleLocked = false;
    paletteMocks.windowSlots.clear();
    paletteMocks.windowCommands.commandsAvailable = true;
    paletteMocks.desktop = desktopFixture({ "window:tasks": windowFixture() }, "window:tasks");
    paletteDispatch.run.mockClear();
    paletteDispatch.runById.mockClear();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("Should render every row from the registry projection and dispatch through the seam", async () => {
    const user = userEvent.setup();
    renderPalette();

    expect(screen.getByTestId("os-palette-command-app.open.tasks")).toBeInTheDocument();
    await user.click(screen.getByTestId("os-palette-command-app.open.tasks"));

    expect(paletteDispatch.run).toHaveBeenCalledOnce();
    expect(paletteDispatch.run.mock.lastCall?.[0]).toMatchObject({ id: "app.open.tasks" });
  });

  it("Should scope the picker to the destination tab and release it on close [UT-081, UT-082]", () => {
    paletteMocks.paletteIntent = { kind: "destination", windowId: "window:new-tab" };
    paletteMocks.desktop = desktopFixture({}, null);
    const { result, rerender } = renderRoot();

    expect(result.current.destinationWindowId).toBe("window:new-tab");
    rerender({ open: false });
    // Dismissing the palette abandons the intent without destroying the tab.
    expect(result.current.destinationWindowId).toBeNull();
    expect(paletteMocks.closeWindow).not.toHaveBeenCalled();
  });

  it("Should hand the empty tab's place to the picked session [UT-080]", async () => {
    paletteMocks.paletteIntent = { kind: "destination", windowId: "window:new-tab" };
    paletteMocks.desktop = desktopFixture({}, null);
    const { result } = renderRoot();

    await act(async () => {
      result.current.openSession({
        sessionId: "session:one",
        title: "One",
        agentName: "codex",
        workspaceId: "workspace:alpha",
        route: { pathname: "/agents/codex/sessions/session%3Aone", search: {} },
      });
    });

    expect(paletteMocks.paletteIntentCleared).toHaveBeenCalledOnce();
    expect(paletteMocks.coordinator.userOpen).toHaveBeenCalledWith({
      app: "session",
      instanceKey: "session:one",
      route: { pathname: "/agents/codex/sessions/session%3Aone", search: {} },
      stackTargetWindowId: "window:new-tab",
    });
    await waitFor(() => expect(paletteMocks.closeWindow).toHaveBeenCalledWith("window:new-tab"));
    // BR-20: destination picking is not a landing, so the shared jump stays out.
    expect(paletteMocks.jumpToSession).not.toHaveBeenCalled();
  });

  it("Should land on a session through the shared attention jump [BR-20]", async () => {
    const { result } = renderRoot();

    await act(async () => {
      result.current.openSession({
        sessionId: "session:one",
        title: "One",
        agentName: "codex",
        workspaceId: "workspace:alpha",
        route: { pathname: "/agents/codex/sessions/session%3Aone", search: {} },
      });
    });

    expect(paletteMocks.jumpToSession).toHaveBeenCalledExactlyOnceWith({
      sessionId: "session:one",
      agentName: "codex",
      workspaceId: "workspace:alpha",
    });
    // The root must not keep a second landing implementation of its own.
    expect(paletteMocks.coordinator.userOpen).not.toHaveBeenCalled();
  });

  it("Should offer only navigable targets in destination mode and stay honest when none are eligible [UT-107, UT-108]", () => {
    paletteMocks.paletteIntent = { kind: "destination", windowId: "window:new-tab" };
    paletteMocks.desktop = desktopFixture({}, null);
    const { result } = renderRoot();

    expect(result.current.destination).toBe(true);
    const offered = result.current.sections.flatMap(section =>
      section.commands.map(command => command.action.kind)
    );
    expect(offered.length).toBeGreaterThan(0);
    expect(new Set(offered)).toEqual(new Set(["navigate"]));
    // Tabs and worktrees are absent in destination mode, not disabled.
    expect(result.current.entities.tabs).toHaveLength(0);
    expect(result.current.entities.worktrees).toHaveLength(0);

    act(() => result.current.setQuery("zzzz"));
    expect(result.current.sections).toHaveLength(0);
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
    const { result } = renderRoot();

    await act(async () => {
      result.current.goToTab("window:deep-task");
    });

    expect(paletteMocks.coordinator.userActivateWindow).toHaveBeenCalledWith("window:deep-task");
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

    const rendered = renderPalette();

    expect(screen.getByText("Go to tab")).toBeInTheDocument();
    expect(screen.getByTestId("os-palette-tab-window:alpha")).toHaveTextContent("TasksAlpha");
    expect(screen.getByTestId("os-palette-tab-window:beta")).toHaveTextContent("TasksBeta");
    expect(screen.getByTestId("os-palette-tab-window:waiting")).toHaveTextContent(
      "Needs input1Beta"
    );

    paletteMocks.desktop = desktopFixture({}, null);
    rendered.rerender(
      <PaletteHarness>
        <OsCommandPalette open dispatch={paletteDispatch} onOpenChange={vi.fn()} />
      </PaletteHarness>
    );
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
    const { result } = renderRoot();

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
  await user.click(screen.getByTestId("os-palette-command-palette.view.sessions"));
  return screen.getByPlaceholderText("Search sessions…");
}

describe("palette nested views", () => {
  beforeEach(() => {
    paletteMocks.desktop = desktopFixture({ "window:tasks": windowFixture() }, "window:tasks");
    paletteMocks.writeViewStack([]);
    paletteMocks.sessions = [];
    paletteMocks.sessionListScope = "workspace";
    paletteMocks.setSessionListScope.mockClear();
    paletteMocks.jumpToSession.mockClear();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("Should push a view, keep editing text, and pop only on an empty query [UT-059]", async () => {
    const user = userEvent.setup();
    paletteMocks.sessions = [paletteSession({ id: "s-1", name: "Refactor session store" })];
    renderPalette();
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
    expect(screen.getByTestId("os-palette-command-palette.view.sessions")).toBeInTheDocument();
  });

  it("Should render only the active view's results and name its own empty state [UT-060]", async () => {
    const user = userEvent.setup();
    paletteMocks.sessions = [paletteSession({ id: "s-1", name: "Refactor session store" })];
    const rendered = renderPalette();
    expect(screen.getByTestId("os-palette-command-app.open.tasks")).toBeInTheDocument();

    await pushSessionsView(user);
    expect(screen.getByTestId("os-palette-session-view-s-1")).toBeInTheDocument();
    // No root row bleeds into the level (BR-32): the view owns the surface.
    expect(screen.queryByTestId("os-palette-command-app.open.tasks")).not.toBeInTheDocument();
    expect(
      screen.queryByTestId("os-palette-command-palette.view.sessions")
    ).not.toBeInTheDocument();

    paletteMocks.sessions = [];
    rendered.rerender(
      <PaletteHarness>
        <OsCommandPalette open dispatch={paletteDispatch} onOpenChange={vi.fn()} />
      </PaletteHarness>
    );
    expect(screen.getByText("No sessions in this workspace yet.")).toBeInTheDocument();
    expect(screen.getByTestId("os-palette-breadcrumb")).toBeInTheDocument();
  });

  it("Should narrow by chip, name a zero match, and clear it in one keystroke [UT-061]", async () => {
    const user = userEvent.setup();
    paletteMocks.sessions = [
      paletteSession({ id: "s-needs", name: "Chargeback flow", badge: "waiting-for-input" }),
      paletteSession({ id: "s-running", name: "Fix payment retries", badge: "running" }),
    ];
    renderPalette();
    const input = await pushSessionsView(user);

    expect(screen.getByTestId("os-palette-session-filter-all")).toHaveTextContent(/All\s*2/);
    expect(screen.getByTestId("os-palette-session-filter-finished")).toHaveTextContent(
      /Finished\s*0/
    );

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

  it("Should widen through the operator's persisted session-list scope [UT-099]", async () => {
    const user = userEvent.setup();
    paletteMocks.sessions = [paletteSession({ id: "s-alpha", name: "Refactor session store" })];
    const rendered = renderPalette();
    await pushSessionsView(user);

    const scope = screen.getByTestId("os-palette-session-scope");
    expect(scope).toHaveAccessibleName("All workspaces");
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
    rendered.rerender(
      <PaletteHarness>
        <OsCommandPalette open dispatch={paletteDispatch} onOpenChange={vi.fn()} />
      </PaletteHarness>
    );

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
    expect(paletteMocks.setSessionListScope).toHaveBeenLastCalledWith("workspace");

    paletteMocks.workspaceGroups = [];
    rendered.rerender(
      <PaletteHarness>
        <OsCommandPalette open dispatch={paletteDispatch} onOpenChange={vi.fn()} />
      </PaletteHarness>
    );
    expect(screen.getByText("No sessions across workspaces yet.")).toBeInTheDocument();
  });

  it("Should read the archive through its own toggle without touching the breadth [UT-061]", async () => {
    const user = userEvent.setup();
    paletteMocks.sessions = [paletteSession({ id: "s-alpha", name: "Refactor session store" })];
    renderPalette();
    await pushSessionsView(user);

    const archived = screen.getByTestId("os-palette-session-archived");
    expect(archived).toHaveAccessibleName("Archived");
    expect(archived).toHaveAttribute("aria-pressed", "false");
    expect(paletteMocks.sessionsFilters).toHaveBeenLastCalledWith(
      expect.not.objectContaining({ archive: expect.anything() })
    );

    await user.click(archived);

    expect(screen.getByTestId("os-palette-session-archived")).toHaveAttribute(
      "aria-pressed",
      "true"
    );
    expect(paletteMocks.sessionsFilters).toHaveBeenLastCalledWith(
      expect.objectContaining({ archive: "only" })
    );
    // The archive is its own axis: flipping it never writes the persisted breadth.
    expect(paletteMocks.setSessionListScope).not.toHaveBeenCalled();
  });

  it("Should land on a session through the shared attention jump [UT-060]", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    paletteMocks.sessions = [
      paletteSession({ id: "s-1", name: "Refactor session store", agent_name: "codex" }),
    ];
    render(
      <PaletteHarness>
        <OsCommandPalette open dispatch={paletteDispatch} onOpenChange={onOpenChange} />
      </PaletteHarness>
    );
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

  it("Should keep the keyboard selection while the session catalog moves underneath it [UT-076]", async () => {
    const user = userEvent.setup();
    const stamped = (id: string, minute: string) =>
      paletteSession({ id, name: id, attention_changed_at: `2026-08-16T10:0${minute}:00Z` });
    paletteMocks.sessions = [stamped("s-a", "3"), stamped("s-b", "2"), stamped("s-c", "1")];
    const rendered = renderPalette();
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
    rendered.rerender(
      <PaletteHarness>
        <OsCommandPalette open dispatch={paletteDispatch} onOpenChange={vi.fn()} />
      </PaletteHarness>
    );
    expect(screen.getByTestId("os-palette-session-view-s-b")).toHaveAttribute(
      "aria-selected",
      "true"
    );

    paletteMocks.sessions = [stamped("s-new", "4"), stamped("s-a", "3"), stamped("s-c", "1")];
    rendered.rerender(
      <PaletteHarness>
        <OsCommandPalette open dispatch={paletteDispatch} onOpenChange={vi.fn()} />
      </PaletteHarness>
    );
    expect(screen.getByTestId("os-palette-session-view-s-c")).toHaveAttribute(
      "aria-selected",
      "true"
    );
  });

  it("Should keep the selected domain row when an earlier async domain arrives [UT-114]", async () => {
    const user = userEvent.setup();
    const taskRows = ["alpha", "beta"].map(id => ({
      key: `task:${id}`,
      label: `Task ${id}`,
      app: "tasks" as const,
      route: { pathname: "/tasks", search: { q: id } },
    }));
    paletteMocks.domainSections = [
      { title: "Tasks", rows: taskRows, total: taskRows.length, loading: false, error: null },
    ];
    const rendered = renderPalette();

    const selectedTask = screen.getByTestId("os-palette-domain-row-task:beta");
    await user.hover(selectedTask);
    expect(selectedTask).toHaveAttribute("aria-selected", "true");

    paletteMocks.domainSections = [
      {
        title: "Agents",
        rows: [
          {
            key: "agent:codex",
            label: "Codex",
            app: "agents",
            route: { pathname: "/agents", search: {} },
          },
        ],
        total: 1,
        loading: false,
        error: null,
      },
      { title: "Tasks", rows: taskRows, total: taskRows.length, loading: false, error: null },
    ];
    rendered.rerender(
      <PaletteHarness>
        <OsCommandPalette open dispatch={paletteDispatch} onOpenChange={vi.fn()} />
      </PaletteHarness>
    );

    expect(screen.getByTestId("os-palette-domain-row-task:beta")).toHaveAttribute(
      "aria-selected",
      "true"
    );
  });

  it("Should bound the rendered rows and say how many matches went unrendered", async () => {
    const user = userEvent.setup();
    paletteMocks.sessions = Array.from({ length: PALETTE_SESSION_ROW_LIMIT + 12 }, (_, index) =>
      paletteSession({ id: `s-${index}`, name: `Session ${index}` })
    );
    renderPalette();
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

  it("Should gate every domain read by the daemon query bounds [UT-110, UT-111]", () => {
    expect(isPaletteDomainSearchEnabled(true, "ab", TEST_WEIGHTS)).toBe(true);
    expect(isPaletteDomainSearchEnabled(true, "a", TEST_WEIGHTS)).toBe(false);
    expect(
      isPaletteDomainSearchEnabled(
        true,
        "x".repeat(TEST_WEIGHTS.max_query_length + 1),
        TEST_WEIGHTS
      )
    ).toBe(false);
    expect(isPaletteDomainSearchEnabled(false, "ab", TEST_WEIGHTS)).toBe(false);
  });

  it("Should project Vault names and metadata without any secret value [UT-112]", () => {
    type HasValue = "value" extends keyof ReturnType<typeof projectVaultRows>[number]
      ? true
      : false;
    const hasValue: HasValue = false;
    const rows = projectVaultRows(
      [
        {
          ref: "vault:providers/codex/api_key",
          namespace: "providers",
          kind: "api_key",
          present: true,
          created_at: "2026-08-19T12:00:00Z",
          updated_at: "2026-08-19T12:00:00Z",
          value: "must-never-cross-the-projection",
        } as never,
      ],
      "workspace",
      new Map()
    );

    expect(hasValue).toBe(false);
    expect(rows).toEqual([
      expect.objectContaining({ label: "api_key", namespace: "providers", kind: "api_key" }),
    ]);
    expect(rows[0]).not.toHaveProperty("value");
  });

  it("Should cap a domain, preserve siblings, name failures, and label global rows [UT-113, UT-115, UT-116]", () => {
    paletteMocks.domainSections = [
      {
        title: "Tasks",
        rows: Array.from({ length: TEST_WEIGHTS.entity_section_visible_cap }, (_, index) => ({
          key: `task:${index}`,
          label: `Task ${index}`,
          workspaceLabel: "Alpha",
          app: "tasks" as const,
          route: { pathname: "/tasks", search: { q: `Task ${index}` } },
        })),
        total: TEST_WEIGHTS.entity_section_visible_cap + 4,
        loading: false,
        error: null,
      },
      {
        title: "Bridges",
        rows: [],
        total: 0,
        loading: false,
        error: "Bridges: transport offline",
      },
    ];
    renderPalette();

    expect(screen.getAllByTestId(/^os-palette-domain-row-task:/)).toHaveLength(
      TEST_WEIGHTS.entity_section_visible_cap
    );
    expect(
      screen.getByText(
        `showing ${TEST_WEIGHTS.entity_section_visible_cap} of ${TEST_WEIGHTS.entity_section_visible_cap + 4}`
      )
    ).toBeInTheDocument();
    expect(screen.getByTestId("os-palette-domain-error-bridges")).toHaveTextContent(
      "Bridges: transport offline"
    );
    for (const row of screen.getAllByTestId(/^os-palette-domain-row-task:/)) {
      expect(within(row).getByText("Alpha")).toBeInTheDocument();
    }
  });

  it("Should keep matching commands and settings destinations in separate groups [UT-117]", async () => {
    const user = userEvent.setup();
    paletteMocks.rankSignals = TEST_RANK_SIGNALS;
    paletteMocks.domainSections = [
      {
        title: "Settings",
        rows: [
          {
            key: "settings:shortcuts",
            label: "Keyboard shortcuts",
            app: "settings",
            route: { pathname: "/settings/shortcuts", search: {} },
          },
        ],
        total: 1,
        loading: false,
        error: null,
      },
    ];
    const shortcut = {
      ...PALETTE_REGISTRY.commands[0],
      id: "cheatsheet.shortcuts",
      title: "Keyboard shortcuts",
      section: "Commands",
    } as ResolvedPaletteCommand;
    const registry = {
      ...PALETTE_REGISTRY,
      commands: [...PALETTE_REGISTRY.commands, shortcut],
      byId: new Map([...PALETTE_REGISTRY.byId, [shortcut.id, shortcut]]),
    };
    render(
      <CmdPaletteRegistryProvider registry={registry}>
        <OsCommandPalette open dispatch={paletteDispatch} onOpenChange={vi.fn()} />
      </CmdPaletteRegistryProvider>
    );

    await user.type(
      screen.getByPlaceholderText("Search apps, sessions, and actions…"),
      "shortcuts"
    );
    expect(screen.getByTestId("os-palette-section-commands")).toBeInTheDocument();
    expect(screen.getByTestId("os-palette-domain-settings")).toBeInTheDocument();
  });

  it("Should render and accept a casing-preserving ghost only at the input end [UT-118, UT-119]", async () => {
    const user = userEvent.setup();
    paletteMocks.rankSignals = TEST_RANK_SIGNALS;
    renderPalette();
    const input = screen.getByPlaceholderText(
      "Search apps, sessions, and actions…"
    ) as HTMLInputElement;

    await user.type(input, "Ne");
    expect(screen.getByTestId("os-palette-ghost")).toHaveTextContent("w tab");
    await user.keyboard("{ArrowRight}");
    expect(input).toHaveValue("New tab");

    await user.clear(input);
    await user.type(input, "Ne");
    input.setSelectionRange(1, 1);
    await user.keyboard("{ArrowRight}");
    expect(input).toHaveValue("Ne");
  });
});
