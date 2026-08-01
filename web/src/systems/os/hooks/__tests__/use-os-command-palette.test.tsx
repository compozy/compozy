// Suite: OS command palette
// Invariant: palette commands preserve the selected tab's identity and route by
// delegating each lifecycle decision to the routing coordinator exactly once.
// Owning layer: command-palette hook and presentation boundary.
// Boundary OUT: keyboard action dispatch (window-manager-action-dispatch) and
// daemon command transport.
import { act, render, renderHook, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { LayoutDesktop } from "../../lib/window-manager-types";
import type { OsDesktopRuntimeStore, OsWindow } from "../../lib/os-types";
import { OsCommandPalette } from "../../components/os-command-palette";
import { useOsCommandPalette } from "../use-os-command-palette";

const paletteMocks = vi.hoisted(() => ({
  activeWorkspaceId: "workspace:alpha" as string | null,
  closeWindow: vi.fn(async () => true),
  coordinator: {
    userActivateWindow: vi.fn(async () => true),
    userOpen: vi.fn(async () => "window:new-tab" as string | null),
  },
  desktop: null as OsDesktopRuntimeStore | null,
  isWaiting: vi.fn<(sessionId: string) => boolean>(() => false),
  openForAgent: vi.fn(),
  paletteIntent: null as { kind: "destination"; windowId: string } | null,
  paletteIntentCleared: vi.fn(),
  paletteIntentRequested: vi.fn(),
  sessions: [] as Array<{ id: string; name: string | null; agent_name: string; badge: string }>,
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
  workspaces: [{ id: "workspace:alpha", name: "Alpha" }],
}));

vi.mock("@/systems/session", () => ({
  getSessionDisplayTitle: (session: { id: string; name: string | null }) =>
    session.name ?? session.id,
  useSessionCreateActions: () => ({ openForAgent: paletteMocks.openForAgent }),
  useSessions: () => ({ data: paletteMocks.sessions }),
}));

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({
    activeWorkspaceId: paletteMocks.activeWorkspaceId,
    setActiveWorkspaceId: vi.fn(),
    workspaces: paletteMocks.workspaces,
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

vi.mock("../../hooks/use-window-manager-store", () => ({
  useWindowPaletteIntent: () => paletteMocks.paletteIntent,
}));

vi.mock("../../lib/attention-model", () => ({
  isWaitingSession: (session: { id: string }) => paletteMocks.isWaiting(session.id),
}));

vi.mock("../../lib/window-slot-registry", () => ({
  pruneWindowSlotStores: vi.fn(),
  subscribeWindowSlotRegistry: () => () => undefined,
  windowSlotRegistryVersion: () => 0,
  windowSlotSnapshot: (windowId: string) => paletteMocks.windowSlots.get(windowId) ?? null,
}));

vi.mock("../../stores/window-manager-store", () => ({
  windowManagerStore: {
    trigger: {
      paletteIntentCleared: paletteMocks.paletteIntentCleared,
      paletteIntentRequested: paletteMocks.paletteIntentRequested,
    },
  },
}));

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
    paletteMocks.sessions = [];
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
      { agent_name: "codex", badge: "running", id: "session:waiting", name: "Needs input" },
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
