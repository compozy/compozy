// Suite: OS attention data readiness
// Invariant: either session catalog query failing leaves attention disconnected instead of implying no sessions.
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
vi.mock("@/systems/workspace/hooks/use-user-home-dir", () => ({ useUserHomeDir: vi.fn() }));

import { useOsAttention } from "../use-os-attention";
import { useSessions } from "@/systems/session";
import { taskScopeForActiveWorkspace, useTaskDashboard, useTasks } from "@/systems/tasks";
import { useUserHomeDir, type WorkspacePayload } from "@/systems/workspace";

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

describe("useOsAttention", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useUserHomeDir).mockReturnValue("/workspace");
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
      .mockReturnValueOnce(sessionsQuery({ data: undefined, isError: true }));

    const { result } = renderHook(() => useOsAttention(workspace, "live"));

    expect(result.current.sessionsDisconnected).toBe(true);
    expect(result.current.archivedSessions).toEqual([]);
  });
});
