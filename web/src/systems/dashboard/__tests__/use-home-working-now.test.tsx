import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

const useSessions = vi.fn();
const useTaskDashboard = vi.fn();

vi.mock("@/systems/session/hooks/use-sessions", () => ({
  useSessions: (...args: unknown[]) => useSessions(...args),
}));

vi.mock("@/systems/tasks/hooks/use-task-dashboard", () => ({
  useTaskDashboard: (...args: unknown[]) => useTaskDashboard(...args),
}));

import { useHomeWorkingNow } from "../hooks/use-home-working-now";
import type { HomeScope } from "../lib/home-scope";

const scope = {
  dataScope: { kind: "all" },
  workspaceParam: "",
  taskScope: { all_workspaces: true },
  overview: { allWorkspaces: true },
} satisfies HomeScope;

function wrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useHomeWorkingNow", () => {
  it("Should count only the deduplicated loaded live set, never a hidden aggregate", () => {
    // The backend totals report more work than the loaded pages, and one loaded
    // run is bound to a loaded session. A truthful count must dedupe that run and
    // never add the unloaded tail — those hidden rows could be session-bound too.
    useSessions.mockReturnValue({
      data: [
        {
          id: "sess-1",
          agent_name: "coder",
          name: "Coder",
          created_at: "2026-07-24T00:00:00Z",
          activity: undefined,
        },
      ],
      total: 5,
      isLoading: false,
      isError: false,
    });
    useTaskDashboard.mockReturnValue({
      data: {
        active_runs: {
          items: [
            {
              run_id: "run-bound",
              session_id: "sess-1",
              task_id: "task-1",
              task_title: "Bound run",
              run_status: "running",
              age_ms: 1000,
              task_owner: null,
            },
            {
              run_id: "run-standalone",
              session_id: null,
              task_id: "task-2",
              task_title: "Standalone run",
              run_status: "running",
              age_ms: 2000,
              task_owner: null,
            },
          ],
          total: 9,
        },
      },
      isLoading: false,
      isError: false,
    });

    const { result } = renderHook(() => useHomeWorkingNow(scope, true), { wrapper: wrapper() });
    const model = result.current;

    expect(model.cards.map(card => card.key)).toEqual(["session:sess-1", "run:run-standalone"]);
    expect(model.total).toBe(model.cards.length);
    expect(model.total).toBe(2);
    expect(model.sessionCount).toBe(1);
    expect(model.runCount).toBe(1);
    expect(model.status).toBe("ready");
    expect(model.errorMessage).toBeNull();
  });

  // S7: under All projects every live card names its project and opens bound to it; the
  // task runs come from the all_workspaces aggregate, never `scope=global`.
  it("Should badge each card with its project under All projects", () => {
    useSessions.mockReturnValue({
      data: [
        {
          id: "sess-1",
          agent_name: "coder",
          name: "Coder",
          workspace_id: "ws-b",
          created_at: "2026-07-24T00:00:00Z",
          activity: undefined,
        },
      ],
      isLoading: false,
      isError: false,
    });
    useTaskDashboard.mockReturnValue({
      data: {
        active_runs: {
          items: [
            {
              run_id: "run-user",
              task_id: "task-1",
              task_title: "User run",
              run_status: "running",
              age_ms: 1000,
              task_owner: null,
            },
          ],
          total: 1,
        },
      },
      isLoading: false,
      isError: false,
    });

    const names = new Map([["ws-b", "Beta"]]);
    const { result } = renderHook(() => useHomeWorkingNow(scope, true, names), {
      wrapper: wrapper(),
    });

    expect(useTaskDashboard).toHaveBeenLastCalledWith(
      { all_workspaces: true },
      expect.objectContaining({ enabled: true })
    );
    expect(result.current.cards.map(card => card.project)).toEqual([
      { badge: expect.objectContaining({ kind: "project", name: "Beta" }), binding: "ws-b" },
      { badge: { kind: "user", name: "User", monogram: "~" }, binding: "@user" },
    ]);

    const { result: focused } = renderHook(
      () =>
        useHomeWorkingNow(
          {
            dataScope: { kind: "project", workspaceId: "ws-b" },
            workspaceParam: "ws-b",
            taskScope: { scope: "workspace", workspace: "ws-b" },
            overview: { workspace: "ws-b" },
          },
          true,
          names
        ),
      { wrapper: wrapper() }
    );
    expect(focused.current.cards.every(card => card.project === undefined)).toBe(true);
  });

  it("Should surface an error when one source fails and no cards loaded", () => {
    // Regression: the old `&&` let a failed sessions read hide behind an empty
    // task dashboard, rendering "No agents working right now" as if both reads
    // had succeeded. Any failed source with nothing to show is an error.
    useSessions.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error("sessions endpoint unreachable"),
    });
    useTaskDashboard.mockReturnValue({
      data: { active_runs: { items: [], total: 0 } },
      isLoading: false,
      isError: false,
      error: null,
    });

    const { result } = renderHook(() => useHomeWorkingNow(scope, true), { wrapper: wrapper() });
    const model = result.current;

    expect(model.cards).toHaveLength(0);
    expect(model.status).toBe("error");
    expect(model.errorMessage).toBe("sessions endpoint unreachable");
  });

  it("Should stay partial when one source fails but the other returned cards", () => {
    // Sessions loaded; the task dashboard failed. The loaded cards are shown but
    // the surface is flagged incomplete rather than passed off as the whole set.
    useSessions.mockReturnValue({
      data: [
        {
          id: "sess-1",
          agent_name: "coder",
          name: "Coder",
          created_at: "2026-07-24T00:00:00Z",
          activity: undefined,
        },
      ],
      isLoading: false,
      isError: false,
      error: null,
    });
    useTaskDashboard.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error("task dashboard unavailable"),
    });

    const { result } = renderHook(() => useHomeWorkingNow(scope, true), { wrapper: wrapper() });
    const model = result.current;

    expect(model.cards.map(card => card.key)).toEqual(["session:sess-1"]);
    expect(model.status).toBe("partial");
    expect(model.errorMessage).toBe("task dashboard unavailable");
  });

  it("Should stay loading while a source is in flight with nothing loaded yet", () => {
    // A settled failure must not flash an error over a still-pending read.
    useSessions.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      error: null,
    });
    useTaskDashboard.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
      error: null,
    });

    const { result } = renderHook(() => useHomeWorkingNow(scope, true), { wrapper: wrapper() });
    const model = result.current;

    expect(model.cards).toHaveLength(0);
    expect(model.status).toBe("loading");
    expect(model.errorMessage).toBeNull();
  });

  it("Should keep both sources disabled until the scope is settled", () => {
    // Until the home dir resolves the scope, neither source may fetch — a
    // disabled gate is how a project workspace avoids a global first read.
    useSessions.mockReturnValue({ data: [], isLoading: false, isError: false, error: null });
    useTaskDashboard.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
      error: null,
    });

    renderHook(() => useHomeWorkingNow(scope, false), { wrapper: wrapper() });

    expect(useSessions).toHaveBeenCalledWith(null, expect.objectContaining({ enabled: false }));
    expect(useTaskDashboard).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ enabled: false })
    );
  });
});
