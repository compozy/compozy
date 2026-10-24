import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, useState, type ReactNode } from "react";
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
  options: undefined as unknown,
  error: null as Error | null,
  hasHydrated: true,
  isLoading: false,
  pending: false,
  breadthMode: "focused" as "focused" | "all",
  selectedWorkspaceId: "ws_alpha" as string | null,
}));

const workspaceFixtures = vi.hoisted(() => [
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
]);

vi.mock("@/systems/workspace/hooks/use-active-workspace", async () => {
  const { resolveActiveWorkspace } = await import("@/systems/workspace/lib/active-workspace");
  return {
    // The real resolver over the fixture catalog: a missing remembered project
    // resolves to User, and `pending` claims nothing it cannot prove.
    useActiveWorkspace: (options: unknown) => {
      workspaceMockState.options = options;
      return resolveFixture();
    },
    // Outside a window frame the window's data scope is the menubar destination.
    useDataScope: () => resolveFixture().dataScope,
  };
  function resolveFixture() {
    const selected = workspaceMockState.selectedWorkspaceId;
    const resolution = resolveActiveWorkspace({
      workspaces: workspaceFixtures,
      breadthMode: workspaceMockState.breadthMode,
      focus: selected ? { kind: "project", workspaceId: selected } : { kind: "user" },
      pending: workspaceMockState.pending,
    });
    return {
      ...resolution,
      error: workspaceMockState.error,
      hasHydrated: workspaceMockState.hasHydrated,
      isLoading: workspaceMockState.isLoading,
      isPending: workspaceMockState.isLoading,
      workspaces: resolution.projectWorkspaces,
    };
  }
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
  schedulerBacklogFixture,
  schedulerPausedStatusFixture,
} from "@/systems/scheduler/mocks/fixtures";
import {
  buildInboxFixture,
  buildTaskFixture,
  taskDashboardFixture,
  taskTriageStateFixture,
} from "@/systems/tasks/mocks/fixtures";

import { useTasksPage } from "../use-tasks-page";
import { tasksPageActionsLogic } from "../tasks-page-actions-store";
import {
  type TaskListFilter,
  type TaskListPage,
  tasksKeys,
  type TasksRouteSearch,
  type TaskViewMode,
  validateTasksSearch,
} from "@/systems/tasks";

function createQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
}

function createWrapper(queryClient = createQueryClient()) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

function useControlledTasksPage(initialSearch: TasksRouteSearch = {}) {
  const [search, setSearch] = useState(initialSearch);
  return useTasksPage({
    search,
    onSearchChange: update => setSearch(current => update(current)),
  });
}

/**
 * The same hook, with the surface owned from outside the way the route owns it.
 * Switching surfaces is a navigation, not a hook call — so exercising the reset
 * means rerendering one hook across modes, never mounting a fresh one.
 */
function useRouteControlledTasksPage(mode: TaskViewMode) {
  const [search, setSearch] = useState<TasksRouteSearch>({});
  // `list` is the absence of a mode in the route, exactly as the router spells it.
  const modeSearch: TasksRouteSearch = mode === "list" ? {} : { mode };
  return useTasksPage({
    search: { ...search, ...modeSearch },
    onSearchChange: update => setSearch(current => update({ ...current, ...modeSearch })),
  });
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

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>(settle => {
    resolve = settle;
  });
  return { promise, resolve };
}

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
  workspaceMockState.error = null;
  workspaceMockState.hasHydrated = true;
  workspaceMockState.isLoading = false;
  workspaceMockState.options = undefined;
  workspaceMockState.pending = false;
  workspaceMockState.breadthMode = "focused";
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
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("useTasksPage", () => {
  it("Should refresh external scheduler changes only while the dashboard is active", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const queryClient = createQueryClient();
    const { result, rerender, unmount } = renderHook(
      ({ liveDataEnabled }) => useTasksPage({ search: { mode: "dashboard" }, liveDataEnabled }),
      { initialProps: { liveDataEnabled: true }, wrapper: createWrapper(queryClient) }
    );
    await waitFor(() => expect(result.current.schedulerStatus?.paused).toBe(false));
    expect(result.current.schedulerBacklog?.total).toBe(0);

    vi.mocked(getScheduler).mockResolvedValue(schedulerPausedStatusFixture);
    vi.mocked(getSchedulerBacklog).mockResolvedValue(schedulerBacklogFixture);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(30_001);
    });
    expect(result.current.schedulerStatus?.paused).toBe(true);
    expect(result.current.schedulerBacklog?.runs).toEqual(schedulerBacklogFixture.runs);

    rerender({ liveDataEnabled: false });
    const statusReads = vi.mocked(getScheduler).mock.calls.length;
    const backlogReads = vi.mocked(getSchedulerBacklog).mock.calls.length;
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000);
    });
    expect(getScheduler).toHaveBeenCalledTimes(statusReads);
    expect(getSchedulerBacklog).toHaveBeenCalledTimes(backlogReads);
    unmount();
    queryClient.clear();
  });

  it("disables scope-source observers while its window is inactive", () => {
    const { result } = renderHook(() => useTasksPage({ liveDataEnabled: false }), {
      wrapper: createWrapper(),
    });

    expect(workspaceMockState.options).toEqual({ enabled: false });
    expect(result.current.activeWorkspaceName).toBe("Alpha");
    expect(listTasks).not.toHaveBeenCalled();
    expect(getTaskInbox).not.toHaveBeenCalled();
    expect(getTaskDashboard).not.toHaveBeenCalled();
    // A suspended read has not answered yet: it is loading, never an empty project.
    expect(result.current.isEmpty).toBe(false);
    expect(result.current.listLoading).toBe(true);
  });

  it("ignores a stale row completion token", () => {
    const store = tasksPageActionsLogic.createStore();
    const request = {
      execute: () => new Promise<never>(() => undefined),
      failureMessage: "Failed to approve task",
      id: "task_001",
      kind: "approve" as const,
    };
    let snapshot = store.getInitialSnapshot();
    [snapshot] = store.transition(snapshot, {
      type: "actionRequested",
      ...request,
    });
    [snapshot] = store.transition(snapshot, {
      type: "actionSucceeded",
      ...request,
      token: 0,
    });

    expect(snapshot.context.pending.approve.task_001).toBe(1);
  });

  it("keeps independently validated list and inbox filters in route search", () => {
    expect(
      validateTasksSearch({
        status: "failed",
        inboxUnread: true,
        inboxStatus: "invalid",
      })
    ).toEqual({ status: "failed", inboxUnread: true });
  });

  it("keeps the reveal filter off by default and off the wire", async () => {
    const { result } = renderHook(() => useControlledTasksPage(), { wrapper: createWrapper() });

    await waitFor(() => {
      expect(result.current.visibleTasks).toHaveLength(3);
    });

    expect(result.current.recordsFilter).toBe("work");
    const [listFilters] = vi.mocked(listTasks).mock.calls.at(-1) ?? [];
    // The daemon owns the default: a calm read sends no include flag at all.
    expect(listFilters?.include_loop).toBeUndefined();
    expect(result.current.isEmpty).toBe(false);
  });

  it("sends include_loop only while the reveal is on, and counts it as a list filter", async () => {
    const { result } = renderHook(() => useControlledTasksPage(), { wrapper: createWrapper() });

    await waitFor(() => {
      expect(result.current.visibleTasks).toHaveLength(3);
    });

    act(() => {
      result.current.handleRecordsFilterChange("loop");
    });

    await waitFor(() => {
      expect(result.current.recordsFilter).toBe("loop");
    });
    await waitFor(() => {
      const [revealed] = vi.mocked(listTasks).mock.calls.at(-1) ?? [];
      expect(revealed?.include_loop).toBe(true);
    });

    act(() => {
      result.current.handleRecordsFilterChange("work");
    });
    await waitFor(() => {
      expect(result.current.recordsFilter).toBe("work");
    });

    // Reveal-on and reveal-off are distinct cache keys, so returning to the calm
    // read serves the cached page rather than refetching. What must hold across
    // every request the surface ever makes is that the calm default is the
    // absence of the flag — an explicit `false` would be a second default.
    expect(
      vi.mocked(listTasks).mock.calls.every(([filters]) => filters?.include_loop !== false)
    ).toBe(true);
  });

  it("resets the reveal when the surface changes so the board stays work-items only", async () => {
    const { result, rerender } = renderHook(({ mode }) => useRouteControlledTasksPage(mode), {
      initialProps: { mode: "list" as TaskViewMode },
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.visibleTasks).toHaveLength(3);
    });

    act(() => {
      result.current.handleRecordsFilterChange("loop");
    });
    await waitFor(() => {
      expect(result.current.recordsFilter).toBe("loop");
    });

    act(() => {
      result.current.handleSortChange("priority");
    });
    // A filter change inside the list keeps the revealed context.
    await waitFor(() => {
      expect(result.current.recordsFilter).toBe("loop");
    });

    // The surface changes under the same hook — the only way the reset branch
    // runs at all. A freshly mounted kanban hook starts calm by construction and
    // would pass this even with the reset deleted.
    rerender({ mode: "kanban" as TaskViewMode });
    await waitFor(() => {
      expect(result.current.recordsFilter).toBe("work");
    });
    await waitFor(() => {
      const [boardFilters] = vi.mocked(listTasks).mock.calls.at(-1) ?? [];
      expect(boardFilters?.include_loop).toBeUndefined();
    });

    // And coming back is a fresh context too: a reveal must never survive a
    // round trip through another surface.
    rerender({ mode: "list" as TaskViewMode });
    await waitFor(() => {
      expect(result.current.recordsFilter).toBe("work");
    });
  });

  it("exposes exact Kanban state, counts, and derived columns", async () => {
    const { result } = renderHook(() => useControlledTasksPage({ mode: "kanban" }), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.visibleTasks).toHaveLength(3);
    });

    expect(result.current.mode).toBe("kanban");
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

  it("maps User focus to unbound (scope=global) task queries, never every task", async () => {
    workspaceMockState.selectedWorkspaceId = null;
    const { result } = renderHook(() => useTasksPage(), { wrapper: createWrapper() });

    await waitFor(() => {
      expect(result.current.visibleTasks).toHaveLength(3);
    });

    const [listFilters] = vi.mocked(listTasks).mock.calls.at(-1) ?? [];
    expect(listFilters?.scope).toBe("global");
    expect(listFilters?.workspace).toBeUndefined();
  });

  // W-17: All projects reads the task aggregate (every project plus User), never `scope=global`.
  it("maps All projects to the all_workspaces task aggregate", async () => {
    workspaceMockState.breadthMode = "all";
    const { result } = renderHook(() => useTasksPage(), { wrapper: createWrapper() });

    await waitFor(() => {
      expect(result.current.visibleTasks).toHaveLength(3);
    });

    const [listFilters] = vi.mocked(listTasks).mock.calls.at(-1) ?? [];
    expect(listFilters?.all_workspaces).toBe(true);
    expect(listFilters?.scope).toBeUndefined();
    expect(listFilters?.workspace).toBeUndefined();
    expect(listFilters?.workspace_ids).toBeUndefined();
    expect(result.current.activeWorkspaceName).toBeNull();
    expect(result.current.projectFilter.visible).toBe(true);
    expect(result.current.projectBadgeOf?.({ workspace_id: "ws_beta" })).toMatchObject({
      kind: "project",
      name: "Beta",
    });
    expect(result.current.projectBadgeOf?.({})).toMatchObject({ kind: "user", name: "User" });
    // The inbox has no aggregate selector: omission is every task.
    expect(getTaskInbox).toHaveBeenCalledWith(
      expect.objectContaining({ scope: undefined, workspace: undefined, limit: 1 }),
      expect.any(AbortSignal)
    );
  });

  // W-23: the Project filter narrows the aggregate server-side, and exists only under All projects.
  it("narrows the aggregate by the Project filter and hides it outside All projects", async () => {
    workspaceMockState.breadthMode = "all";
    const { result, rerender } = renderHook(() => useTasksPage(), { wrapper: createWrapper() });
    await waitFor(() => {
      expect(result.current.visibleTasks).toHaveLength(3);
    });

    act(() => {
      result.current.projectFilter.setValues(["ws_alpha", "@user"]);
    });

    await waitFor(() => {
      const [listFilters] = vi.mocked(listTasks).mock.calls.at(-1) ?? [];
      expect(listFilters).toMatchObject({
        all_workspaces: true,
        workspace_ids: ["@user", "ws_alpha"],
      });
    });
    expect(result.current.hasListFilters).toBe(true);

    workspaceMockState.breadthMode = "focused";
    rerender();

    await waitFor(() => {
      const [listFilters] = vi.mocked(listTasks).mock.calls.at(-1) ?? [];
      expect(listFilters).toMatchObject({ scope: "workspace", workspace: "ws_alpha" });
      expect(listFilters?.workspace_ids).toBeUndefined();
    });
    expect(result.current.projectFilter.visible).toBe(false);
    expect(result.current.projectFilter.values).toEqual([]);
    expect(result.current.projectBadgeOf).toBeUndefined();

    // Back under All projects the filter starts empty: it is transient per mode.
    workspaceMockState.breadthMode = "all";
    rerender();
    expect(result.current.projectFilter.values).toEqual([]);
  });

  it("withholds task-scoped reads until scope resolution settles", async () => {
    workspaceMockState.pending = true;

    const { result, rerender } = renderHook(() => useTasksPage(), { wrapper: createWrapper() });

    expect(result.current.scopeLoading).toBe(true);
    expect(result.current.scopeError).toBeNull();
    expect(result.current.isEmpty).toBe(false);
    expect(listTasks).not.toHaveBeenCalled();
    expect(getTaskInbox).not.toHaveBeenCalled();
    expect(getTaskDashboard).not.toHaveBeenCalled();

    workspaceMockState.pending = false;
    rerender();

    await waitFor(() => {
      expect(listTasks).toHaveBeenCalled();
      expect(getTaskInbox).toHaveBeenCalledWith(
        expect.objectContaining({ limit: 1, scope: "workspace", workspace: "ws_alpha" }),
        expect.any(AbortSignal)
      );
    });
  });

  it("surfaces scope-source failure without rendering an empty or zero catalog", () => {
    workspaceMockState.pending = true;
    workspaceMockState.error = new Error("daemon status unavailable");

    const { result } = renderHook(() => useTasksPage(), { wrapper: createWrapper() });

    expect(result.current.scopeLoading).toBe(false);
    expect(result.current.scopeError).toEqual(new Error("daemon status unavailable"));
    expect(result.current.isEmpty).toBe(false);
    expect(listTasks).not.toHaveBeenCalled();
    expect(getTaskInbox).not.toHaveBeenCalled();
    expect(getTaskDashboard).not.toHaveBeenCalled();
  });

  it("keeps full catalogs mode-scoped while retaining the minimal inbox badge read", async () => {
    const { result } = renderHook(() => useTasksPage({ search: { mode: "dashboard" } }), {
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
      ({ search }: { search: TasksRouteSearch }) => useTasksPage({ search }),
      { initialProps: { search: {} }, wrapper: createWrapper() }
    );

    rerender({ search: { mode: "inbox" } });

    await waitFor(() => {
      expect(getTaskInbox).toHaveBeenCalled();
    });

    expect(result.current.mode).toBe("inbox");
    const [inboxFilters] = vi.mocked(getTaskInbox).mock.calls.at(-1) ?? [];
    expect(inboxFilters?.scope).toBe("workspace");
    expect(inboxFilters?.workspace).toBe("ws_alpha");
  });

  it("maps inbox lane, status, priority, unread, and search into the backend query", async () => {
    const { result } = renderHook(() => useControlledTasksPage({ mode: "inbox" }), {
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

  it("maps All projects into the aggregate dashboard and backlog queries", async () => {
    workspaceMockState.breadthMode = "all";
    renderHook(() => useTasksPage({ search: { mode: "dashboard" } }), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(getTaskDashboard).toHaveBeenCalled();
      expect(getSchedulerBacklog).toHaveBeenCalled();
    });

    const [dashboardFilters] = vi.mocked(getTaskDashboard).mock.calls.at(-1) ?? [];
    expect(dashboardFilters?.all_workspaces).toBe(true);
    expect(dashboardFilters?.scope).toBeUndefined();
    expect(dashboardFilters?.workspace).toBeUndefined();
    const [backlogFilters] = vi.mocked(getSchedulerBacklog).mock.calls.at(-1) ?? [];
    expect(backlogFilters?.scope).toBe("all");
    expect(backlogFilters?.workspace).toBeUndefined();
  });

  it("updates search params without losing active workspace scope", async () => {
    const { result } = renderHook(() => useControlledTasksPage(), {
      wrapper: createWrapper(),
    });

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

  it("Should keep the rows on screen while a refined list loads, without a continuation", async () => {
    const { result } = renderHook(() => useControlledTasksPage(), { wrapper: createWrapper() });
    await waitFor(() => expect(result.current.visibleTasks).toHaveLength(3));
    expect(result.current.hasMoreTasks).toBe(true);

    const refined = deferred<TaskListPage>();
    vi.mocked(listTasks).mockImplementationOnce(() => refined.promise);
    act(() => result.current.handleStatusChange("failed"));
    await waitFor(() => expect(listTasks).toHaveBeenCalledTimes(2));

    // The refined page is still out: the operator keeps the rows they were reading, not a skeleton.
    expect(result.current.listLoading).toBe(false);
    expect(result.current.visibleTasks).toHaveLength(3);
    // Those rows no longer describe the list being read, so they offer no continuation.
    expect(result.current.hasMoreTasks).toBe(false);
    act(() => result.current.loadMoreTasks());
    expect(listTasks).toHaveBeenCalledTimes(2);

    await act(async () => refined.resolve(taskListPage([failedTaskFixture], 1)));
    await waitFor(() =>
      expect(result.current.visibleTasks.map(task => task.id)).toEqual(["task_002"])
    );
    expect(result.current.hasMoreTasks).toBe(false);
  });

  it("Should not call the project empty while clearing filters bridges an empty result", async () => {
    vi.mocked(listTasks).mockResolvedValueOnce(taskListPage([], 0));
    const { result } = renderHook(() => useControlledTasksPage({ status: "failed" }), {
      wrapper: createWrapper(),
    });
    await waitFor(() => expect(result.current.listLoading).toBe(false));
    expect(result.current.visibleTasks).toEqual([]);
    expect(result.current.isEmpty).toBe(false);

    const unfiltered = deferred<TaskListPage>();
    vi.mocked(listTasks).mockImplementationOnce(() => unfiltered.promise);
    act(() => result.current.clearListFilters());
    await waitFor(() => expect(listTasks).toHaveBeenCalledTimes(2));

    // The bridged result has zero rows, and no filters are active any more: still not an empty
    // project, because the unfiltered read has not answered.
    expect(result.current.listLoading).toBe(false);
    expect(result.current.isEmpty).toBe(false);

    await act(async () => unfiltered.resolve(taskListPage()));
    await waitFor(() => expect(result.current.visibleTasks).toHaveLength(3));
    expect(result.current.isEmpty).toBe(false);
  });

  it("Should not carry another project's rows into a project switch", async () => {
    const { result, rerender } = renderHook(() => useControlledTasksPage(), {
      wrapper: createWrapper(),
    });
    await waitFor(() => expect(result.current.visibleTasks).toHaveLength(3));

    const beta = deferred<TaskListPage>();
    vi.mocked(listTasks).mockImplementationOnce(() => beta.promise);
    workspaceMockState.selectedWorkspaceId = "ws_beta";
    rerender();
    await waitFor(() => expect(listTasks).toHaveBeenCalledTimes(2));

    // A different project is a different list: nothing of Alpha's stands in for it.
    expect(result.current.visibleTasks).toEqual([]);
    expect(result.current.listLoading).toBe(true);

    await act(async () => beta.resolve(taskListPage([failedTaskFixture], 1)));
    await waitFor(() => expect(result.current.visibleTasks).toHaveLength(1));
    expect(result.current.listLoading).toBe(false);
  });

  it("Should keep the inbox on screen across a lane change, without a continuation", async () => {
    vi.mocked(getTaskInbox).mockResolvedValue(buildInboxFixture({ unread_total: 6 }));
    const { result } = renderHook(() => useControlledTasksPage({ mode: "inbox" }), {
      wrapper: createWrapper(),
    });
    await waitFor(() => expect(result.current.inbox?.unread_total).toBe(6));

    const lane = deferred<ReturnType<typeof buildInboxFixture>>();
    vi.mocked(getTaskInbox).mockImplementationOnce(() => lane.promise);
    const reads = vi.mocked(getTaskInbox).mock.calls.length;
    act(() => result.current.handleInboxLaneChange("approvals"));
    await waitFor(() => expect(vi.mocked(getTaskInbox).mock.calls.length).toBe(reads + 1));

    // The lane's page is still out: the inbox the operator was reading stays, not a skeleton.
    expect(result.current.inboxLoading).toBe(false);
    expect(result.current.inbox?.unread_total).toBe(6);
    expect(result.current.hasMoreInbox).toBe(false);

    await act(async () => lane.resolve(buildInboxFixture({ unread_total: 1 })));
    await waitFor(() => expect(result.current.inbox?.unread_total).toBe(1));
  });

  it("Should keep the search draft responsive and publish only the latest debounced query", async () => {
    const onSearchChange = vi.fn();
    const { result } = renderHook(() => useTasksPage({ search: {}, onSearchChange }), {
      wrapper: createWrapper(),
    });

    act(() => {
      result.current.setSearchQuery("r");
      result.current.setSearchQuery("re");
      result.current.setSearchQuery("review");
    });

    expect(result.current.searchQuery).toBe("review");
    expect(onSearchChange).not.toHaveBeenCalled();
    await waitFor(() => expect(onSearchChange).toHaveBeenCalledTimes(1));
    const update = onSearchChange.mock.calls[0]?.[0] as (
      current: TasksRouteSearch
    ) => TasksRouteSearch;
    expect(update({ status: "ready" })).toEqual({ query: "review", status: "ready" });
  });

  it("retries a failed task continuation without restarting the catalog", async () => {
    const { result } = renderHook(() => useControlledTasksPage(), {
      wrapper: createWrapper(),
    });
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
    const { result } = renderHook(() => useControlledTasksPage(), {
      wrapper: createWrapper(),
    });
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

    act(() => {
      result.current.handleApproveTask("task_001");
      result.current.handleRejectTask("task_001");
      result.current.handleArchiveTask("task_001");
      result.current.handleDismissTask("task_001");
      result.current.handleMarkTaskRead("task_001");
      result.current.handleRetryRun("run_001");
    });

    await waitFor(() => {
      expect(approveTask).toHaveBeenCalledWith("task_001");
      expect(rejectTask).toHaveBeenCalledWith("task_001");
      expect(archiveTask).toHaveBeenCalledWith("task_001");
      expect(dismissTask).toHaveBeenCalledWith("task_001");
      expect(markTaskRead).toHaveBeenCalledWith("task_001");
      expect(retryTaskRun).toHaveBeenCalledWith("run_001", {});
    });
  });

  it("blocks duplicate retry submission while preserving the pending run identity", async () => {
    let resolveRetry: ((value: unknown) => void) | undefined;
    vi.mocked(retryTaskRun).mockImplementationOnce(
      () => new Promise(resolve => (resolveRetry = resolve)) as never
    );
    const { result } = renderHook(() => useTasksPage(), { wrapper: createWrapper() });

    act(() => {
      result.current.handleRetryRun("run_001");
      result.current.handleRetryRun("run_001");
    });

    await waitFor(() => expect(retryTaskRun).toHaveBeenCalledTimes(1));
    expect(result.current.pendingRetryIds.has("run_001")).toBe(true);

    await act(async () => {
      resolveRetry?.({ id: "run_retry" });
      await Promise.resolve();
    });
    await waitFor(() => expect(result.current.pendingRetryIds.has("run_001")).toBe(false));
  });
});
