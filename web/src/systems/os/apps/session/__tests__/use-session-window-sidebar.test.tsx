// Suite: session-window sidebar selection
// Invariant: same-workspace selection retargets the current window; a foreign
// session goes through the workspace-ready jump coordinator.
// Owning layer: session-window sidebar hook. No prior suite owned this branch.
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { SessionPayload } from "@/systems/session";

const { jumpToSession, userRetarget } = vi.hoisted(() => ({
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
vi.mock("../../../hooks/use-os-shell", () => ({
  useOsShell: () => ({ coordinator: { userRetarget } }),
}));
vi.mock("@/systems/session", () => ({
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
  beforeEach(() => vi.clearAllMocks());

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
