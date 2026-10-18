import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  agentContextFixture,
  taskContextBundleFixture,
  taskExecutionProfileFixture,
  taskRunReviewFixture,
  taskRunReviewListFixture,
  taskRunReviewVerdictResultFixture,
} from "@/systems/tasks/mocks/fixtures";
import {
  useAgentContext,
  useDeleteTaskExecutionProfile,
  useRequestTaskRunReview,
  useSetTaskExecutionProfile,
  useSubmitTaskRunReviewVerdict,
  useTaskContextBundle,
  useTaskExecutionProfile,
  useTaskReviews,
  useTaskRunReview,
  useTaskRunReviews,
} from "@/systems/tasks";

vi.mock("@/systems/tasks/adapters/tasks-api", () => ({
  TasksApiError: class TasksApiError extends Error {},
  getTaskExecutionProfile: vi.fn(),
  setTaskExecutionProfile: vi.fn(),
  deleteTaskExecutionProfile: vi.fn(),
  listTaskRunReviews: vi.fn(),
  listTaskReviews: vi.fn(),
  getTaskRunReview: vi.fn(),
  requestTaskRunReview: vi.fn(),
  submitTaskRunReviewVerdict: vi.fn(),
  inspectTask: vi.fn().mockResolvedValue(null),
  inspectRun: vi.fn().mockResolvedValue(null),
  forceFailTaskRun: vi.fn(),
  forceReleaseTaskRun: vi.fn(),
  retryTaskRun: vi.fn(),
  getAgentContext: vi.fn(),
}));

import {
  deleteTaskExecutionProfile,
  getAgentContext,
  getTaskExecutionProfile,
  getTaskRunReview,
  listTaskReviews,
  listTaskRunReviews,
  requestTaskRunReview,
  setTaskExecutionProfile,
  submitTaskRunReviewVerdict,
} from "@/systems/tasks/adapters/tasks-api";

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("execution profile hooks", () => {
  it("Should load profile and disable on empty id", async () => {
    vi.mocked(getTaskExecutionProfile).mockResolvedValue(taskExecutionProfileFixture);

    const { result } = renderHook(() => useTaskExecutionProfile("task_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.data?.task_id).toBe("task_001");
    });

    renderHook(() => useTaskExecutionProfile(""), { wrapper: createWrapper() });
    expect(getTaskExecutionProfile).toHaveBeenCalledTimes(1);
  });

  it("Should expose loading state before resolution", () => {
    vi.mocked(getTaskExecutionProfile).mockImplementation(() => new Promise(() => undefined));

    const { result } = renderHook(() => useTaskExecutionProfile("task_001"), {
      wrapper: createWrapper(),
    });

    expect(result.current.isLoading).toBe(true);
    expect(result.current.data).toBeUndefined();
  });

  it("Should expose error state on failure", async () => {
    vi.mocked(getTaskExecutionProfile).mockRejectedValue(new Error("boom"));

    const { result } = renderHook(() => useTaskExecutionProfile("task_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
  });

  it("Should call set mutation with id and body", async () => {
    vi.mocked(setTaskExecutionProfile).mockResolvedValue(taskExecutionProfileFixture);

    const { result } = renderHook(() => useSetTaskExecutionProfile(), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync({
        id: "task_001",
        data: taskExecutionProfileFixture,
      });
    });

    expect(setTaskExecutionProfile).toHaveBeenCalledWith("task_001", taskExecutionProfileFixture);
  });

  it("Should call delete mutation with id", async () => {
    vi.mocked(deleteTaskExecutionProfile).mockResolvedValue(undefined);

    const { result } = renderHook(() => useDeleteTaskExecutionProfile(), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync({ id: "task_001" });
    });

    expect(deleteTaskExecutionProfile).toHaveBeenCalledWith("task_001");
  });
});

describe("review hooks", () => {
  it("Should load run reviews and respect filters", async () => {
    vi.mocked(listTaskRunReviews).mockResolvedValue(taskRunReviewListFixture);

    const { result } = renderHook(() => useTaskRunReviews("run_001", { status: "in_review" }), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.data).toHaveLength(2);
    });

    expect(listTaskRunReviews).toHaveBeenCalledWith(
      "run_001",
      { status: "in_review", profile: "default" },
      expect.any(AbortSignal)
    );
  });

  it("Should expose empty state when reviews list is empty", async () => {
    vi.mocked(listTaskRunReviews).mockResolvedValue([]);

    const { result } = renderHook(() => useTaskRunReviews("run_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.data).toEqual([]);
    });
  });

  it("Should load task-level reviews and review detail", async () => {
    vi.mocked(listTaskReviews).mockResolvedValue(taskRunReviewListFixture);
    vi.mocked(getTaskRunReview).mockResolvedValue(taskRunReviewFixture);

    const taskReviews = renderHook(() => useTaskReviews("task_001"), {
      wrapper: createWrapper(),
    });
    const reviewDetail = renderHook(() => useTaskRunReview("review_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(taskReviews.result.current.data).toHaveLength(2);
      expect(reviewDetail.result.current.data?.review_id).toBe("review_001");
    });

    expect(listTaskReviews).toHaveBeenCalledWith(
      "task_001",
      { profile: "default" },
      expect.any(AbortSignal)
    );
    expect(getTaskRunReview).toHaveBeenCalledWith("review_001", expect.any(AbortSignal));
  });

  it("Should request a review via mutation", async () => {
    vi.mocked(requestTaskRunReview).mockResolvedValue({
      review: taskRunReviewFixture,
      created: true,
    });

    const { result } = renderHook(() => useRequestTaskRunReview(), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync({
        runId: "run_001",
        data: {
          run_id: "run_001",
          task_id: "task_001",
          deadline_at: "2026-04-17T19:00:00Z",
        },
      });
    });

    expect(requestTaskRunReview).toHaveBeenCalledWith(
      "run_001",
      expect.objectContaining({
        run_id: "run_001",
        task_id: "task_001",
      })
    );
  });

  it("Should submit a verdict via mutation", async () => {
    vi.mocked(submitTaskRunReviewVerdict).mockResolvedValue(taskRunReviewVerdictResultFixture);

    const { result } = renderHook(() => useSubmitTaskRunReviewVerdict(), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync({
        reviewId: "review_001",
        data: {
          run_id: "run_001",
          verdict: {
            outcome: "rejected",
            reason: "missing artifacts",
            confidence: 0.5,
            delivery_id: "delivery_review_001",
          },
        },
      });
    });

    expect(submitTaskRunReviewVerdict).toHaveBeenCalledWith(
      "review_001",
      expect.objectContaining({ run_id: "run_001" })
    );
  });
});

describe("agent context hooks", () => {
  const identity = { agentName: "worker", sessionId: "session_001" };

  it("Should expose full agent context", async () => {
    vi.mocked(getAgentContext).mockResolvedValue(agentContextFixture);

    const { result } = renderHook(() => useAgentContext(identity), { wrapper: createWrapper() });

    await waitFor(() => {
      expect(result.current.data?.task.bundle?.task.id).toBe("task_001");
    });
    expect(vi.mocked(getAgentContext).mock.calls[0]?.[0]).toEqual(identity);
  });

  it("Should extract task context bundle", async () => {
    vi.mocked(getAgentContext).mockResolvedValue(agentContextFixture);

    const { result } = renderHook(() => useTaskContextBundle(identity), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.data?.latest_event_seq).toBe(taskContextBundleFixture.latest_event_seq);
    });
    expect(vi.mocked(getAgentContext).mock.calls[0]?.[0]).toEqual(identity);
    expect(getAgentContext).toHaveBeenCalledTimes(1);
  });
});
