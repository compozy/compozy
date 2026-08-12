// Suite: OS attention data readiness
// Invariant: either session catalog query failing leaves attention disconnected instead of implying no sessions,
// and attention authority stays workspace-wide while the sessions modal follows the focused window's worktree scope.
// Owning layer: OS attention query adapter. Canonical suite: this hook test.
import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/systems/session/hooks/use-sessions", () => ({ useSessions: vi.fn() }));
vi.mock("@/systems/tasks/lib/workspace-scope", () => ({
  taskScopeForActiveWorkspace: vi.fn(),
}));
vi.mock("@/systems/tasks/hooks/use-task-dashboard", () => ({
  useTaskDashboard: vi.fn(),
}));
vi.mock("@/systems/tasks/hooks/use-tasks", () => ({
  useTasks: vi.fn(),
}));
vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: vi.fn(),
}));
vi.mock("@/systems/workspace/hooks/use-active-worktree", () => ({
  useScopedWorktreeFilter: vi.fn(),
}));
vi.mock("../use-worktree-scope", () => ({ useFocusedWorktreeScopeId: vi.fn(() => "window:one") }));

import { useOsAttention } from "../use-os-attention";
import { useSessions, type SessionPayload } from "@/systems/session";
import { taskScopeForActiveWorkspace, useTaskDashboard, useTasks } from "@/systems/tasks";
import {
  useActiveWorkspace,
  useScopedWorktreeFilter,
  type WorkspacePayload,
} from "@/systems/workspace";

const workspace: WorkspacePayload = {
  id: "ws_alpha",
  name: "alpha",
  root_dir: "/workspace/alpha",
  add_dirs: [],
  created_at: "2026-08-04T12:00:00Z",
  updated_at: "2026-08-04T12:00:00Z",
};

function sessionsQuery({
  data = [],
  isError = false,
  isLoading = false,
}: {
  data?: ReturnType<typeof useOsAttention>["sessions"] | undefined;
  isError?: boolean;
  isLoading?: boolean;
}) {
  return { data, isError, isLoading, total: data?.length ?? 0 } as ReturnType<typeof useSessions>;
}

/** `badge` is what marks a session as needing attention (see isWaitingSession). */
function waitingSession(id: string): SessionPayload {
  return {
    id,
    agent_name: "atlas",
    badge: "waiting-for-auth",
    workspace_id: workspace.id,
  } as SessionPayload;
}

/** Call order in the hook: attention (unscoped), modal (scoped), archived (scoped). */
const ATTENTION_CALL = 0;
const MODAL_CALL = 1;
const ARCHIVED_CALL = 2;

function filtersForCall(call: number): Record<string, unknown> {
  const options = vi.mocked(useSessions).mock.calls[call]?.[1];
  return (options?.filters ?? {}) as Record<string, unknown>;
}

describe("useOsAttention", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useActiveWorkspace).mockReturnValue({
      scope: "workspace",
      activeWorkspaceId: "ws_alpha",
    } as never);
    vi.mocked(useScopedWorktreeFilter).mockReturnValue(undefined);
    vi.mocked(taskScopeForActiveWorkspace).mockReturnValue({} as never);
    vi.mocked(useTaskDashboard).mockReturnValue({
      data: { freshness: { stale: false }, totals: { awaiting_approval_tasks: 0 } },
      isError: false,
      isLoading: false,
    } as never);
    vi.mocked(useTasks).mockReturnValue({ data: [], isError: false, isLoading: false } as never);
  });

  it("Should mark attention disconnected when the archived catalog query fails", () => {
    vi.mocked(useSessions)
      .mockReturnValueOnce(sessionsQuery({}))
      .mockReturnValueOnce(sessionsQuery({}))
      .mockReturnValueOnce(sessionsQuery({ data: undefined, isError: true }));

    const { result } = renderHook(() => useOsAttention(workspace, "live"));

    expect(result.current.sessionsDisconnected).toBe(true);
    expect(result.current.archivedSessions).toEqual([]);
  });

  it("Should keep attention badges workspace-wide while the modal follows the worktree scope", () => {
    vi.mocked(useScopedWorktreeFilter).mockReturnValue("wt_payments");
    const attentionSessions = [waitingSession("sess_other_worktree")];
    vi.mocked(useSessions)
      .mockReturnValueOnce(sessionsQuery({ data: attentionSessions }))
      .mockReturnValueOnce(sessionsQuery({ data: [] }))
      .mockReturnValueOnce(sessionsQuery({ data: [] }));

    const { result } = renderHook(() => useOsAttention(workspace, "live"));

    // The attention query must not carry the scope, or a waiting session in
    // another worktree would stop raising a notification.
    expect(filtersForCall(ATTENTION_CALL).worktree).toBeUndefined();
    expect(filtersForCall(MODAL_CALL).worktree).toBe("wt_payments");
    expect(filtersForCall(ARCHIVED_CALL).worktree).toBe("wt_payments");
    // Badges see the unscoped page; the modal list sees the scoped one.
    expect(result.current.notificationCount).toBeGreaterThan(0);
    expect(result.current.sessions).toEqual([]);
  });

  it("Should drop the worktree filter entirely when the scope falls back to the workspace", () => {
    vi.mocked(useScopedWorktreeFilter).mockReturnValue(undefined);
    vi.mocked(useSessions).mockReturnValue(sessionsQuery({ data: [] }));

    renderHook(() => useOsAttention(workspace, "live"));

    // A missing or unavailable selection sends no filter at all rather than an
    // empty one, so the list matches the fallback notice.
    expect(filtersForCall(MODAL_CALL)).not.toHaveProperty("worktree", expect.anything());
    expect(filtersForCall(MODAL_CALL).worktree).toBeUndefined();
    expect(filtersForCall(ARCHIVED_CALL).worktree).toBeUndefined();
  });

  it("Should not let a scoped page make attention badges look fresh", () => {
    vi.mocked(useScopedWorktreeFilter).mockReturnValue("wt_payments");
    vi.mocked(useSessions)
      // Attention query failed; the scoped modal queries succeeded.
      .mockReturnValueOnce(sessionsQuery({ data: undefined, isError: true }))
      .mockReturnValueOnce(sessionsQuery({ data: [] }))
      .mockReturnValueOnce(sessionsQuery({ data: [] }));

    const { result } = renderHook(() => useOsAttention(workspace, "live"));

    // A stale attention query hides the count rather than reporting zero from
    // the scoped page that happened to load.
    expect(result.current.badges.sessions).toBeUndefined();
  });
});
