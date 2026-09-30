// Suite: OS dock state
// Invariant: dock activation resolves the correct instance, every available destination is reachable,
// and launchers wait for the authoritative HTTP command fence rather than the live event stream.
// Boundary IN: OsDock presentation, DesktopDock projection, and OsDockAppMenu interaction semantics.
// Boundary OUT: coordinator command execution and browser lifecycle journeys.
import { act, fireEvent, render, renderHook, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type * as React from "react";

import { TooltipProvider } from "@compozy/ui";

import type { OsDesktopRuntimeStore, OsWindow } from "../../lib/os-types";
import type { WindowManagerConfig, WindowManagerSnapshot } from "../../lib/window-manager-types";
import { DesktopDock } from "../desktop-dock";
import { OsDock } from "../os-dock";
import { OsDockAppMenu } from "../os-dock-app-menu";
import { useDesktopDock } from "../../hooks/use-desktop-dock";
import { pickLastCreatedSession } from "../../lib/last-created-session";
import type { SessionPayload } from "@/systems/session";

const dockShell = vi.hoisted(() => ({
  state: null as unknown,
  manager: { getState: vi.fn() },
  coordinator: {
    userActivateWindow: vi.fn(),
    userMinimize: vi.fn(),
    userOpen: vi.fn(),
  },
}));

const launchCatalog = vi.hoisted(() => ({
  ready: true,
  sessions: [] as SessionPayload[],
  workspaceId: "workspace:test" as string | null,
  resolveLatest: vi.fn<() => Promise<SessionPayload | null>>(),
}));

const jumpToSession = vi.hoisted(() => vi.fn());

const catalogInputs = vi.hoisted(() => ({
  workspace: { runtimeWorkspaceId: null as string | null, pending: false },
  catalog: {
    sessions: [] as SessionPayload[],
    filters: [] as Array<Record<string, unknown>>,
  },
}));

vi.mock("@/systems/workspace", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/workspace")>()),
  useActiveWorkspace: () => catalogInputs.workspace,
}));

vi.mock("@/systems/session", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/session")>()),
  sessionCatalogOptions: (filters: Record<string, unknown>) => {
    catalogInputs.catalog.filters.push(filters);
    return {
      queryKey: ["launch-catalog", JSON.stringify(filters)],
      queryFn: async () => ({ sessions: catalogInputs.catalog.sessions, page: { has_more: false } }),
      initialPageParam: null,
      getNextPageParam: () => undefined,
    };
  },
}));

vi.mock("@/systems/profiles", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/profiles")>()),
  useProfileReadScope: () => ({ params: {} }),
}));

const SNAPSHOT: WindowManagerSnapshot = {
  version: 4,
  workspaceId: "workspace:test",
  revision: 1,
  desktops: [],
  windows: {},
  closedEntryCount: 0,
  overrides: {},
  updatedAt: "2026-07-31T00:00:00Z",
};

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
  globalShortcuts: {},
  shortcutDefaults: {},
  effectiveShortcuts: {},
};

vi.mock("../../hooks/use-desktop", () => ({
  useDesktop: (selector: (state: unknown) => unknown) => selector(dockShell.state),
}));

vi.mock("../../hooks/use-os-shell", () => ({
  useOsShell: () => ({ manager: dockShell.manager, coordinator: dockShell.coordinator }),
}));

vi.mock("../../hooks/use-session-launch-catalog", () => ({
  useSessionLaunchCatalog: () => launchCatalog,
}));

vi.mock("../../hooks/use-attention-jump", () => ({
  useAttentionJump: () => jumpToSession,
}));

function windowFixture(
  id: string,
  app: OsWindow["app"],
  overrides: Partial<OsWindow> = {}
): OsWindow {
  return {
    id,
    app,
    instanceKey: null,
    route: { pathname: `/${app}`, search: {} },
    navStack: [],
    pinned: false,
    desktopId: "desktop:one",
    placement: "floating",
    rect: { x: 0, y: 0, w: 600, h: 420 },
    layer: 1,
    minimized: false,
    zoomed: false,
    groupId: null,
    nodeId: null,
    stackId: null,
    stackActive: true,
    parentAxis: null,
    ...overrides,
  };
}

function desktopState(
  windows: Record<string, OsWindow> = {},
  focusedId: string | null = null,
  focusOrder: readonly string[] = []
): OsDesktopRuntimeStore {
  return {
    snapshot: SNAPSHOT,
    windowManagerConfig: CONFIG,
    clientAttachmentToken: null,
    client: {
      workspaceId: "workspace:test",
      clientId: "client:web",
      presentationRevision: 1,
      activeDesktopId: "desktop:one",
      focusedWindowId: focusedId,
      focusOrder,
      stackActive: {},
      connectedAt: "2026-07-31T00:00:00Z",
    },
    desktops: [],
    projections: {},
    frames: {},
    windows,
    activeDesktopId: "desktop:one",
    focusedId,
    wallpaper: "ember",
    reduceMotion: false,
    presentation: "floating",
    viewportState: "ready",
    hydration: "live",
    connectionStatus: "connected",
    desktopBounds: null,
  };
}

function setDockState(state: OsDesktopRuntimeStore): void {
  dockShell.state = state;
  dockShell.manager.getState.mockReturnValue(state);
}

function renderDock(ui: React.ReactElement) {
  return render(<TooltipProvider delay={0}>{ui}</TooltipProvider>);
}

function catalogSession(
  id: string,
  createdAt: string,
  overrides: Partial<SessionPayload> = {}
): SessionPayload {
  return {
    id,
    agent_name: "qa-agent",
    workspace_id: "workspace:test",
    created_at: createdAt,
    archived_at: null,
    ...overrides,
  } as SessionPayload;
}

describe("OsDock", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    launchCatalog.ready = true;
    launchCatalog.sessions = [];
    launchCatalog.workspaceId = "workspace:test";
    launchCatalog.resolveLatest.mockImplementation(async () =>
      pickLastCreatedSession(launchCatalog.sessions)
    );
    setDockState(desktopState());
  });

  it("Should expose the real closed, running, focused, and minimized state for each launcher", () => {
    renderDock(
      <OsDock
        items={[
          { id: "dashboard", name: "Dashboard", icon: "dashboard", running: true },
          { id: "session", name: "Sessions", icon: "sessions", running: true, active: true },
          { id: "tasks", name: "Tasks", icon: "tasks", minimized: true },
          { id: "agents", name: "Agents", icon: "agents" },
        ]}
        onSelect={vi.fn()}
      />
    );

    expect(screen.getByRole("button", { name: "Dashboard" })).toHaveAttribute(
      "data-state",
      "running"
    );
    expect(screen.getByRole("button", { name: "Dashboard" })).not.toHaveAttribute("aria-current");
    expect(screen.getByRole("button", { name: "Sessions" })).toHaveAttribute(
      "aria-current",
      "true"
    );
    expect(screen.getByRole("button", { name: "Tasks" })).toHaveAttribute(
      "data-state",
      "minimized"
    );
    expect(screen.getByRole("button", { name: "Agents" })).toHaveAttribute("data-state", "closed");
  });

  it("Should wait for authoritative hydration but stay enabled while the stream reconnects", async () => {
    const user = userEvent.setup();
    setDockState({ ...desktopState(), hydration: "pending", connectionStatus: "reconnecting" });
    const view = renderDock(
      <DesktopDock
        badges={{}}
        onNewSession={vi.fn()}
        onOpenSettings={vi.fn()}
        contextMenusEnabled
      />
    );
    const tasks = screen.getByRole("button", { name: "Tasks" });

    expect(tasks).toBeDisabled();
    fireEvent.click(tasks);
    expect(dockShell.coordinator.userOpen).not.toHaveBeenCalled();

    setDockState({ ...desktopState(), connectionStatus: "reconnecting" });
    view.rerender(
      <TooltipProvider delay={0}>
        <DesktopDock
          badges={{}}
          onNewSession={vi.fn()}
          onOpenSettings={vi.fn()}
          contextMenusEnabled
        />
      </TooltipProvider>
    );

    const reconnectedTasks = screen.getByRole("button", { name: "Tasks" });
    expect(reconnectedTasks).toBeEnabled();
    await user.click(reconnectedTasks);
    expect(dockShell.coordinator.userOpen).toHaveBeenCalledWith({ app: "tasks" });
  });

  it("Should hide zero badges and cap large attention counts at 9+ (UT-066)", () => {
    renderDock(
      <OsDock
        items={[
          { id: "dashboard", name: "Dashboard", icon: "dashboard", badge: 0 },
          { id: "terminal", name: "Terminal", icon: "terminal", badge: 1 },
          { id: "sessions", name: "Sessions", icon: "sessions", badge: 3 },
          { id: "tasks", name: "Tasks", icon: "tasks", badge: 12 },
        ]}
        onSelect={vi.fn()}
      />
    );

    expect(screen.getByRole("button", { name: "Dashboard" })).not.toHaveTextContent("0");
    expect(screen.getByRole("button", { name: "Terminal — 1 needs you" })).toHaveTextContent("1");
    expect(screen.getByRole("button", { name: "Sessions — 3 need you" })).toHaveTextContent("3");
    expect(screen.getByRole("button", { name: "Tasks — 12 need you" })).toHaveTextContent("9+");
  });

  it("Should rove focus across launchers with one tab stop and show the name in a tooltip", async () => {
    const user = userEvent.setup();
    renderDock(
      <OsDock
        items={[
          { id: "dashboard", name: "Dashboard", icon: "dashboard" },
          { id: "tasks", name: "Tasks", icon: "tasks", running: true, active: true },
          { id: "knowledge", name: "Knowledge", icon: "knowledge" },
        ]}
        onSelect={vi.fn()}
      />
    );

    await user.tab();
    expect(screen.getByRole("button", { name: "Tasks" })).toHaveFocus();
    await user.keyboard("{ArrowDown}");
    expect(screen.getByRole("button", { name: "Knowledge" })).toHaveFocus();
    await waitFor(() => {
      expect(screen.getByText("Knowledge")).toBeInTheDocument();
    });
    await user.keyboard("{ArrowDown}");
    expect(screen.getByRole("button", { name: "Dashboard" })).toHaveFocus();
    await user.keyboard("{ArrowUp}");
    expect(screen.getByRole("button", { name: "Knowledge" })).toHaveFocus();
    await user.keyboard("{Home}");
    expect(screen.getByRole("button", { name: "Dashboard" })).toHaveFocus();

    await user.tab();
    expect(document.body).toHaveFocus();
    await user.tab({ shift: true });
    expect(screen.getByRole("button", { name: "Dashboard" })).toHaveFocus();
  });

  it("Should focus the MRU task instance, cycle on repeat, and restore a minimized turn (UT-043)", () => {
    const first = windowFixture("window:tasks-a", "tasks");
    const second = windowFixture("window:tasks-b", "tasks", { minimized: true });
    const other = windowFixture("window:dashboard", "dashboard");
    const { result, rerender } = renderHook(() => useDesktopDock({}, { onNewSession: vi.fn() }));

    setDockState(
      desktopState({ [first.id]: first, [second.id]: second, [other.id]: other }, other.id, [
        second.id,
        first.id,
      ])
    );
    rerender();
    act(() => result.current.handleSelect("tasks"));
    expect(dockShell.coordinator.userActivateWindow).toHaveBeenLastCalledWith(second.id);

    setDockState(
      desktopState(
        { [first.id]: first, [second.id]: { ...second, minimized: false }, [other.id]: other },
        second.id,
        [second.id, first.id]
      )
    );
    rerender();
    act(() => result.current.handleSelect("tasks"));
    expect(dockShell.coordinator.userActivateWindow).toHaveBeenLastCalledWith(first.id);
  });

  it("Should restore a minimized window that the client still reports as focused", () => {
    // The topology can mark a window minimized before the client frame drops
    // its focus; the hollow-ring launcher must still restore it, not minimize.
    const tasks = windowFixture("window:tasks", "tasks", { minimized: true, zoomed: true });
    const { result } = renderHook(() => useDesktopDock({}, { onNewSession: vi.fn() }));
    setDockState(desktopState({ [tasks.id]: tasks }, tasks.id, [tasks.id]));

    act(() => result.current.handleSelect("tasks"));

    expect(dockShell.coordinator.userMinimize).not.toHaveBeenCalled();
    expect(dockShell.coordinator.userActivateWindow).toHaveBeenCalledWith(tasks.id);
  });

  it("Should target a task instance on another desktop through the activation coordinator (UT-044)", () => {
    const remote = windowFixture("window:tasks-remote", "tasks", { desktopId: "desktop:two" });
    const { result } = renderHook(() => useDesktopDock({}, { onNewSession: vi.fn() }));
    setDockState(desktopState({ [remote.id]: remote }, null, [remote.id]));

    act(() => result.current.handleSelect("tasks"));

    expect(dockShell.coordinator.userActivateWindow).toHaveBeenCalledWith(remote.id);
  });

  it("Should keep a sessions attention badge after its last session tab closes (UT-045)", () => {
    const { result, rerender } = renderHook(() =>
      useDesktopDock({ sessions: 1 }, { onNewSession: vi.fn() })
    );
    const session = windowFixture("window:session", "session", {
      instanceKey: "session:needs-input",
    });
    setDockState(desktopState({ [session.id]: session }, session.id, [session.id]));
    rerender();

    setDockState(desktopState());
    rerender();

    expect(result.current.entries.find(entry => entry.id === "session")).toMatchObject({
      badge: 1,
    });
  });

  it("Should list every launcher in catalog order with no group separators", () => {
    const { result } = renderHook(() => useDesktopDock({}, { onNewSession: vi.fn() }));

    expect(result.current.entries.map(entry => entry.id)).toEqual([
      "session",
      "dashboard",
      "terminal",
      "agents",
      "tasks",
      "loops",
      "jobs",
      "triggers",
      "marketplace",
      "knowledge",
      "vault",
    ]);
  });

  it("Should resolve the Sessions launch catalog across every workspace in Global scope", async () => {
    const { useSessionLaunchCatalog } = await vi.importActual<
      typeof import("../../hooks/use-session-launch-catalog")
    >("../../hooks/use-session-launch-catalog");
    catalogInputs.workspace = { runtimeWorkspaceId: null, pending: false };
    catalogInputs.catalog.filters = [];
    catalogInputs.catalog.sessions = [
      catalogSession("sess-global", "2026-08-02T00:00:00Z", { workspace_id: "workspace:other" }),
    ];
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );

    const { result } = renderHook(() => useSessionLaunchCatalog(), { wrapper });

    // Global has no runtime workspace: the catalog still reads (all workspaces)
    // and resolves, so the launcher never drops the click.
    await waitFor(() => expect(result.current.ready).toBe(true));
    expect(catalogInputs.catalog.filters.at(-1)).toMatchObject({
      workspace_id: undefined,
      limit: 1,
      sort: "created",
      archive: "exclude",
    });
    expect(result.current.workspaceId).toBeNull();
    await waitFor(() =>
      expect(result.current.sessions.map(session => session.id)).toEqual(["sess-global"])
    );

    catalogInputs.workspace = { runtimeWorkspaceId: null, pending: true };
    const pending = renderHook(() => useSessionLaunchCatalog(), { wrapper });
    expect(pending.result.current.ready).toBe(false);
  });

  it("Should launch a new session from the dock when the catalog is empty", async () => {
    const onNewSession = vi.fn();
    const { result } = renderHook(() => useDesktopDock({}, { onNewSession }));
    setDockState(desktopState());

    await act(async () => result.current.handleSelect("session"));

    expect(onNewSession).toHaveBeenCalledOnce();
    expect(jumpToSession).not.toHaveBeenCalled();
    expect(dockShell.coordinator.userActivateWindow).not.toHaveBeenCalled();
  });

  it("Should hand a Global dock click with no sessions to the workspace switcher and say why", () => {
    launchCatalog.workspaceId = null;
    launchCatalog.sessions = [];
    const onNewSession = vi.fn();
    const onPickProject = vi.fn();
    const { result } = renderHook(() => useDesktopDock({}, { onNewSession, onPickProject }));
    setDockState(desktopState());

    const sessions = result.current.entries.find(entry => entry.id === "session");
    expect(sessions?.hint).toBe("Pick a project to start a session");
    act(() => result.current.handleSelect("session"));

    expect(onPickProject).toHaveBeenCalledOnce();
    expect(onNewSession).not.toHaveBeenCalled();
  });

  it("Should name the Sessions launcher with its Global hint", () => {
    renderDock(
      <OsDock
        items={[
          {
            id: "session",
            name: "Sessions",
            icon: "sessions",
            hint: "Pick a project to start a session",
          },
        ]}
        onSelect={vi.fn()}
      />
    );

    expect(
      screen.getByRole("button", { name: "Sessions. Pick a project to start a session" })
    ).toBeInTheDocument();
  });

  it("Should ignore a Sessions dock click while the catalog is still unknown", async () => {
    launchCatalog.ready = false;
    const onNewSession = vi.fn();
    const { result } = renderHook(() => useDesktopDock({}, { onNewSession }));
    setDockState(desktopState());

    await act(async () => result.current.handleSelect("session"));

    expect(onNewSession).not.toHaveBeenCalled();
    expect(jumpToSession).not.toHaveBeenCalled();
  });

  it("Should open the last created session when the catalog has rows and no window is open", async () => {
    launchCatalog.sessions = [
      catalogSession("sess-older", "2026-08-01T00:00:00Z"),
      catalogSession("sess-newer", "2026-08-02T00:00:00Z"),
    ];
    const onNewSession = vi.fn();
    const { result } = renderHook(() => useDesktopDock({}, { onNewSession }));
    setDockState(desktopState());

    await act(async () => result.current.handleSelect("session"));

    expect(onNewSession).not.toHaveBeenCalled();
    expect(jumpToSession).toHaveBeenCalledExactlyOnceWith({
      sessionId: "sess-newer",
      agentName: "qa-agent",
      workspaceId: "workspace:test",
    });
    expect(dockShell.coordinator.userActivateWindow).not.toHaveBeenCalled();
  });

  it("Should open the last created session instead of the most recently used window", async () => {
    launchCatalog.sessions = [
      catalogSession("sess-older", "2026-08-01T00:00:00Z"),
      catalogSession("sess-newer", "2026-08-02T00:00:00Z"),
    ];
    const older = windowFixture("window:session-older", "session", {
      instanceKey: "sess-older",
      stackId: "stack:session",
      stackActive: true,
    });
    const onNewSession = vi.fn();
    const { result, rerender } = renderHook(() => useDesktopDock({}, { onNewSession }));
    setDockState(desktopState({ [older.id]: older }, older.id, [older.id]));
    rerender();

    await act(async () => result.current.handleSelect("session"));

    expect(onNewSession).not.toHaveBeenCalled();
    expect(jumpToSession).toHaveBeenCalledExactlyOnceWith({
      sessionId: "sess-newer",
      agentName: "qa-agent",
      workspaceId: "workspace:test",
    });
    expect(dockShell.coordinator.userActivateWindow).not.toHaveBeenCalled();
  });

  it.each([
    {
      label: "minimized",
      overrides: { minimized: true },
      projection: { running: false, minimized: true },
    },
    {
      label: "on another desktop",
      overrides: { desktopId: "desktop:two" },
      projection: { running: true, minimized: false },
    },
    {
      label: "an inactive stack tab",
      overrides: { stackId: "stack:session", stackActive: false },
      projection: { running: true, minimized: false },
    },
  ])(
    "Should jump to the last created session when its window is $label",
    async ({ overrides, projection }) => {
      launchCatalog.sessions = [catalogSession("sess-target", "2026-08-02T00:00:00Z")];
      const target = windowFixture("window:session-target", "session", {
        instanceKey: "sess-target",
        ...overrides,
      });
      const { result, rerender } = renderHook(() => useDesktopDock({}, { onNewSession: vi.fn() }));
      setDockState(desktopState({ [target.id]: target }, null, [target.id]));
      rerender();

      await act(async () => result.current.handleSelect("session"));

      expect(jumpToSession).toHaveBeenCalledExactlyOnceWith({
        sessionId: "sess-target",
        agentName: "qa-agent",
        workspaceId: "workspace:test",
      });
      expect(dockShell.coordinator.userActivateWindow).not.toHaveBeenCalled();
      expect(result.current.entries.find(entry => entry.id === "session")).toMatchObject(
        projection
      );
    }
  );

  it("Should put the theme toggle and Settings in the rail foot, outside the launcher navigation", async () => {
    const user = userEvent.setup();
    const onOpenSettings = vi.fn();
    renderDock(
      <DesktopDock
        badges={{}}
        onNewSession={vi.fn()}
        onOpenSettings={onOpenSettings}
        contextMenusEnabled
        profileSwitcher={<button type="button">Profile</button>}
      />
    );

    const foot = document.querySelector('[data-slot="os-rail-foot"]');
    if (!(foot instanceof HTMLElement)) throw new Error("Expected the rail foot");
    const order = Array.from(foot.querySelectorAll("button")).map(
      button => button.getAttribute("aria-label") ?? button.textContent
    );
    expect(order).toEqual([
      "Profile",
      expect.stringMatching(/^Switch to (light|dark) mode$/),
      "Settings",
    ]);
    expect(foot.closest("nav")).toBeNull();

    await user.click(screen.getByRole("button", { name: "Settings" }));
    expect(onOpenSettings).toHaveBeenCalledOnce();

    const toggle = screen.getByRole("button", { name: /^Switch to (light|dark) mode$/ });
    const before = toggle.getAttribute("aria-label");
    await user.click(toggle);
    expect(
      screen.getByRole("button", { name: /^Switch to (light|dark) mode$/ })
    ).not.toHaveAttribute("aria-label", before);
  });

  it("Should keep every launcher and the foot controls in the compact tab bar, with no New session", () => {
    setDockState({ ...desktopState(), presentation: "compact" });
    renderDock(
      <DesktopDock
        badges={{ tasks: 2 }}
        onNewSession={vi.fn()}
        onOpenSettings={vi.fn()}
        contextMenusEnabled
      />
    );

    const tabBar = document.querySelector('[data-slot="os-dock-tabbar"]');
    if (!(tabBar instanceof HTMLElement)) throw new Error("Expected the compact tab bar");
    expect(tabBar.querySelectorAll('[data-slot="os-dock-item"]')).toHaveLength(11);
    expect(screen.getByRole("button", { name: "Tasks — 2 need you" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "New session" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Settings" })).toBeInTheDocument();
    expect(document.querySelector('[data-slot="os-rail"]')).toBeNull();

    // One tab stop for the strip; Left/Right rove and wrap like the rail's Up/Down.
    const launchers = Array.from(
      tabBar.querySelectorAll<HTMLButtonElement>('[data-slot="os-dock-item"]')
    );
    expect(launchers.filter(button => button.tabIndex === 0)).toHaveLength(1);
    launchers[0]?.focus();
    fireEvent.keyDown(launchers[0] as HTMLElement, { key: "ArrowLeft" });
    expect(launchers.at(-1)).toHaveFocus();
    fireEvent.keyDown(launchers.at(-1) as HTMLElement, { key: "ArrowRight" });
    expect(launchers[0]).toHaveFocus();
  });

  it("Should mark Terminal running from catalog truth rather than an open window", () => {
    const open = windowFixture("window:terminal", "terminal");
    const { result, rerender } = renderHook(
      (live: boolean) => useDesktopDock({}, { onNewSession: vi.fn(), terminalLive: live }),
      { initialProps: true }
    );
    setDockState(desktopState());
    rerender(true);

    expect(result.current.entries.find(entry => entry.id === "terminal")).toMatchObject({
      running: true,
    });

    setDockState(desktopState({ [open.id]: open }, open.id, [open.id]));
    rerender(false);

    expect(result.current.entries.find(entry => entry.id === "terminal")).toMatchObject({
      running: false,
    });
  });

  it("Should mark the sessions dock icon minimized when every session window is minimized", () => {
    const session = windowFixture("window:session-minimized", "session", {
      instanceKey: "session:minimized",
      minimized: true,
    });
    const { result, rerender } = renderHook(() => useDesktopDock({}, { onNewSession: vi.fn() }));
    setDockState(desktopState({ [session.id]: session }, null, [session.id]));
    rerender();

    expect(result.current.entries.find(entry => entry.id === "session")).toMatchObject({
      running: false,
      minimized: true,
    });
  });

  it("Should dispatch each explicit launch destination from an open app menu (UT-083)", async () => {
    const user = userEvent.setup();
    const task = windowFixture("window:tasks", "tasks");
    const focused = windowFixture("window:dashboard", "dashboard");
    setDockState(
      desktopState({ [task.id]: task, [focused.id]: focused }, focused.id, [task.id, focused.id])
    );
    render(
      <OsDockAppMenu appId="tasks">
        <button type="button">Tasks</button>
      </OsDockAppMenu>
    );

    fireEvent.contextMenu(screen.getByRole("button", { name: "Tasks" }));
    await user.click(await screen.findByText("Open in new window"));
    expect(dockShell.coordinator.userOpen).toHaveBeenLastCalledWith({
      app: "tasks",
      forceNewInstance: true,
    });

    fireEvent.contextMenu(screen.getByRole("button", { name: "Tasks" }));
    await user.click(await screen.findByText("Open as tab in focused window"));
    expect(dockShell.coordinator.userOpen).toHaveBeenLastCalledWith({
      app: "tasks",
      forceNewInstance: true,
      stackTargetWindowId: focused.id,
    });
  });

  it("Should expose Go to tab and explain why a tab destination is unavailable (UT-084)", async () => {
    const user = userEvent.setup();
    const task = windowFixture("window:tasks", "tasks");
    setDockState(desktopState({ [task.id]: task }, null, [task.id]));
    render(
      <OsDockAppMenu appId="tasks">
        <button type="button">Tasks</button>
      </OsDockAppMenu>
    );

    fireEvent.contextMenu(screen.getByRole("button", { name: "Tasks" }));
    const tabDestination = await screen.findByText("Open as tab (no window focused)");
    expect(tabDestination).toHaveAttribute("data-disabled");
    expect(await screen.findByText("Go to tab")).toBeInTheDocument();

    await user.click(screen.getByText("Go to tab"));
    expect(dockShell.coordinator.userActivateWindow).toHaveBeenCalledWith(task.id);
  });

  it("Should close an open destination menu and keep it unavailable while an overlay is active (UT-085)", async () => {
    const task = windowFixture("window:tasks", "tasks");
    setDockState(desktopState({ [task.id]: task }, task.id, [task.id]));
    const view = renderDock(
      <DesktopDock
        badges={{}}
        onNewSession={vi.fn()}
        onOpenSettings={vi.fn()}
        contextMenusEnabled
      />
    );
    const taskButton = screen.getByRole("button", { name: "Tasks" });
    fireEvent.contextMenu(taskButton);
    await screen.findByText("Open in new window");

    view.rerender(
      <TooltipProvider delay={0}>
        <DesktopDock
          badges={{}}
          onNewSession={vi.fn()}
          onOpenSettings={vi.fn()}
          contextMenusEnabled={false}
        />
      </TooltipProvider>
    );

    await waitFor(() => expect(screen.queryByText("Open in new window")).not.toBeInTheDocument());
    fireEvent.contextMenu(screen.getByRole("button", { name: "Tasks" }));
    expect(screen.queryByText("Open in new window")).not.toBeInTheDocument();
  });

  it("Should open the destination menu with the keyboard and select its focused item (UT-086)", async () => {
    const user = userEvent.setup();
    const task = windowFixture("window:tasks", "tasks");
    const focused = windowFixture("window:dashboard", "dashboard");
    setDockState(
      desktopState({ [task.id]: task, [focused.id]: focused }, focused.id, [task.id, focused.id])
    );
    render(
      <OsDockAppMenu appId="tasks">
        <button type="button">Tasks</button>
      </OsDockAppMenu>
    );

    const trigger = screen.getByRole("button", { name: "Tasks" });
    vi.spyOn(trigger, "getBoundingClientRect").mockReturnValue({
      left: 100,
      top: 40,
      right: 148,
      bottom: 88,
      width: 48,
      height: 48,
      x: 100,
      y: 40,
      toJSON: () => ({}),
    });
    const menuTrigger = trigger.closest("[data-slot=context-menu-trigger]");
    if (menuTrigger === null) throw new Error("Expected context-menu trigger wrapper");
    const contextMenuEvents = vi.fn();
    menuTrigger.addEventListener("contextmenu", contextMenuEvents);
    trigger.focus();
    await user.keyboard("{Shift>}{F10}{/Shift}");
    await screen.findByText("Open in new window");
    const keyboardMenuEvent = contextMenuEvents.mock.calls.at(-1)?.[0] as MouseEvent | undefined;
    expect(keyboardMenuEvent?.clientX).toBe(124);
    expect(keyboardMenuEvent?.clientY).toBe(64);
    await user.keyboard("{ArrowDown}{Enter}");

    expect(dockShell.coordinator.userOpen).toHaveBeenCalledWith({
      app: "tasks",
      forceNewInstance: true,
    });
  });
});
