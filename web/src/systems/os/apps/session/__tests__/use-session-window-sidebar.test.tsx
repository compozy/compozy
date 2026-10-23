// Suite: session-window sidebar selection
// Invariant: the rail follows the topbar breadth — the aggregate under All projects, the
// focused project or User otherwise (never a worktree filter outside a project) — and an
// unsettled destination stays idle. A selection in the window's own scope retargets the window;
// under All projects every selection retargets in place, rebinding the window to the session's
// scope; otherwise a foreign session goes through the jump coordinator.
// Owning layer: session-window sidebar hook. No prior suite owned this branch.
import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { SessionPayload } from "@/systems/session";

const {
  jumpToSession,
  userRetarget,
  transport,
  useSessionListView,
  useScopedWorktreeFilter,
  scopes,
} = vi.hoisted(() => ({
  scopes: {
    destination: { kind: "project", workspaceId: "workspace-1" } as unknown,
    window: { kind: "project", workspaceId: "workspace-1" } as unknown,
  },
  transport: { degradedAt: null as number | null },
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
vi.mock("../../../hooks/use-os-reduced-motion", () => ({ useOsReducedMotion: () => true }));
vi.mock("../../../hooks/use-os-shell", () => ({
  useOsShell: () => ({ coordinator: { userRetarget } }),
}));
vi.mock("@/systems/session", async () => ({
  useSessionTransportState: () => transport,
  useTransportGraceElapsed: (await import("@/systems/session/hooks/use-transport-grace-elapsed"))
    .useTransportGraceElapsed,
  sessionListSortParam: (sort: string) => sort,
  useSessionCreateActions: () => ({ openForAgent: vi.fn() }),
  useSessionLifecycleActions: () => ({
    actions: {},
    deleteDialog: { open: false },
    renameDialog: { open: false },
  }),
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

import { useSessionWindowSidebar } from "../use-session-window-sidebar";

function session(workspaceId: string): SessionPayload {
  return {
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

    expect(useSessionListView).toHaveBeenLastCalledWith(
      expect.objectContaining({ scope: null, enabled: false })
    );
  });
});
