import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  taskExecutionProfileFixture,
  taskRunReviewListFixture,
} from "@/systems/tasks/mocks/fixtures";

vi.mock("sonner", () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
  },
}));

vi.mock("@/systems/tasks/adapters/tasks-api", () => ({
  listTasks: vi.fn(),
  getTask: vi.fn(),
  listTaskRuns: vi.fn(),
  getTaskTimeline: vi.fn(),
  getTaskRun: vi.fn(),
  inspectTask: vi.fn().mockResolvedValue(null),
  inspectRun: vi.fn().mockResolvedValue(null),
  getTaskExecutionProfile: vi.fn().mockResolvedValue(null),
  listTaskReviews: vi.fn().mockResolvedValue([]),
  getTaskDashboard: vi.fn(),
  getTaskInbox: vi.fn(),
  recoverTask: vi.fn(),
  recoverTaskRun: vi.fn(),
  approveTask: vi.fn(),
  rejectTask: vi.fn(),
  retryTaskRun: vi.fn(),
  clearTaskBlock: vi.fn(),
  fanOutTaskRuns: vi.fn(),
}));

import {
  approveTask,
  clearTaskBlock,
  fanOutTaskRuns,
  getTask,
  getTaskExecutionProfile,
  getTaskTimeline,
  listTaskReviews,
  listTaskRuns,
  recoverTask,
  recoverTaskRun,
  rejectTask,
  retryTaskRun,
} from "@/systems/tasks/adapters/tasks-api";
import { toast } from "sonner";

import { useTaskDetailPage } from "../use-task-detail-page";

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

const detailFixture = {
  task: { id: "task_001", title: "Review", status: "ready", scope: "workspace" },
  summary: {
    id: "task_001",
    title: "Review",
    status: "ready",
    scope: "workspace",
    active_run: { id: "run_active" },
  },
};

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(getTask).mockResolvedValue(detailFixture as never);
  vi.mocked(getTaskTimeline).mockResolvedValue([{ event_id: "evt_1", sequence: 1 }] as never);
  vi.mocked(listTaskRuns).mockResolvedValue([{ id: "run_1", status: "running" }] as never);
  vi.mocked(getTaskExecutionProfile).mockResolvedValue(taskExecutionProfileFixture);
  vi.mocked(listTaskReviews).mockResolvedValue(taskRunReviewListFixture);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useTaskDetailPage", () => {
  it("Should load detail, timeline, runs, profile, and reviews for a task", async () => {
    const { result } = renderHook(() => useTaskDetailPage("task_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.detail?.task.id).toBe("task_001");
      expect(result.current.timeline).toHaveLength(1);
      expect(result.current.runs).toHaveLength(1);
      expect(result.current.profile).toEqual(taskExecutionProfileFixture);
      expect(result.current.reviews).toEqual(taskRunReviewListFixture);
    });

    expect(result.current.activeRun?.id).toBe("run_active");
  });

  it("Should honor enable flags for optional live reads", async () => {
    const { result } = renderHook(
      () =>
        useTaskDetailPage("task_001", {
          enableTimeline: false,
          enableRuns: false,
        }),
      { wrapper: createWrapper() }
    );

    await waitFor(() => {
      expect(result.current.detail?.task.id).toBe("task_001");
    });

    expect(getTaskTimeline).not.toHaveBeenCalled();
    expect(listTaskRuns).not.toHaveBeenCalled();
  });

  it("Should expose a profile load error through the detail view model", async () => {
    const runtimeError = new Error("Profile unavailable");
    vi.mocked(getTaskExecutionProfile).mockRejectedValue(runtimeError);

    const { result } = renderHook(() => useTaskDetailPage("task_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.profileError).toBe(runtimeError));
    expect(result.current.profile).toBeNull();
  });

  it("Should report a fatal error when no task id is supplied", () => {
    const { result } = renderHook(() => useTaskDetailPage(""), { wrapper: createWrapper() });

    expect(result.current.fatalError).toBeInstanceOf(Error);
    expect(getTask).not.toHaveBeenCalled();
  });

  it("Should increase the timeline page limit when handleTimelineLoadMore is called", () => {
    const { result } = renderHook(
      () => useTaskDetailPage("task_001", { initialTimelineLimit: 25 }),
      { wrapper: createWrapper() }
    );

    expect(result.current.timelineLimit).toBe(25);

    act(() => {
      result.current.handleTimelineLoadMore();
    });

    expect(result.current.timelineLimit).toBeGreaterThan(25);
  });

  it("Should derive an isLive flag from the active run status", async () => {
    vi.mocked(getTask).mockResolvedValue({
      task: { id: "task_001", title: "Review", status: "in_progress", scope: "workspace" },
      summary: {
        id: "task_001",
        title: "Review",
        status: "in_progress",
        scope: "workspace",
        active_run: { id: "run_active", status: "running" },
      },
    } as never);

    const { result } = renderHook(() => useTaskDetailPage("task_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isLive).toBe(true);
    });
  });

  it("Should approve the current task through the approval verb", async () => {
    vi.mocked(approveTask).mockResolvedValue({ id: "task_001" } as never);

    const { result } = renderHook(() => useTaskDetailPage("task_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.detail?.task.id).toBe("task_001"));

    await act(async () => {
      await result.current.handleApproveTask();
    });
    expect(approveTask).toHaveBeenCalledWith("task_001");
  });

  it("Should reject the current task through the rejection verb", async () => {
    vi.mocked(rejectTask).mockResolvedValue({ id: "task_001" } as never);

    const { result } = renderHook(() => useTaskDetailPage("task_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.detail?.task.id).toBe("task_001"));

    await act(async () => {
      await result.current.handleRejectTask();
    });
    expect(rejectTask).toHaveBeenCalledWith("task_001");
  });

  it("Should bind clear-block to the current task and consume a notified rejection", async () => {
    const runtimeError = new Error("Block was already cleared");
    vi.mocked(clearTaskBlock).mockRejectedValue(runtimeError);

    const { result } = renderHook(() => useTaskDetailPage("task_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.detail?.task.id).toBe("task_001"));
    await act(async () => {
      await expect(result.current.handleClearBlock("block_007")).resolves.toBeUndefined();
    });

    expect(clearTaskBlock).toHaveBeenCalledWith("task_001", "block_007", undefined);
    expect(toast.error).toHaveBeenCalledWith("Block was already cleared");
  });

  it("Should bind fan-out payloads to the current task and preserve runtime rejection", async () => {
    const runtimeError = new Error("Task already has an open run");
    vi.mocked(fanOutTaskRuns).mockRejectedValue(runtimeError);
    const request = {
      designations: [{ brief: "Investigate checkout" }],
      network_participation: { mode: "local" as const },
    };

    const { result } = renderHook(() => useTaskDetailPage("task_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.detail?.task.id).toBe("task_001"));
    await act(async () => {
      await expect(result.current.handleFanOutRuns(request)).rejects.toBe(runtimeError);
    });

    expect(fanOutTaskRuns).toHaveBeenCalledWith("task_001", request);
    expect(toast.error).toHaveBeenCalledWith("Task already has an open run");
  });

  it("Should report the number of runs created by a successful fan-out", async () => {
    vi.mocked(fanOutTaskRuns).mockResolvedValue({
      runs: [{ id: "run_designated" }],
    } as never);
    const request = {
      designations: [{ brief: "Investigate checkout" }],
      network_participation: { mode: "local" as const },
    };
    const { result } = renderHook(() => useTaskDetailPage("task_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.detail?.task.id).toBe("task_001"));
    await act(async () => result.current.handleFanOutRuns(request));

    expect(fanOutTaskRuns).toHaveBeenCalledWith("task_001", request);
    expect(toast.success).toHaveBeenCalledWith("Created 1 run.");
  });

  it("Should retry a failed run by run id", async () => {
    vi.mocked(retryTaskRun).mockResolvedValue({
      run: { id: "run_retry", status: "queued" },
    } as never);

    const { result } = renderHook(() => useTaskDetailPage("task_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.detail?.task.id).toBe("task_001"));

    await act(async () => {
      await result.current.handleRetryRun("run_failed");
    });

    expect(retryTaskRun).toHaveBeenCalledWith("run_failed", {});
  });

  it("Should recover the active needs_attention run by run id exactly once", async () => {
    vi.mocked(getTask).mockResolvedValue({
      task: {
        id: "task_001",
        title: "Review",
        status: "ready",
        scope: "workspace",
        max_attempts: 2,
      },
      summary: {
        id: "task_001",
        title: "Review",
        status: "ready",
        scope: "workspace",
        active_run: {
          id: "run_attention",
          attempt: 1,
          status: "needs_attention",
        },
      },
    } as never);
    vi.mocked(recoverTaskRun).mockResolvedValue({
      previous_run: { id: "run_attention", status: "canceled" },
      run: { id: "run_continuation", status: "queued" },
    } as never);

    const { result } = renderHook(() => useTaskDetailPage("task_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.activeRun?.id).toBe("run_attention"));
    await act(async () => {
      await result.current.handleRecoverTask();
    });

    expect(recoverTaskRun).toHaveBeenCalledTimes(1);
    expect(recoverTaskRun).toHaveBeenCalledWith("run_attention", {});
  });

  it("Should not recover through either mutation when the active run exhausted attempts", async () => {
    vi.mocked(getTask).mockResolvedValue({
      task: {
        id: "task_001",
        title: "Review",
        status: "ready",
        scope: "workspace",
        max_attempts: 1,
      },
      summary: {
        id: "task_001",
        title: "Review",
        status: "ready",
        scope: "workspace",
        active_run: {
          id: "run_exhausted",
          attempt: 1,
          status: "needs_attention",
        },
      },
    } as never);

    const { result } = renderHook(() => useTaskDetailPage("task_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.activeRun?.id).toBe("run_exhausted"));
    await act(async () => {
      await result.current.handleRecoverTask();
    });

    expect(recoverTaskRun).not.toHaveBeenCalled();
    expect(recoverTask).not.toHaveBeenCalled();
  });
});
