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
  createTask: vi.fn(),
  updateTask: vi.fn(),
  publishTask: vi.fn(),
  cancelTask: vi.fn(),
  approveTask: vi.fn(),
  rejectTask: vi.fn(),
  createChildTask: vi.fn(),
  addTaskDependency: vi.fn(),
  removeTaskDependency: vi.fn(),
  enqueueTaskRun: vi.fn(),
  retryTaskRun: vi.fn(),
  attachTaskRunSession: vi.fn(),
  cancelTaskRun: vi.fn(),
  startTaskRun: vi.fn(),
  completeTaskRun: vi.fn(),
  failTaskRun: vi.fn(),
  markTaskRead: vi.fn(),
  archiveTask: vi.fn(),
  dismissTask: vi.fn(),
}));

vi.mock("@/systems/scheduler/adapters/scheduler-api", () => ({
  getScheduler: vi.fn(),
  getSchedulerBacklog: vi.fn(),
  pauseScheduler: vi.fn(),
  resumeScheduler: vi.fn(),
  drainScheduler: vi.fn(),
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

const workspaceMockState = vi.hoisted(() => ({
  error: null as Error | null,
  hasHydrated: true,
  isLoading: false,
  selectedWorkspaceId: "ws_alpha" as string | null,
}));

const daemonStatusMockState = vi.hoisted(
  (): {
    data: { user_home_dir: string } | undefined;
    error: Error | null;
    isLoading: boolean;
    isPending: boolean;
  } => ({
    data: { user_home_dir: "/Users/operator" },
    error: null,
    isLoading: false,
    isPending: false,
  })
);

vi.mock("@/systems/status", () => ({
  useDaemonStatus: () => daemonStatusMockState,
}));

vi.mock("@/systems/workspace", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/workspace")>();
  const workspaces = [
    {
      add_dirs: [],
      created_at: "2026-04-17T10:00:00Z",
      id: "ws_home",
      name: "Home",
      root_dir: "/Users/operator",
      updated_at: "2026-04-17T10:00:00Z",
    },
    {
      add_dirs: [],
      created_at: "2026-04-17T10:00:00Z",
      id: "ws_alpha",
      name: "Alpha",
      root_dir: "/workspace/alpha",
      updated_at: "2026-04-17T10:00:00Z",
    },
    {
      add_dirs: [],
      created_at: "2026-04-17T10:00:00Z",
      id: "ws_beta",
      name: "Beta",
      root_dir: "/workspace/beta",
      updated_at: "2026-04-17T10:00:00Z",
    },
  ];

  return {
    ...actual,
    useActiveWorkspace: () => {
      const activeWorkspace = workspaceMockState.hasHydrated
        ? (workspaces.find(workspace => workspace.id === workspaceMockState.selectedWorkspaceId) ??
          workspaces[0])
        : undefined;
      return {
        activeWorkspace,
        activeWorkspaceId: activeWorkspace?.id ?? null,
        error: workspaceMockState.error,
        hasHydrated: workspaceMockState.hasHydrated,
        isLoading: workspaceMockState.isLoading,
        isPending: workspaceMockState.isLoading,
        workspaces,
      };
    },
  };
});

import {
  approveTask,
  archiveTask,
  dismissTask,
  getTaskDashboard,
  getTaskInbox,
  listTasks,
  markTaskRead,
  rejectTask,
  retryTaskRun,
} from "@/systems/tasks/adapters/tasks-api";
import { getScheduler, getSchedulerBacklog } from "@/systems/scheduler/adapters/scheduler-api";
import {
  buildInboxFixture,
  buildTaskFixture,
  taskDashboardFixture,
  taskTriageStateFixture,
} from "@/systems/tasks/mocks/fixtures";
import type { TaskListFilter, TaskListPage } from "@/systems/tasks";
import { tasksKeys } from "@/systems/tasks";

import { useTasksPage, type UseTasksPageOptions } from "../use-tasks-page";

function createQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
}

function createWrapper(queryClient = createQueryClient()) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

const taskFixture = buildTaskFixture({
  active_run: null,
  id: "task_001",
  title: "Review PR",
  identifier: "TASK-1",
  status: "ready",
});
const failedTaskFixture = buildTaskFixture({
  active_run: null,
  id: "task_002",
  title: "Fix bug",
  status: "failed",
});
const taskFixtures = [
  taskFixture,
  failedTaskFixture,
  buildTaskFixture({
    active_run: null,
    id: "task_003",
    title: "Draft proposal",
    status: "draft",
    draft: true,
  }),
];

function taskListPage(tasks = taskFixtures, total = 21): TaskListPage {
  return {
    facets: {
      owners: [{ count: 12, owner: { kind: "agent_session", ref: "product" } }],
      statuses: [
        { count: 10, status: "ready" },
        { count: 8, status: "failed" },
        { count: 3, status: "draft" },
      ],
    },
    page: {
      has_more: total > tasks.length,
      limit: 50,
      next_cursor: total > tasks.length ? "tasks:3" : undefined,
      total,
    },
    tasks,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  daemonStatusMockState.data = { user_home_dir: "/Users/operator" };
  daemonStatusMockState.error = null;
  daemonStatusMockState.isLoading = false;
  daemonStatusMockState.isPending = false;
  workspaceMockState.error = null;
  workspaceMockState.hasHydrated = true;
  workspaceMockState.isLoading = false;
  workspaceMockState.selectedWorkspaceId = "ws_alpha";
  vi.mocked(listTasks).mockImplementation((filters: TaskListFilter = {}) => {
    if (filters.query === "Fix") {
      return Promise.resolve(taskListPage([failedTaskFixture], 1));
    }
    return Promise.resolve(taskListPage());
  });
  vi.mocked(getTaskDashboard).mockResolvedValue(taskDashboardFixture);
  vi.mocked(getTaskInbox).mockResolvedValue(buildInboxFixture({ unread_total: 6 }));
  vi.mocked(getScheduler).mockResolvedValue({
    paused: false,
    queued_run_count: 0,
    active_claim_count: 0,
    paused_task_count: 0,
    as_of: "2026-04-17T10:00:00Z",
  } as never);
  vi.mocked(getSchedulerBacklog).mockResolvedValue({ runs: [], total: 0 } as never);
  vi.mocked(retryTaskRun).mockResolvedValue({ id: "run_retry" } as never);
  vi.mocked(approveTask).mockResolvedValue({ id: "task_001" } as never);
  vi.mocked(rejectTask).mockResolvedValue({ id: "task_001" } as never);
  vi.mocked(markTaskRead).mockResolvedValue(taskTriageStateFixture);
  vi.mocked(archiveTask).mockResolvedValue({ ...taskTriageStateFixture, archived: true });
  vi.mocked(dismissTask).mockResolvedValue({ ...taskTriageStateFixture, dismissed: true });
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useTasksPage", () => {
  it("exposes exact list state, counts, and derived flags", async () => {
    const { result } = renderHook(() => useTasksPage(), { wrapper: createWrapper() });

    await waitFor(() => {
      expect(result.current.visibleTasks).toHaveLength(3);
    });

    expect(result.current.mode).toBe("list");
    expect(result.current.tasksCount).toBe(21);
    expect(result.current.effectiveSelectedTaskId).toBe("task_001");
    expect(result.current.statusCounts.ready).toBe(10);
    expect(result.current.statusCounts.failed).toBe(8);
    expect(result.current.statusCounts.draft).toBe(3);
    // 4-column kanban collapses draft + ready + pending + blocked into "pending"; fixture has 1 draft + 1 ready.
    expect(result.current.kanbanColumns.find(c => c.column.id === "pending")?.tasks).toHaveLength(
      2
    );
    const [listFilters] = vi.mocked(listTasks).mock.calls.at(-1) ?? [];
    expect(listFilters?.scope).toBe("workspace");
    expect(listFilters?.workspace).toBe("ws_alpha");
    expect(result.current.activeWorkspaceName).toBe("Alpha");
    expect(result.current.isEmpty).toBe(false);
    expect(result.current.inboxUnreadCount).toBe(6);
    expect(getTaskInbox).toHaveBeenCalledWith(
      expect.objectContaining({
        scope: "workspace",
        workspace: "ws_alpha",
        limit: 1,
      }),
      expect.any(AbortSignal)
    );
  });

  it("maps the home workspace to global task queries", async () => {
    workspaceMockState.selectedWorkspaceId = "ws_home";
    const { result } = renderHook(() => useTasksPage(), { wrapper: createWrapper() });

    await waitFor(() => {
      expect(result.current.visibleTasks).toHaveLength(3);
    });

    const [listFilters] = vi.mocked(listTasks).mock.calls.at(-1) ?? [];
    expect(listFilters?.scope).toBe("global");
    expect(listFilters?.workspace).toBeUndefined();
    expect(result.current.activeWorkspaceName).toBe("Home");
  });

  it("waits for daemon status before issuing any task-scoped read", async () => {
    daemonStatusMockState.data = undefined;
    daemonStatusMockState.isLoading = true;
    daemonStatusMockState.isPending = true;

    const { result, rerender } = renderHook(() => useTasksPage(), { wrapper: createWrapper() });

    expect(result.current.scopeLoading).toBe(true);
    expect(result.current.scopeError).toBeNull();
    expect(result.current.isEmpty).toBe(false);
    expect(listTasks).not.toHaveBeenCalled();
    expect(getTaskInbox).not.toHaveBeenCalled();
    expect(getTaskDashboard).not.toHaveBeenCalled();

    daemonStatusMockState.data = { user_home_dir: "/Users/operator" };
    daemonStatusMockState.isLoading = false;
    daemonStatusMockState.isPending = false;
    rerender();

    await waitFor(() => {
      expect(listTasks).toHaveBeenCalled();
      expect(getTaskInbox).toHaveBeenCalledWith(
        expect.objectContaining({ limit: 1, scope: "workspace", workspace: "ws_alpha" }),
        expect.any(AbortSignal)
      );
    });
  });

  it("surfaces daemon status failure without rendering an empty or zero catalog", () => {
    daemonStatusMockState.data = undefined;
    daemonStatusMockState.error = new Error("daemon status unavailable");

    const { result } = renderHook(() => useTasksPage(), { wrapper: createWrapper() });

    expect(result.current.scopeLoading).toBe(false);
    expect(result.current.scopeError).toEqual(new Error("daemon status unavailable"));
    expect(result.current.isEmpty).toBe(false);
    expect(listTasks).not.toHaveBeenCalled();
    expect(getTaskInbox).not.toHaveBeenCalled();
    expect(getTaskDashboard).not.toHaveBeenCalled();
  });

  it("keeps full catalogs mode-scoped while retaining the minimal inbox badge read", async () => {
    const { result } = renderHook(() => useTasksPage({ mode: "dashboard" }), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(getTaskDashboard).toHaveBeenCalled();
      expect(getSchedulerBacklog).toHaveBeenCalled();
    });

    expect(result.current.mode).toBe("dashboard");
    const [dashboardFilters] = vi.mocked(getTaskDashboard).mock.calls.at(-1) ?? [];
    expect(dashboardFilters?.scope).toBe("workspace");
    expect(dashboardFilters?.workspace).toBe("ws_alpha");
    const [backlogFilters] = vi.mocked(getSchedulerBacklog).mock.calls.at(-1) ?? [];
    expect(backlogFilters?.scope).toBe("workspace");
    expect(backlogFilters?.workspace).toBe("ws_alpha");
    expect(listTasks).not.toHaveBeenCalled();
    expect(getTaskInbox).toHaveBeenCalledTimes(1);
    expect(getTaskInbox).toHaveBeenCalledWith(
      expect.objectContaining({ limit: 1 }),
      expect.any(AbortSignal)
    );
  });

  it("swaps to inbox reads when the inbox tab is active", async () => {
    const { result, rerender } = renderHook(
      (options?: UseTasksPageOptions) => useTasksPage(options),
      { wrapper: createWrapper() }
    );

    rerender({ mode: "inbox" });

    await waitFor(() => {
      expect(getTaskInbox).toHaveBeenCalled();
    });

    expect(result.current.mode).toBe("inbox");
    const [inboxFilters] = vi.mocked(getTaskInbox).mock.calls.at(-1) ?? [];
    expect(inboxFilters?.scope).toBe("workspace");
    expect(inboxFilters?.workspace).toBe("ws_alpha");
  });

  it("maps inbox lane, status, priority, unread, and search into the backend query", async () => {
    const { result } = renderHook(() => useTasksPage({ mode: "inbox" }), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(getTaskInbox).toHaveBeenCalled();
    });

    act(() => {
      result.current.handleInboxLaneChange("approvals");
      result.current.handleInboxStatusChange("pending");
      result.current.handleInboxPriorityChange("high");
      result.current.handleInboxUnreadToggle(true);
      result.current.setInboxSearchQuery("rotate");
    });

    await waitFor(() => {
      expect(getTaskInbox).toHaveBeenLastCalledWith(
        expect.objectContaining({
          lane: "approvals",
          status: "pending",
          priority: "high",
          unread: true,
          query: "rotate",
        }),
        expect.any(AbortSignal)
      );
    });
  });

  it("maps the active workspace scope into the dashboard query", async () => {
    const { result } = renderHook(() => useTasksPage({ mode: "dashboard" }), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(getTaskDashboard).toHaveBeenCalled();
    });

    expect(result.current.mode).toBe("dashboard");
    expect(getTaskDashboard).toHaveBeenLastCalledWith(
      expect.objectContaining({ scope: "workspace", workspace: "ws_alpha" }),
      expect.any(AbortSignal)
    );
  });

  it("maps the home workspace into global dashboard and backlog queries", async () => {
    workspaceMockState.selectedWorkspaceId = "ws_home";
    renderHook(() => useTasksPage({ mode: "dashboard" }), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(getTaskDashboard).toHaveBeenCalled();
      expect(getSchedulerBacklog).toHaveBeenCalled();
    });

    const [dashboardFilters] = vi.mocked(getTaskDashboard).mock.calls.at(-1) ?? [];
    expect(dashboardFilters?.scope).toBe("global");
    expect(dashboardFilters?.workspace).toBeUndefined();
    const [backlogFilters] = vi.mocked(getSchedulerBacklog).mock.calls.at(-1) ?? [];
    expect(backlogFilters?.scope).toBe("global");
    expect(backlogFilters?.workspace).toBeUndefined();
  });

  it("updates search params without losing active workspace scope", async () => {
    const { result } = renderHook(() => useTasksPage(), { wrapper: createWrapper() });

    await waitFor(() => {
      expect(result.current.visibleTasks).toHaveLength(3);
    });

    act(() => {
      result.current.setSearchQuery("Fix");
    });

    await waitFor(() => {
      expect(result.current.visibleTasks.map(task => task.id)).toEqual(["task_002"]);
    });

    expect(result.current.effectiveSelectedTaskId).toBe("task_002");
    expect(listTasks).toHaveBeenCalledWith(
      expect.objectContaining({
        scope: "workspace",
        workspace: "ws_alpha",
        query: "Fix",
        sort: "recent",
        limit: 50,
      }),
      expect.any(AbortSignal)
    );
  });

  it("retries a failed task continuation without restarting the catalog", async () => {
    const { result } = renderHook(() => useTasksPage(), { wrapper: createWrapper() });
    await waitFor(() => expect(result.current.visibleTasks).toHaveLength(3));

    vi.mocked(listTasks).mockRejectedValueOnce(new Error("next page failed"));
    act(() => result.current.loadMoreTasks());
    await waitFor(() => expect(result.current.listError?.message).toBe("next page failed"));

    vi.mocked(listTasks).mockResolvedValueOnce(taskListPage([], 21));
    act(() => result.current.retryTasks());

    await waitFor(() => {
      expect(listTasks).toHaveBeenLastCalledWith(
        expect.objectContaining({ cursor: "tasks:3" }),
        expect.any(AbortSignal)
      );
    });
  });

  it("retries a failed task refetch from the first page", async () => {
    const queryClient = createQueryClient();
    const { result } = renderHook(() => useTasksPage(), {
      wrapper: createWrapper(queryClient),
    });
    await waitFor(() => expect(result.current.visibleTasks).toHaveLength(3));

    vi.mocked(listTasks).mockRejectedValueOnce(new Error("refetch failed"));
    await act(async () => {
      await queryClient.refetchQueries({ queryKey: tasksKeys.lists() });
    });
    await waitFor(() => expect(result.current.listError?.message).toBe("refetch failed"));

    vi.mocked(listTasks).mockResolvedValueOnce(taskListPage());
    act(() => result.current.retryTasks());

    await waitFor(() => {
      expect(listTasks).toHaveBeenLastCalledWith(
        expect.not.objectContaining({ cursor: expect.anything() }),
        expect.any(AbortSignal)
      );
    });
  });

  it("preserves owner kind when server facets share the same ref", async () => {
    vi.mocked(listTasks).mockResolvedValue({
      facets: {
        owners: [
          { count: 4, owner: { kind: "human", ref: "operator" } },
          { count: 7, owner: { kind: "agent_session", ref: "operator" } },
        ],
        statuses: [],
      },
      page: { has_more: false, limit: 50, total: 0 },
      tasks: [],
    });
    const { result } = renderHook(() => useTasksPage(), { wrapper: createWrapper() });
    await waitFor(() => expect(result.current.ownerOptions).toHaveLength(2));

    const agentOwner = result.current.ownerOptions[1];
    if (!agentOwner) throw new Error("expected an agent owner facet");
    act(() => {
      result.current.handleOwnerChange(agentOwner);
    });

    await waitFor(() => {
      expect(listTasks).toHaveBeenLastCalledWith(
        expect.objectContaining({ owner_kind: "agent_session", owner_ref: "operator" }),
        expect.any(AbortSignal)
      );
    });
    expect(result.current.ownerFilter).toEqual({ kind: "agent_session", ref: "operator" });
  });

  it("delegates triage actions and retries a failed run by run id", async () => {
    const { result } = renderHook(() => useTasksPage(), { wrapper: createWrapper() });

    await act(async () => {
      await result.current.handleApproveTask("task_001");
      await result.current.handleRejectTask("task_001");
      await result.current.handleArchiveTask("task_001");
      await result.current.handleDismissTask("task_001");
      await result.current.handleMarkTaskRead("task_001");
      await result.current.handleRetryRun("run_001");
    });

    expect(approveTask).toHaveBeenCalledWith("task_001");
    expect(rejectTask).toHaveBeenCalledWith("task_001");
    expect(archiveTask).toHaveBeenCalledWith("task_001");
    expect(dismissTask).toHaveBeenCalledWith("task_001");
    expect(markTaskRead).toHaveBeenCalledWith("task_001");
    expect(retryTaskRun).toHaveBeenCalledWith("run_001", {});
  });

  it("blocks duplicate retry submission while preserving the pending run identity", async () => {
    let resolveRetry: ((value: unknown) => void) | undefined;
    vi.mocked(retryTaskRun).mockImplementationOnce(
      () => new Promise(resolve => (resolveRetry = resolve)) as never
    );
    const { result } = renderHook(() => useTasksPage(), { wrapper: createWrapper() });

    let first: Promise<void> | undefined;
    await act(async () => {
      first = result.current.handleRetryRun("run_001");
      void result.current.handleRetryRun("run_001");
      await Promise.resolve();
    });

    expect(retryTaskRun).toHaveBeenCalledTimes(1);
    expect(result.current.pendingRetryIds.has("run_001")).toBe(true);

    await act(async () => {
      resolveRetry?.({ id: "run_retry" });
      await first;
    });
    expect(result.current.pendingRetryIds.has("run_001")).toBe(false);
  });
});
