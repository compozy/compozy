import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  useTask,
  useTaskDashboard,
  useTaskInbox,
  useTaskRunDetail,
  useTaskRuns,
  useTaskTimeline,
  useTaskTree,
  useTasks,
} from "@/systems/tasks";

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
  forceFailTaskRun: vi.fn(),
  forceReleaseTaskRun: vi.fn(),
  retryTaskRun: vi.fn(),
}));

import {
  getTask,
  getTaskDashboard,
  getTaskInbox,
  getTaskRun,
  getTaskTimeline,
  getTaskTree,
  listTaskRuns,
  listTasks,
} from "@/systems/tasks/adapters/tasks-api";
import { buildInboxItemFixture, buildTaskFixture } from "@/systems/tasks/mocks/fixtures";

function createWrapper(
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

const taskFixture = buildTaskFixture({ id: "task_001", title: "Review", status: "ready" });
const runFixture = { id: "run_001", task_id: "task_001", status: "running" };
const timelineFixture = { event_id: "evt_001", sequence: 1 };
const treeFixture = { root: { depth: 0, task: { id: "task_001" } } };
const runDetailFixture = { run: runFixture, task: { id: "task_001" }, summary: {} };
const dashboardFixture = { totals: { tasks_total: 0 } };
const inboxFixture = {
  archived_total: 0,
  facets: { priorities: [], statuses: [] },
  groups: [],
  page: { has_more: false, limit: 1, total: 0 },
  unread_total: 0,
};

describe("tasks read hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("loads tasks list with filters", async () => {
    vi.mocked(listTasks).mockResolvedValue({
      facets: { owners: [], statuses: [{ count: 3, status: "ready" }] },
      page: { has_more: false, limit: 50, total: 3 },
      tasks: [taskFixture],
    });

    const { result } = renderHook(() => useTasks({ scope: "workspace", workspace: "ws_alpha" }), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.data).toHaveLength(1);
    });

    expect(result.current.total).toBe(3);

    expect(listTasks).toHaveBeenCalledWith(
      { scope: "workspace", workspace: "ws_alpha" },
      expect.any(AbortSignal)
    );
  });

  it("appends task pages in backend order while preserving the exact first-page total", async () => {
    vi.mocked(listTasks)
      .mockResolvedValueOnce({
        facets: { owners: [], statuses: [] },
        page: { has_more: true, limit: 1, next_cursor: "next", total: 2 },
        tasks: [taskFixture],
      })
      .mockResolvedValueOnce({
        facets: { owners: [], statuses: [] },
        page: { has_more: false, limit: 1, total: 2 },
        tasks: [{ ...taskFixture, id: "task_002", title: "Second" }],
      });

    const { result } = renderHook(() => useTasks({ limit: 1 }), { wrapper: createWrapper() });
    await waitFor(() => expect(result.current.data).toHaveLength(1));
    await result.current.fetchNextPage();
    await waitFor(() => {
      expect(result.current.data?.map(task => task.id)).toEqual(["task_001", "task_002"]);
    });
    expect(result.current.total).toBe(2);
    expect(listTasks).toHaveBeenNthCalledWith(
      2,
      { cursor: "next", limit: 1 },
      expect.any(AbortSignal)
    );
  });

  it("respects explicit disable flag for tasks list", () => {
    renderHook(() => useTasks({}, { enabled: false }), { wrapper: createWrapper() });
    expect(listTasks).not.toHaveBeenCalled();
  });

  it("loads task detail and guards empty ids", async () => {
    vi.mocked(getTask).mockResolvedValue({ task: taskFixture, summary: {} } as never);

    const { result } = renderHook(() => useTask("task_001"), { wrapper: createWrapper() });

    await waitFor(() => {
      expect(result.current.data?.task.id).toBe("task_001");
    });

    renderHook(() => useTask(""), { wrapper: createWrapper() });
    expect(getTask).toHaveBeenCalledTimes(1);
  });

  it("loads task runs with filters and respects enabled flag", async () => {
    vi.mocked(listTaskRuns).mockResolvedValue([runFixture] as never);

    const { result } = renderHook(() => useTaskRuns("task_001", { status: "running", limit: 5 }), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.data).toHaveLength(1);
    });

    expect(listTaskRuns).toHaveBeenCalledWith(
      "task_001",
      { status: "running", limit: 5 },
      expect.any(AbortSignal)
    );

    renderHook(() => useTaskRuns("task_001", {}, { enabled: false }), {
      wrapper: createWrapper(),
    });
    expect(listTaskRuns).toHaveBeenCalledTimes(1);
  });

  it("loads live reads (timeline, tree, run detail)", async () => {
    vi.mocked(getTaskTimeline).mockResolvedValue([timelineFixture] as never);
    vi.mocked(getTaskTree).mockResolvedValue(treeFixture as never);
    vi.mocked(getTaskRun).mockResolvedValue(runDetailFixture as never);

    const timeline = renderHook(() => useTaskTimeline("task_001", { limit: 20 }), {
      wrapper: createWrapper(),
    });
    const tree = renderHook(() => useTaskTree("task_001"), { wrapper: createWrapper() });
    const runDetail = renderHook(() => useTaskRunDetail("run_001"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(timeline.result.current.data).toHaveLength(1);
      expect(tree.result.current.data?.root.task.id).toBe("task_001");
      expect(runDetail.result.current.data?.run.id).toBe("run_001");
    });

    expect(getTaskTimeline).toHaveBeenCalledWith(
      "task_001",
      { limit: 20 },
      expect.any(AbortSignal)
    );
    expect(getTaskTree).toHaveBeenCalledWith("task_001", expect.any(AbortSignal));
    expect(getTaskRun).toHaveBeenCalledWith("run_001", expect.any(AbortSignal));
  });

  it("loads dashboard and inbox aggregates", async () => {
    vi.mocked(getTaskDashboard).mockResolvedValue(dashboardFixture as never);
    vi.mocked(getTaskInbox).mockResolvedValue(inboxFixture);

    const dashboard = renderHook(() => useTaskDashboard({ scope: "workspace" }), {
      wrapper: createWrapper(),
    });
    const inbox = renderHook(() => useTaskInbox({ lane: "approvals" }), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(dashboard.result.current.data?.totals.tasks_total).toBe(0);
      expect(inbox.result.current.data?.page.total).toBe(0);
    });

    expect(getTaskDashboard).toHaveBeenCalledWith({ scope: "workspace" }, expect.any(AbortSignal));
    expect(getTaskInbox).toHaveBeenCalledWith({ lane: "approvals" }, expect.any(AbortSignal));
  });

  it("refetches the canonical inbox when an existing query becomes active again", async () => {
    const refreshedItem = buildInboxItemFixture({
      task: { id: "task_002", title: "Created outside the mounted inbox" },
    });
    vi.mocked(getTaskInbox)
      .mockResolvedValueOnce(inboxFixture)
      .mockResolvedValueOnce({
        ...inboxFixture,
        groups: [
          {
            count: 1,
            items: [refreshedItem],
            lane: "approvals",
            unread_count: 1,
          },
        ],
        page: { has_more: false, limit: 1, total: 1 },
        unread_total: 1,
      });
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const wrapper = createWrapper(queryClient);

    const initial = renderHook(({ enabled }) => useTaskInbox({ lane: "approvals" }, { enabled }), {
      initialProps: { enabled: true },
      wrapper,
    });
    await waitFor(() => expect(initial.result.current.data?.page.total).toBe(0));
    initial.rerender({ enabled: false });
    initial.rerender({ enabled: true });

    await waitFor(() => expect(initial.result.current.data?.page.total).toBe(1));

    const refreshedData = initial.result.current.data;
    expect(refreshedData).toBeDefined();
    expect(refreshedData?.groups.at(0)?.items?.at(0)?.task?.id).toBe("task_002");
    expect(getTaskInbox).toHaveBeenCalledTimes(2);
  });

  it("merges inbox pages by lane without summing exact group or page totals", async () => {
    const firstItem = buildInboxItemFixture({
      task: { id: "task_001", title: "First" },
    });
    const secondItem = buildInboxItemFixture({
      task: { id: "task_002", title: "Second" },
    });
    vi.mocked(getTaskInbox)
      .mockResolvedValueOnce({
        archived_total: 7,
        facets: {
          priorities: [{ count: 12, priority: "high" }],
          statuses: [{ count: 12, status: "ready" }],
        },
        groups: [
          {
            count: 12,
            items: [firstItem],
            lane: "my_work",
            unread_count: 9,
          },
        ],
        page: { has_more: true, limit: 1, next_cursor: "inbox:1", total: 20 },
        unread_total: 15,
      })
      .mockResolvedValueOnce({
        archived_total: 7,
        facets: {
          priorities: [{ count: 12, priority: "high" }],
          statuses: [{ count: 12, status: "ready" }],
        },
        groups: [
          {
            count: 12,
            items: [firstItem, secondItem],
            lane: "my_work",
            unread_count: 9,
          },
        ],
        page: { has_more: false, limit: 1, total: 20 },
        unread_total: 15,
      });

    const { result } = renderHook(() => useTaskInbox({ limit: 1 }), {
      wrapper: createWrapper(),
    });
    await waitFor(() => expect(result.current.data?.groups[0]?.items).toHaveLength(1));
    await result.current.fetchNextPage();
    await waitFor(() => expect(result.current.data?.groups[0]?.items).toHaveLength(2));

    expect(result.current.data?.groups[0]?.count).toBe(12);
    expect(result.current.data?.groups[0]?.unread_count).toBe(9);
    expect(result.current.data?.page.total).toBe(20);
    expect(result.current.data?.unread_total).toBe(15);
    const items = result.current.data?.groups[0]?.items ?? [];
    expect(items.map(item => item.task.id)).toEqual(["task_001", "task_002"]);
    expect(getTaskInbox).toHaveBeenNthCalledWith(
      2,
      { cursor: "inbox:1", limit: 1 },
      expect.any(AbortSignal)
    );
  });
});
