import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  useApproveTask,
  useArchiveTask,
  useCancelTask,
  useCancelTaskRun,
  useCreateTask,
  useDeleteTask,
  useDismissTask,
  useEnqueueTaskRun,
  useFailTaskRun,
  useForceFailTaskRun,
  useForceReleaseTaskRun,
  useMarkTaskRead,
  usePublishTask,
  useRejectTask,
  useRecoverTaskRun,
  useRetryTaskRun,
  useUpdateTask,
} from "@/systems/tasks";

vi.mock("@/systems/tasks/adapters/tasks-api", () => ({
  createTask: vi.fn(),
  deleteTask: vi.fn(),
  updateTask: vi.fn(),
  publishTask: vi.fn(),
  cancelTask: vi.fn(),
  approveTask: vi.fn(),
  rejectTask: vi.fn(),
  createChildTask: vi.fn(),
  addTaskDependency: vi.fn(),
  removeTaskDependency: vi.fn(),
  enqueueTaskRun: vi.fn(),
  attachTaskRunSession: vi.fn(),
  cancelTaskRun: vi.fn(),
  completeTaskRun: vi.fn(),
  failTaskRun: vi.fn(),
  forceFailTaskRun: vi.fn(),
  forceReleaseTaskRun: vi.fn(),
  recoverTaskRun: vi.fn(),
  retryTaskRun: vi.fn(),
  inspectTask: vi.fn().mockResolvedValue(null),
  inspectRun: vi.fn().mockResolvedValue(null),
  startTaskRun: vi.fn(),
  markTaskRead: vi.fn(),
  archiveTask: vi.fn(),
  dismissTask: vi.fn(),
}));

import {
  approveTask,
  archiveTask,
  cancelTask,
  cancelTaskRun,
  createTask,
  deleteTask,
  dismissTask,
  enqueueTaskRun,
  failTaskRun,
  forceFailTaskRun,
  forceReleaseTaskRun,
  markTaskRead,
  publishTask,
  rejectTask,
  recoverTaskRun,
  retryTaskRun,
  updateTask,
} from "@/systems/tasks/adapters/tasks-api";
import { tasksKeys } from "@/systems/tasks/lib/query-keys";
import { buildTaskFixture } from "@/systems/tasks/mocks/fixtures";

const taskFixture = { id: "task_001", title: "Review", status: "ready" };
const runFixture = { id: "run_001", task_id: "task_001", status: "queued" };
const triageFixture = { task_id: "task_001", read: true };

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

function buildClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
}

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("task mutation hooks", () => {
  it("invalidates task, aggregate, and dependent queries after creating a task", async () => {
    vi.mocked(createTask).mockResolvedValue(taskFixture as never);

    const queryClient = buildClient();
    const listKey = tasksKeys.list({ scope: "global", limit: 1 });
    const cachedList = {
      pageParams: [undefined],
      pages: [
        {
          facets: { owners: [], statuses: [{ count: 1, status: "ready" }] },
          page: { has_more: false, limit: 1, total: 1 },
          tasks: [buildTaskFixture({ active_run: null, status: "ready" })],
        },
      ],
    };
    queryClient.setQueryData(listKey, cachedList);
    const spy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useCreateTask(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      result.current.mutate({ title: "Review", scope: "workspace" });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(createTask).toHaveBeenCalledWith(expect.objectContaining({ title: "Review" }));
    expect(spy).toHaveBeenCalledWith({ queryKey: ["tasks", "list"] });
    expect(spy).toHaveBeenCalledWith({ queryKey: ["tasks", "dashboard"] });
    expect(spy).toHaveBeenCalledWith({ queryKey: ["tasks", "inbox"] });
    expect(queryClient.getQueryData(listKey)).toEqual(cachedList);
  });

  it("invalidates task detail when updating a task", async () => {
    vi.mocked(updateTask).mockResolvedValue(taskFixture as never);

    const queryClient = buildClient();
    const spy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useUpdateTask(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      result.current.mutate({ id: "task_001", data: { title: "Next" } });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(updateTask).toHaveBeenCalledWith("task_001", { title: "Next" });
    expect(spy).toHaveBeenCalledWith({ queryKey: ["tasks", "detail", "task_001"] });
  });

  it("removes task detail cache and invalidates aggregates when deleting a task", async () => {
    vi.mocked(deleteTask).mockResolvedValue(undefined);

    const queryClient = buildClient();
    queryClient.setQueryData(["tasks", "detail", "task_001"], taskFixture);
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    const removeSpy = vi.spyOn(queryClient, "removeQueries");

    const { result } = renderHook(() => useDeleteTask(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      result.current.mutate({ id: "task_001" });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(deleteTask).toHaveBeenCalledWith("task_001");
    expect(removeSpy).toHaveBeenCalledWith({ queryKey: ["tasks", "detail", "task_001"] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["tasks", "dashboard"] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["tasks", "inbox"] });
  });

  it("invalidates task surfaces after publish, cancel, approve, reject", async () => {
    vi.mocked(publishTask).mockResolvedValue(taskFixture as never);
    vi.mocked(cancelTask).mockResolvedValue(taskFixture as never);
    vi.mocked(approveTask).mockResolvedValue(taskFixture as never);
    vi.mocked(rejectTask).mockResolvedValue(taskFixture as never);

    const queryClient = buildClient();
    const spy = vi.spyOn(queryClient, "invalidateQueries");

    const publishHook = renderHook(() => usePublishTask(), {
      wrapper: createWrapper(queryClient),
    });
    const cancelHook = renderHook(() => useCancelTask(), {
      wrapper: createWrapper(queryClient),
    });
    const approveHook = renderHook(() => useApproveTask(), {
      wrapper: createWrapper(queryClient),
    });
    const rejectHook = renderHook(() => useRejectTask(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      publishHook.result.current.mutate({ id: "task_001" });
      cancelHook.result.current.mutate({ id: "task_001" });
      approveHook.result.current.mutate({ id: "task_001" });
      rejectHook.result.current.mutate({ id: "task_001" });
    });

    await waitFor(() => {
      expect(publishHook.result.current.isSuccess).toBe(true);
      expect(cancelHook.result.current.isSuccess).toBe(true);
      expect(approveHook.result.current.isSuccess).toBe(true);
      expect(rejectHook.result.current.isSuccess).toBe(true);
    });

    expect(cancelTask).toHaveBeenCalledWith("task_001", {});
    expect(spy).toHaveBeenCalledWith({ queryKey: ["tasks", "detail", "task_001"] });
    expect(spy).toHaveBeenCalledWith({ queryKey: ["tasks", "dashboard"] });
  });

  it("invalidates run and task surfaces when running task-run commands", async () => {
    vi.mocked(enqueueTaskRun).mockResolvedValue(runFixture as never);
    vi.mocked(cancelTaskRun).mockResolvedValue(runFixture as never);
    vi.mocked(failTaskRun).mockResolvedValue(runFixture as never);
    vi.mocked(forceReleaseTaskRun).mockResolvedValue(runFixture as never);
    vi.mocked(forceFailTaskRun).mockResolvedValue(runFixture as never);
    vi.mocked(retryTaskRun).mockResolvedValue({
      previous_run: runFixture,
      run: runFixture,
    } as never);

    const queryClient = buildClient();
    const spy = vi.spyOn(queryClient, "invalidateQueries");

    const enqueueHook = renderHook(() => useEnqueueTaskRun(), {
      wrapper: createWrapper(queryClient),
    });
    const cancelRunHook = renderHook(() => useCancelTaskRun(), {
      wrapper: createWrapper(queryClient),
    });
    const failRunHook = renderHook(() => useFailTaskRun(), {
      wrapper: createWrapper(queryClient),
    });
    const forceReleaseHook = renderHook(() => useForceReleaseTaskRun(), {
      wrapper: createWrapper(queryClient),
    });
    const forceFailHook = renderHook(() => useForceFailTaskRun(), {
      wrapper: createWrapper(queryClient),
    });
    const retryHook = renderHook(() => useRetryTaskRun(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      enqueueHook.result.current.mutate({ id: "task_001" });
      cancelRunHook.result.current.mutate({ runId: "run_001" });
      failRunHook.result.current.mutate({ runId: "run_001", data: { error: "boom" } });
      forceReleaseHook.result.current.mutate({ runId: "run_001", data: { reason: "handoff" } });
      forceFailHook.result.current.mutate({
        runId: "run_001",
        data: { reason: "operator recovery" },
      });
      retryHook.result.current.mutate({
        runId: "run_001",
        data: { metadata: { source: "operator" } },
      });
    });

    await waitFor(() => {
      expect(enqueueHook.result.current.isSuccess).toBe(true);
      expect(cancelRunHook.result.current.isSuccess).toBe(true);
      expect(failRunHook.result.current.isSuccess).toBe(true);
      expect(forceReleaseHook.result.current.isSuccess).toBe(true);
      expect(forceFailHook.result.current.isSuccess).toBe(true);
      expect(retryHook.result.current.isSuccess).toBe(true);
    });

    expect(enqueueTaskRun).toHaveBeenCalledWith("task_001", {});
    expect(cancelTaskRun).toHaveBeenCalledWith("run_001", {});
    expect(failTaskRun).toHaveBeenCalledWith("run_001", { error: "boom" });
    expect(forceReleaseTaskRun).toHaveBeenCalledWith("run_001", { reason: "handoff" });
    expect(forceFailTaskRun).toHaveBeenCalledWith("run_001", { reason: "operator recovery" });
    expect(retryTaskRun).toHaveBeenCalledWith("run_001", { metadata: { source: "operator" } });
    expect(spy).toHaveBeenCalledWith({ queryKey: ["tasks", "run-detail", "run_001"] });
  });

  it("targets the owning task and aggregate queries after recovering a run", async () => {
    vi.mocked(recoverTaskRun).mockResolvedValue({
      previous_run: { ...runFixture, status: "needs_attention" },
      run: { ...runFixture, id: "run_002", status: "queued" },
    } as never);

    const queryClient = buildClient();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useRecoverTaskRun(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      result.current.mutate({ runId: "run_001", taskId: "task_001" });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(recoverTaskRun).toHaveBeenCalledTimes(1);
    expect(recoverTaskRun).toHaveBeenCalledWith("run_001", {});
    expect(spy).toHaveBeenCalledWith({ queryKey: ["tasks", "run-detail", "run_001"] });
    expect(spy).toHaveBeenCalledWith({ queryKey: ["tasks", "detail", "task_001"] });
    expect(spy).toHaveBeenCalledWith({ queryKey: ["tasks", "inbox"] });
  });

  it("invalidates triage, list, detail, and inbox queries on triage actions", async () => {
    vi.mocked(markTaskRead).mockResolvedValue(triageFixture as never);
    vi.mocked(archiveTask).mockResolvedValue(triageFixture as never);
    vi.mocked(dismissTask).mockResolvedValue(triageFixture as never);

    const queryClient = buildClient();
    const spy = vi.spyOn(queryClient, "invalidateQueries");

    const readHook = renderHook(() => useMarkTaskRead(), {
      wrapper: createWrapper(queryClient),
    });
    const archiveHook = renderHook(() => useArchiveTask(), {
      wrapper: createWrapper(queryClient),
    });
    const dismissHook = renderHook(() => useDismissTask(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      readHook.result.current.mutate({ id: "task_001" });
      archiveHook.result.current.mutate({ id: "task_001" });
      dismissHook.result.current.mutate({ id: "task_001" });
    });

    await waitFor(() => {
      expect(readHook.result.current.isSuccess).toBe(true);
      expect(archiveHook.result.current.isSuccess).toBe(true);
      expect(dismissHook.result.current.isSuccess).toBe(true);
    });

    expect(spy).toHaveBeenCalledWith({ queryKey: ["tasks", "triage"] });
    expect(spy).toHaveBeenCalledWith({ queryKey: ["tasks", "list"] });
    expect(spy).toHaveBeenCalledWith({ queryKey: ["tasks", "detail", "task_001"] });
    expect(spy).toHaveBeenCalledWith({ queryKey: ["tasks", "inbox"] });
  });
});
