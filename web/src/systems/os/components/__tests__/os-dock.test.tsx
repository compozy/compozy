// Suite: OS dock state
// Invariant: dock activation resolves the correct instance and every available destination is reachable.
// Boundary IN: OsDock presentation, DesktopDock projection, and OsDockAppMenu interaction semantics.
// Boundary OUT: coordinator command execution and browser lifecycle journeys.
import { act, fireEvent, render, renderHook, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Bot, LayoutDashboard, ListChecks } from "lucide-react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { TooltipProvider } from "@compozy/ui";

import type { OsDesktopRuntimeStore, OsWindow } from "../../lib/os-types";
import { DesktopDock } from "../desktop-dock";
import { OsDock } from "../os-dock";
import { OsDockAppMenu } from "../os-dock-app-menu";
import { useDesktopDock } from "../../hooks/use-desktop-dock";

const dockShell = vi.hoisted(() => ({
  state: null as unknown,
  manager: { getState: vi.fn() },
  coordinator: {
    userActivateWindow: vi.fn(),
    userMinimize: vi.fn(),
    userOpen: vi.fn(),
  },
}));

vi.mock("../../hooks/use-desktop", () => ({
  useDesktop: (selector: (state: unknown) => unknown) => selector(dockShell.state),
}));

vi.mock("../../hooks/use-os-shell", () => ({
  useOsShell: () => ({ manager: dockShell.manager, coordinator: dockShell.coordinator }),
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
    snapshot: null,
    windowManagerConfig: null,
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
    railCollapsedAgentIds: [],
    wallpaper: "ember",
    reduceMotion: false,
    dockMagnify: true,
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

describe("OsDock", () => {
  beforeEach(() => {
    vi.stubGlobal("requestAnimationFrame", (cb: FrameRequestCallback) => {
      cb(0);
      return 1;
    });
    vi.stubGlobal("cancelAnimationFrame", () => undefined);
    vi.clearAllMocks();
    setDockState(desktopState());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("Should expose the real closed, running, and minimized state for each launcher", () => {
    renderDock(
      <OsDock
        items={[
          { id: "dashboard", name: "Dashboard", icon: LayoutDashboard, running: true },
          { id: "tasks", name: "Tasks", icon: ListChecks, minimized: true },
          { id: "agents", name: "Agents", icon: Bot },
        ]}
        onSelect={vi.fn()}
      />
    );

    expect(screen.getByRole("button", { name: "Dashboard" })).toHaveAttribute(
      "data-state",
      "running"
    );
    expect(screen.getByRole("button", { name: "Tasks" })).toHaveAttribute(
      "data-state",
      "minimized"
    );
    expect(screen.getByRole("button", { name: "Agents" })).toHaveAttribute("data-state", "closed");
  });

  it("Should hide zero badges and cap large attention counts at 9+ (UT-066)", () => {
    renderDock(
      <OsDock
        items={[
          { id: "dashboard", name: "Dashboard", icon: LayoutDashboard, badge: 0 },
          { id: "tasks", name: "Tasks", icon: ListChecks, badge: 12 },
        ]}
        onSelect={vi.fn()}
      />
    );

    expect(screen.getByRole("button", { name: "Dashboard" })).not.toHaveTextContent("0");
    expect(screen.getByRole("button", { name: "Tasks" })).toHaveTextContent("9+");
  });

  it("Should show the launcher name in a tooltip on focus", async () => {
    const user = userEvent.setup();
    renderDock(
      <OsDock
        magnify={false}
        items={[
          { id: "dashboard", name: "Dashboard", icon: LayoutDashboard },
          { id: "network", name: "Network", icon: Bot },
        ]}
        onSelect={vi.fn()}
      />
    );

    await user.tab();
    await user.tab();
    expect(screen.getByRole("button", { name: "Network" })).toHaveFocus();
    await waitFor(() => {
      expect(screen.getByText("Network")).toBeInTheDocument();
    });
  });

  it("Should magnify the nearest launcher on pointer proximity", () => {
    renderDock(
      <OsDock
        items={[
          { id: "dashboard", name: "Dashboard", icon: LayoutDashboard },
          { id: "tasks", name: "Tasks", icon: ListChecks },
        ]}
        onSelect={vi.fn()}
      />
    );

    const dock = screen.getByRole("navigation", { name: "Dock" });
    const dashboard = screen.getByRole("button", { name: "Dashboard" });
    vi.spyOn(dashboard, "getBoundingClientRect").mockReturnValue({
      left: 100,
      width: 46,
      top: 0,
      height: 46,
      right: 146,
      bottom: 46,
      x: 100,
      y: 0,
      toJSON: () => ({}),
    });

    fireEvent.pointerMove(dock, { clientX: 123 });

    expect(dashboard.style.transform).toContain("scale(");
    expect(dashboard.style.transform).toContain("translateY(");
  });

  it("Should keep launchers static when magnification is disabled", () => {
    renderDock(
      <OsDock
        magnify={false}
        items={[
          { id: "dashboard", name: "Dashboard", icon: LayoutDashboard },
          { id: "tasks", name: "Tasks", icon: ListChecks },
        ]}
        onSelect={vi.fn()}
      />
    );

    const dock = screen.getByRole("navigation", { name: "Dock" });
    const dashboard = screen.getByRole("button", { name: "Dashboard" });
    vi.spyOn(dashboard, "getBoundingClientRect").mockReturnValue({
      left: 100,
      width: 46,
      top: 0,
      height: 46,
      right: 146,
      bottom: 46,
      x: 100,
      y: 0,
      toJSON: () => ({}),
    });

    fireEvent.pointerMove(dock, { clientX: 123 });

    expect(dashboard.style.transform).toBe("");
  });

  it("Should focus the MRU task instance, cycle on repeat, and restore a minimized turn (UT-043)", () => {
    const first = windowFixture("window:tasks-a", "tasks");
    const second = windowFixture("window:tasks-b", "tasks", { minimized: true });
    const other = windowFixture("window:dashboard", "dashboard");
    const { result, rerender } = renderHook(() =>
      useDesktopDock({}, { sessionsOpen: false, onToggleSessions: vi.fn() })
    );

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

  it("Should target a task instance on another desktop through the activation coordinator (UT-044)", () => {
    const remote = windowFixture("window:tasks-remote", "tasks", { desktopId: "desktop:two" });
    const { result } = renderHook(() =>
      useDesktopDock({}, { sessionsOpen: false, onToggleSessions: vi.fn() })
    );
    setDockState(desktopState({ [remote.id]: remote }, null, [remote.id]));

    act(() => result.current.handleSelect("tasks"));

    expect(dockShell.coordinator.userActivateWindow).toHaveBeenCalledWith(remote.id);
  });

  it("Should keep a sessions attention badge after its last session tab closes (UT-045)", () => {
    const { result, rerender } = renderHook(() =>
      useDesktopDock({ sessions: 1 }, { sessionsOpen: false, onToggleSessions: vi.fn() })
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
        onToggleSessions={vi.fn()}
        pager={null}
        contextMenusEnabled
        sessionsOpen={false}
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
          onToggleSessions={vi.fn()}
          pager={null}
          contextMenusEnabled={false}
          sessionsOpen
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
