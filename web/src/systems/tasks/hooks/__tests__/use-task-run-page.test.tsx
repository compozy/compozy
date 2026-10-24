// Suite: task run page hook
// Invariant: run-detail reads remain scoped and issue no network work while their window is inactive.
// Boundary IN: run query admission, polling, and composed page state.
// Boundary OUT: HTTP contracts and visual rendering, owned by adapter and component suites.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/systems/tasks/adapters/tasks-api", () => ({
  listTasks: vi.fn(),
  getTask: vi.fn(),
  listTaskRuns: vi.fn(),
  getTaskTimeline: vi.fn(),
  getTaskTree: vi.fn(),
  getTaskRun: vi.fn(),
  inspectTask: vi.fn().mockResolvedValue(null),
  inspectRun: vi.fn().mockResolvedValue(null),
  getTaskDashboard: vi.fn(),
  getTaskInbox: vi.fn(),
  listTaskRunReviews: vi.fn(),
  recoverTaskRun: vi.fn(),
}));

import {
  getTask,
  getTaskRun,
  inspectRun,
  listTaskRunReviews,
  recoverTaskRun,
} from "@/systems/tasks/adapters/tasks-api";

import { useTaskRunPage } from "../use-task-run-page";
import { resetProfileViews, setProfileView } from "@/systems/profiles";

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

const runDetailFixture = {
  run: { id: "run_001", task_id: "task_001", status: "running" },
  task: { id: "task_001", title: "Review", status: "ready", scope: "workspace" },
  summary: { last_activity_at: "2026-04-11T09:00:00Z" },
  session: {
    session_id: "sess_a",
    created_at: "2026-04-11T09:00:00Z",
    updated_at: "2026-04-11T09:00:00Z",
  },
};

const taskDetailFixture = {
  task: { id: "task_001", title: "Review", status: "ready", scope: "workspace" },
  summary: { id: "task_001", title: "Review", status: "ready", scope: "workspace" },
};

beforeEach(() => {
  vi.clearAllMocks();
  resetProfileViews();
  vi.mocked(getTaskRun).mockResolvedValue(runDetailFixture as never);
  vi.mocked(getTask).mockResolvedValue(taskDetailFixture as never);
  vi.mocked(listTaskRunReviews).mockResolvedValue([] as never);
});

afterEach(() => {
  act(() => resetProfileViews());
  vi.restoreAllMocks();
});

describe("useTaskRunPage loading", () => {
  it("Should read as loading until the run answers, and never again for a refetch", async () => {
    let settle!: (run: unknown) => void;
    vi.mocked(getTaskRun).mockReturnValueOnce(
      new Promise(resolve => {
        settle = resolve;
      }) as never
    );
    const { result } = renderHook(() => useTaskRunPage("task_001", "run_001"), {
      wrapper: createWrapper(),
    });
    expect(result.current.runLoading).toBe(true);

    await act(async () => settle(runDetailFixture));
    await waitFor(() => expect(result.current.runLoading).toBe(false));

    // A refetch with the run already in hand is not a load: the page keeps the run it has.
    let settleRefetch!: (run: unknown) => void;
    vi.mocked(getTaskRun).mockReturnValueOnce(
      new Promise(resolve => {
        settleRefetch = resolve;
      }) as never
    );
    act(() => void result.current.handleRetryLoad());
    await waitFor(() => expect(getTaskRun).toHaveBeenCalledTimes(2));
    expect(result.current.runLoading).toBe(false);
    expect(result.current.run?.run.id).toBe("run_001");
    await act(async () => settleRefetch(runDetailFixture));
  });

  it("Should keep a suspended window waiting rather than calling the run missing", () => {
    const { result } = renderHook(
      () => useTaskRunPage("task_001", "run_001", { liveDataEnabled: false }),
      { wrapper: createWrapper() }
    );

    expect(result.current.runLoading).toBe(true);
    expect(result.current.notFound).toBeFalsy();
    expect(getTaskRun).not.toHaveBeenCalled();
  });

  it("Should keep the secondary reads waiting while suspended, and wait on none opted out of", () => {
    const suspended = renderHook(
      () => useTaskRunPage("task_001", "run_001", { liveDataEnabled: false }),
      { wrapper: createWrapper() }
    );
    expect(suspended.result.current.inspectLoading).toBe(true);
    expect(suspended.result.current.reviewsLoading).toBe(true);

    const optedOut = renderHook(
      () =>
        useTaskRunPage("task_001", "run_001", {
          enableInspect: false,
          enableRunReviews: false,
          liveDataEnabled: false,
        }),
      { wrapper: createWrapper() }
    );
    expect(optedOut.result.current.inspectLoading).toBe(false);
    expect(optedOut.result.current.reviewsLoading).toBe(false);
  });

  it("Should not wait for a run that has no id", () => {
    const { result } = renderHook(() => useTaskRunPage("task_001", ""), {
      wrapper: createWrapper(),
    });

    expect(result.current.runLoading).toBe(false);
    expect(result.current.fatalError?.message).toBe("Missing run id");
  });
});

describe("useTaskRunPage", () => {
  it("Should scope every run-page read and isolate cached results after a profile switch", async () => {
    setProfileView({ kind: "profile", profile: "marketing" });
    const { result } = renderHook(() => useTaskRunPage("task_001", "run_001"), {
      wrapper: createWrapper(),
    });
    await waitFor(() => expect(result.current.run?.run.id).toBe("run_001"));
    expect(getTaskRun).toHaveBeenLastCalledWith(
      "run_001",
      { profile: "marketing" },
      expect.any(AbortSignal)
    );
    expect(inspectRun).toHaveBeenLastCalledWith(
      "run_001",
      { profile: "marketing" },
      expect.any(AbortSignal)
    );
    expect(listTaskRunReviews).toHaveBeenLastCalledWith(
      "run_001",
      { profile: "marketing" },
      expect.any(AbortSignal)
    );

    vi.mocked(getTaskRun).mockRejectedValue(new Error("Task run not found: run_001"));
    act(() => setProfileView({ kind: "profile", profile: "default" }));
    await waitFor(() => expect(result.current.notFound).toBe(true));
    expect(result.current.run).toBeNull();
    expect(getTaskRun).toHaveBeenLastCalledWith(
      "run_001",
      { profile: "default" },
      expect.any(AbortSignal)
    );

    vi.mocked(getTaskRun).mockResolvedValue(runDetailFixture as never);
    act(() => setProfileView({ kind: "aggregate" }));
    await waitFor(() => expect(result.current.run?.run.id).toBe("run_001"));
    expect(getTaskRun).toHaveBeenLastCalledWith(
      "run_001",
      { all_profiles: true },
      expect.any(AbortSignal)
    );
    expect(inspectRun).toHaveBeenLastCalledWith(
      "run_001",
      { all_profiles: true },
      expect.any(AbortSignal)
    );
    expect(listTaskRunReviews).toHaveBeenLastCalledWith(
      "run_001",
      { all_profiles: true },
      expect.any(AbortSignal)
    );
  });

  it("loads run detail and task detail together", async () => {
    const { result } = renderHook(() => useTaskRunPage("task_001", "run_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.run?.run.id).toBe("run_001");
      expect(result.current.task?.task.id).toBe("task_001");
    });

    expect(result.current.session?.session_id).toBe("sess_a");
    expect(result.current.summary?.last_activity_at).toBe("2026-04-11T09:00:00Z");
  });

  it("reports fatal error when ids are missing", () => {
    const { result } = renderHook(() => useTaskRunPage("", ""), { wrapper: createWrapper() });

    expect(result.current.fatalError).toBeInstanceOf(Error);
    expect(getTaskRun).not.toHaveBeenCalled();
    expect(getTask).not.toHaveBeenCalled();
  });

  it("does not observe run data while the retained window is inactive", () => {
    const { result } = renderHook(
      () => useTaskRunPage("task_001", "run_001", { liveDataEnabled: false }),
      { wrapper: createWrapper() }
    );

    expect(result.current.run).toBeNull();
    expect(getTaskRun).not.toHaveBeenCalled();
    expect(inspectRun).not.toHaveBeenCalled();
    expect(getTask).not.toHaveBeenCalled();
    expect(listTaskRunReviews).not.toHaveBeenCalled();
  });

  it("skips task detail query when disabled", async () => {
    const { result } = renderHook(
      () => useTaskRunPage("task_001", "run_001", { enableTaskDetail: false }),
      { wrapper: createWrapper() }
    );

    await waitFor(() => {
      expect(result.current.run?.run.id).toBe("run_001");
    });

    expect(getTask).not.toHaveBeenCalled();
  });

  it("derives isLive from the current run status", async () => {
    const { result } = renderHook(() => useTaskRunPage("task_001", "run_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isLive).toBe(true);
    });
  });

  it("recovers the current needs_attention run by run id exactly once", async () => {
    vi.mocked(getTaskRun).mockResolvedValue({
      ...runDetailFixture,
      run: { ...runDetailFixture.run, status: "needs_attention" },
    } as never);
    vi.mocked(recoverTaskRun).mockResolvedValue({
      previous_run: { id: "run_001", status: "canceled" },
      run: { id: "run_002", status: "queued" },
    } as never);

    const { result } = renderHook(() => useTaskRunPage("task_001", "run_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.run?.run.status).toBe("needs_attention"));
    await act(async () => {
      await result.current.handleRecoverRun();
    });

    expect(recoverTaskRun).toHaveBeenCalledTimes(1);
    expect(recoverTaskRun).toHaveBeenCalledWith("run_001", {});
  });

  it("loads run reviews when run id is provided and reviews are enabled", async () => {
    vi.mocked(listTaskRunReviews).mockResolvedValueOnce([
      { review_id: "review_001", run_id: "run_001" },
    ] as never);

    const { result } = renderHook(() => useTaskRunPage("task_001", "run_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.reviews.length).toBe(1);
    });
    expect(listTaskRunReviews).toHaveBeenCalled();
  });

  it("skips run reviews query when reviews are disabled", async () => {
    const { result } = renderHook(
      () => useTaskRunPage("task_001", "run_001", { enableRunReviews: false }),
      { wrapper: createWrapper() }
    );

    await waitFor(() => {
      expect(result.current.run?.run.id).toBe("run_001");
    });
    expect(result.current.reviews).toEqual([]);
    expect(listTaskRunReviews).not.toHaveBeenCalled();
  });
});
