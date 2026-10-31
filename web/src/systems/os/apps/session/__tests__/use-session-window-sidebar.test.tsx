// Suite: session-window sidebar selection
// Invariant: the rail follows the topbar breadth — the aggregate under All projects, the
// focused project or User otherwise (never a worktree filter outside a project) — and an
// unsettled destination stays idle. A selection in the window's own scope retargets the window;
// under All projects every selection retargets in place, rebinding the window to the session's
// scope; otherwise a foreign session goes through the jump coordinator. An open rail that is still
// waiting on its destination or worktree filter reads as loading, never as an empty list, and an
// in-place retarget animates through React (the window's keyed boundary), not by calling the
// document's view transition itself. Over the board (#1052) the rail is never read, and an
// in-place open keeps the window's drawer open.
// Owning layer: session-window sidebar hook. No prior suite owned this branch.
import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { subagentRevealStore } from "@/systems/session/lib/subagent-navigation";
import type { SessionPayload } from "@/systems/session";

const {
  jumpToSession,
  getState,
  setSessionWindowView,
  userRetarget,
  transport,
  useSessionListView,
  useScopedWorktreeFilter,
  scopes,
  presentation,
} = vi.hoisted(() => ({
  presentation: { value: "window" as "window" | "board-drawer" },
  scopes: {
    destination: { kind: "project", workspaceId: "workspace-1" } as unknown,
    window: { kind: "project", workspaceId: "workspace-1" } as unknown,
  },
  transport: { degradedAt: null as number | null },
  getState: vi.fn<
    () => {
      windows: Record<
        string,
        {
          id: string;
          app: string;
          instanceKey: string;
          route?: { pathname: string; search: Record<string, unknown> };
        }
      >;
      client: { focusOrder: string[] };
    }
  >(() => ({ windows: {}, client: { focusOrder: [] } })),
  setSessionWindowView: vi.fn(),
  jumpToSession: vi.fn(),
  userRetarget: vi.fn(),
  useSessionListView: vi.fn(() => ({ sort: "last_activity", scope: "workspace" })),
  useScopedWorktreeFilter: vi.fn((_workspaceId: string | null, _scopeId: string) => ({
    resolved: true,
    worktreeId: undefined,
  })),
}));

vi.mock("@/hooks/use-window-scope", () => ({ useWorktreeScopeId: () => "window:one" }));
vi.mock("@/systems/workspace", async () => ({
  ...(await import("@/systems/workspace/lib/destination")),
  ...(await import("@/systems/workspace/lib/window-data-scope")),
  useScopedWorktreeFilter,
  useDataScope: () => scopes.destination,
  useWindowDataWorkspace: () => scopes.window,
}));
vi.mock("../../../hooks/use-attention-jump", () => ({
  useAttentionJump: () => jumpToSession,
}));
vi.mock("../../../hooks/use-os-shell", () => ({
  useOsShell: () => ({ coordinator: { userRetarget }, manager: { getState } }),
}));
vi.mock("@/systems/session", async () => ({
  ...(await import("@/systems/session/lib/session-deeplink-search")),
  useSessionWindowPresentation: () => presentation.value,
  sessionHasUnsentInput: () => false,
  subagentRevealStore: (await import("@/systems/session/lib/subagent-navigation"))
    .subagentRevealStore,
  useSessionTransportState: () => transport,
  useTransportGraceElapsed: (await import("@/systems/session/hooks/use-transport-grace-elapsed"))
    .useTransportGraceElapsed,
  sessionListSortParam: (sort: string) => sort,
  useSessionCreateActions: () => ({ openForAgent: vi.fn() }),
  useSessionLifecycleActions: () => ({
    actions: {},
    deleteDialog: { open: false },
    renameDialog: { open: false },
    laterDialog: { open: false },
  }),
  setSessionWindowView,
  useSessionsBoard: () => ({}),
  useSessionWindowViewState: () => ({ hideChildren: false, setView: vi.fn() }),
  useSessionWindowOrder: () => ({ sort: "created", groupBy: "none" }),
  useSessionListView,
  useSessions: () => ({ data: [], isError: false }),
  useSessionSidebarState: () => ({
    open: true,
    toggle: vi.fn(),
    collapsedThreadIds: [],
    subagentThreadIds: [],
    setThreadBranch: vi.fn(),
  }),
}));

import { useSessionBoardWindow } from "../hooks/use-session-board-window";
import { useSessionWindowSidebar } from "../use-session-window-sidebar";

function session(workspaceId: string): SessionPayload {
  return {
    board_column: "done",
    pull_request: null,
    pull_request_lookup: null,
    supervision: null,
    profile_id: "00000000000000000000000000",
    profile_name: "default",
    scope: "workspace",
    effective_workspace_access: "",
    id: `session-${workspaceId}`,
    agent_name: "claude",
    runtime: { status: "ready", transition: "initial_bind", selection_revision: 0 },
    workspace_id: workspaceId,
    workspace_path: `/workspace/${workspaceId}`,
    state: "active",
    badge: "idle",
    attachable: true,
    available_commands: [],
    created_at: "2026-08-16T10:00:00Z",
    updated_at: "2026-08-16T10:00:00Z",
    archived_at: null,
    pending_interactions: [],
  };
}

describe("useSessionWindowSidebar", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    transport.degradedAt = null;
    scopes.destination = { kind: "project", workspaceId: "workspace-1" };
    scopes.window = { kind: "project", workspaceId: "workspace-1" };
    presentation.value = "window";
    getState.mockReturnValue({ windows: {}, client: { focusOrder: [] } });
  });
  afterEach(() => vi.useRealTimers());

  // Invariant: tab reactivation and brief stream reconnects never warn; sustained loss does.
  // Owner: sidebar transport feedback; canonical suite: this existing sidebar hook suite.
  it("Should wait for transport grace and clear the warning immediately on recovery", () => {
    vi.useFakeTimers();
    vi.setSystemTime(100_000);
    let disconnected = false;
    const { result, rerender, unmount } = renderHook(() =>
      useSessionWindowSidebar({
        windowId: "window-session",
        transportDisconnected: disconnected,
      })
    );
    transport.degradedAt = Date.now();
    disconnected = true;
    rerender();
    expect(result.current.disconnected).toBe(false);
    act(() => vi.advanceTimersByTime(1_999));
    expect(result.current.disconnected).toBe(false);
    transport.degradedAt = null;
    disconnected = false;
    rerender();
    act(() => vi.advanceTimersByTime(2_000));
    expect(result.current.disconnected).toBe(false);
    transport.degradedAt = Date.now();
    disconnected = true;
    rerender();
    act(() => vi.advanceTimersByTime(2_000));
    expect(result.current.disconnected).toBe(true);
    transport.degradedAt = null;
    disconnected = false;
    rerender();
    expect(result.current.disconnected).toBe(false);
    unmount();
  });

  it("Should use the owning navigation path for local and foreign sessions", () => {
    const { result } = renderHook(() =>
      useSessionWindowSidebar({
        windowId: "window-session",
        sessionId: "session-current",
      })
    );

    act(() => result.current.onSelectSession(session("workspace-1")));
    expect(userRetarget).toHaveBeenCalledTimes(1);
    expect(jumpToSession).not.toHaveBeenCalled();

    act(() => result.current.onSelectSession(session("workspace-2")));
    expect(jumpToSession).toHaveBeenCalledExactlyOnceWith({
      sessionId: "session-workspace-2",
      agentName: "claude",
      workspaceId: "workspace-2",
    });
    expect(userRetarget).toHaveBeenCalledTimes(1);
  });

  // Invariant: selecting native work in the current session still issues a reveal.
  // Owner: sidebar navigation; canonical suite: this file.
  it("Should reveal native work without retargeting the already open parent", () => {
    const { result } = renderHook(() =>
      useSessionWindowSidebar({
        windowId: "window-session",
        sessionId: "session-current",
      })
    );
    act(() =>
      result.current.onSelectSession({
        id: "session-current",
        agent_name: "claude",
        workspace_id: "workspace-1",
        subagentId: "native-review",
      })
    );
    const request = subagentRevealStore.getSnapshot().context.request;
    expect(request).toMatchObject({
      sessionId: "session-current",
      workspaceId: "workspace-1",
      subagentId: "native-review",
    });
    expect(userRetarget).not.toHaveBeenCalled();
    expect(jumpToSession).not.toHaveBeenCalled();
    subagentRevealStore.trigger.settled({ key: request!.key });
  });

  it("Should list User sessions under User focus without a worktree filter", () => {
    scopes.destination = { kind: "user" };
    scopes.window = { kind: "user" };
    renderHook(() =>
      useSessionWindowSidebar({ windowId: "window-session", sessionId: "session-user" })
    );

    expect(useScopedWorktreeFilter.mock.calls.at(-1)?.[0]).toBeNull();
    expect(useSessionListView).toHaveBeenLastCalledWith(
      expect.objectContaining({ scope: { kind: "user" }, enabled: true })
    );
  });

  it("Should list every project under All projects and retarget in place with the new binding", () => {
    scopes.destination = { kind: "all" };
    scopes.window = { kind: "project", workspaceId: "workspace-1" };
    const { result } = renderHook(() =>
      useSessionWindowSidebar({ windowId: "window-session", sessionId: "session-current" })
    );

    expect(useSessionListView).toHaveBeenLastCalledWith(
      expect.objectContaining({ scope: { kind: "all" }, enabled: true })
    );
    act(() => result.current.onSelectSession(session("workspace-2")));
    expect(jumpToSession).not.toHaveBeenCalled();
    expect(userRetarget).toHaveBeenCalledExactlyOnceWith("window-session", {
      app: "session",
      instanceKey: "session-workspace-2",
      route: {
        pathname: "/agents/claude/sessions/session-workspace-2",
        search: { workspace: "workspace-2" },
      },
    });
  });

  it("Should keep the rail idle while the destination is unsettled", () => {
    scopes.destination = null;
    renderHook(() => useSessionWindowSidebar({ windowId: "window-session" }));

    // Idle reads nothing, yet an open rail is still on its way to rows: it reads as loading.
    expect(useSessionListView).toHaveBeenLastCalledWith(
      expect.objectContaining({ scope: null, enabled: false, pending: true })
    );
  });

  it("Should read an open rail as loading while its worktree filter resolves, then as settled", () => {
    useScopedWorktreeFilter.mockReturnValue({ resolved: false, worktreeId: undefined });
    try {
      const { rerender } = renderHook(() =>
        useSessionWindowSidebar({ windowId: "window-session", sessionId: "session-current" })
      );
      expect(useSessionListView).toHaveBeenLastCalledWith(
        expect.objectContaining({ enabled: false, pending: true })
      );

      useScopedWorktreeFilter.mockReturnValue({ resolved: true, worktreeId: undefined });
      rerender();
      expect(useSessionListView).toHaveBeenLastCalledWith(
        expect.objectContaining({ enabled: true, pending: false })
      );
    } finally {
      useScopedWorktreeFilter.mockReturnValue({ resolved: true, worktreeId: undefined });
    }
  });

  it("Should retarget in place without calling the document's view transition itself", () => {
    const startViewTransition = vi.fn();
    Object.defineProperty(document, "startViewTransition", {
      configurable: true,
      value: startViewTransition,
    });
    try {
      const { result } = renderHook(() =>
        useSessionWindowSidebar({ windowId: "window-session", sessionId: "session-current" })
      );
      act(() => result.current.onSelectSession(session("workspace-1")));

      expect(userRetarget).toHaveBeenCalledTimes(1);
      expect(startViewTransition).not.toHaveBeenCalled();
    } finally {
      Reflect.deleteProperty(document, "startViewTransition");
    }
  });

  it("Should never read the rail over the board", () => {
    presentation.value = "board-drawer";
    renderHook(() =>
      useSessionWindowSidebar({ windowId: "window-session", sessionId: "session-current" })
    );
    expect(useSessionListView).toHaveBeenLastCalledWith(
      expect.objectContaining({ enabled: false, pending: false })
    );
  });

  it("Should keep the board drawer open when a session opens in place from inside it", () => {
    getState.mockReturnValue({
      windows: {
        "window-session": {
          id: "window-session",
          app: "session",
          instanceKey: "session-current",
          route: {
            pathname: "/agents/claude/sessions/session-current",
            search: { drawer: "open" },
          },
        },
      },
      client: { focusOrder: ["window-session"] },
    });
    const { result } = renderHook(() =>
      useSessionWindowSidebar({ windowId: "window-session", sessionId: "session-current" })
    );
    act(() =>
      result.current.openSession(
        { sessionId: "session-child", agentName: "claude", workspaceId: "workspace-1" },
        { newWindow: false }
      )
    );
    expect(userRetarget).toHaveBeenCalledExactlyOnceWith("window-session", {
      app: "session",
      instanceKey: "session-child",
      route: { pathname: "/agents/claude/sessions/session-child", search: { drawer: "open" } },
    });
  });
});

// Invariant: a board card opens its session in this window's drawer (#1052); a session another
// window already shows is focused there, in List, never duplicated into the drawer.
// Owning layer: session-window navigation; this suite owns selection routing.
describe("useSessionBoardWindow", () => {
  beforeEach(() => vi.clearAllMocks());

  it("Should switch the existing destination window to List before focusing it", () => {
    const target = session("workspace-1");
    getState.mockReturnValue({
      windows: { destination: { id: "destination", app: "session", instanceKey: target.id } },
      client: { focusOrder: ["destination"] },
    });
    const { result } = renderHook(() => useSessionBoardWindow("source", null, false));
    act(() => result.current.onOpen(target));
    expect(setSessionWindowView).toHaveBeenCalledWith("destination", "list");
    expect(userRetarget).toHaveBeenCalledWith(
      "source",
      expect.objectContaining({
        instanceKey: target.id,
        route: expect.objectContaining({ search: {} }),
      })
    );
  });

  it("Should open a card in this window's drawer and reopen the drawer on its own session", () => {
    const target = session("workspace-1");
    getState.mockReturnValue({
      windows: {
        source: {
          id: "source",
          app: "session",
          instanceKey: target.id,
          route: { pathname: `/agents/claude/sessions/${target.id}`, search: {} },
        },
      },
      client: { focusOrder: ["source"] },
    });
    const { result } = renderHook(() => useSessionBoardWindow("source", target.id, false));
    act(() => result.current.onOpen(target));
    expect(setSessionWindowView).not.toHaveBeenCalled();
    expect(userRetarget).toHaveBeenCalledExactlyOnceWith("source", {
      app: "session",
      instanceKey: target.id,
      route: { pathname: `/agents/claude/sessions/${target.id}`, search: { drawer: "open" } },
    });
  });

  it("Should close the drawer when the window leaves the board for List", () => {
    const target = session("workspace-1");
    getState.mockReturnValue({
      windows: {
        source: {
          id: "source",
          app: "session",
          instanceKey: target.id,
          route: {
            pathname: `/agents/claude/sessions/${target.id}`,
            search: { drawer: "open", workspace: "workspace-1" },
          },
        },
      },
      client: { focusOrder: ["source"] },
    });
    const { result } = renderHook(() => useSessionBoardWindow("source", target.id, true));
    act(() => result.current.setView("list"));
    expect(userRetarget).toHaveBeenCalledExactlyOnceWith("source", {
      app: "session",
      instanceKey: target.id,
      route: {
        pathname: `/agents/claude/sessions/${target.id}`,
        search: { workspace: "workspace-1" },
      },
    });
  });
});
