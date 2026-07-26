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
  listTaskRunReviews,
  recoverTaskRun,
} from "@/systems/tasks/adapters/tasks-api";

import { useTaskRunPage } from "../use-task-run-page";

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
  vi.mocked(getTaskRun).mockResolvedValue(runDetailFixture as never);
  vi.mocked(getTask).mockResolvedValue(taskDetailFixture as never);
  vi.mocked(listTaskRunReviews).mockResolvedValue([] as never);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useTaskRunPage", () => {
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

  it("exposes a handleCancelRun action", async () => {
    const { result } = renderHook(() => useTaskRunPage("task_001", "run_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.run?.run.id).toBe("run_001");
    });

    expect(typeof result.current.handleCancelRun).toBe("function");
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
