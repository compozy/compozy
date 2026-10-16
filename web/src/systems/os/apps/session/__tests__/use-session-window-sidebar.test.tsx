// Suite: session-window sidebar selection
// Invariant: same-workspace selection retargets the current window; a foreign
// session goes through the workspace-ready jump coordinator.
// Owning layer: session-window sidebar hook. No prior suite owned this branch.
import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { SessionPayload } from "@/systems/session";

const { jumpToSession, userRetarget, transport } = vi.hoisted(() => ({
  transport: { degradedAt: null as number | null },
  jumpToSession: vi.fn(),
  userRetarget: vi.fn(),
}));

vi.mock("@/hooks/use-window-scope", () => ({ useWorktreeScopeId: () => "window:one" }));
vi.mock("@/systems/workspace", () => ({
  useScopedWorktreeFilter: () => ({ resolved: true, worktreeId: undefined }),
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
  useSessionListView: () => ({ sort: "last_activity", scope: "workspace" }),
  useSessions: () => ({ data: [], isError: false }),
  useSessionSidebarState: () => ({
    open: true,
    toggle: vi.fn(),
    collapsedThreadIds: [],
    toggleThread: vi.fn(),
  }),
}));

import { useSessionWindowSidebar } from "../use-session-window-sidebar";

function session(workspaceId: string): SessionPayload {
  return {
    supervision: null,
    profile_id: "00000000000000000000000000",
    profile_name: "default",
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
        workspaceId: "workspace-1",
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
        workspaceId: "workspace-1",
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
});
