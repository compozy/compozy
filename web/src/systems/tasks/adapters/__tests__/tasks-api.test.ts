import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockJsonResponse } from "@/test/fetch-test-utils";

function mockJsonSequence(body: unknown, status = 200): void {
  vi.mocked(globalThis.fetch).mockImplementation(() =>
    Promise.resolve(
      new Response(JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      })
    )
  );
}
import {
  TasksApiError,
  addTaskDependency,
  approveTask,
  archiveTask,
  attachTaskRunSession,
  cancelTask,
  cancelTaskRun,
  completeTaskRun,
  createChildTask,
  createTask,
  deleteTask,
  dismissTask,
  enqueueTaskRun,
  failTaskRun,
  forceFailTaskRun,
  forceReleaseTaskRun,
  getTask,
  getTaskDashboard,
  getTaskInbox,
  getTaskRun,
  getTaskTimeline,
  getTaskTree,
  inspectRun,
  inspectTask,
  listTaskRuns,
  listTasks,
  markTaskRead,
  publishTask,
  recoverTaskRun,
  recoverTask,
  rejectTask,
  removeTaskDependency,
  retryTaskRun,
  startTaskRun,
  updateTask,
} from "@/systems/tasks/adapters/tasks-api";

const taskFixture = {
  id: "task_001",
  title: "Review changes",
  status: "ready" as const,
  scope: "workspace" as const,
  origin: { kind: "web" as const, ref: "op" },
  created_at: "2026-04-11T09:00:00Z",
  updated_at: "2026-04-11T09:00:00Z",
  created_by: { kind: "human" as const, ref: "op" },
};

const taskDetailFixture = {
  task: taskFixture,
  summary: {
    ...taskFixture,
    active_run: null,
  },
};

const runFixture = {
  id: "run_001",
  task_id: "task_001",
  attempt: 1,
  status: "running" as const,
  queued_at: "2026-04-11T09:00:00Z",
  origin: { kind: "web" as const, ref: "op" },
};

const runDetailFixture = {
  run: runFixture,
  task: {
    id: taskFixture.id,
    title: taskFixture.title,
    status: taskFixture.status,
    scope: taskFixture.scope,
  },
  summary: { last_activity_at: "2026-04-11T09:00:00Z" },
};

const inspectFixture = {
  target: "task",
  task: taskFixture,
  current_run: {
    run_id: runFixture.id,
    task_id: taskFixture.id,
    status: runFixture.status,
    queued_at: runFixture.queued_at,
    attempt: runFixture.attempt,
  },
  scheduler: { paused: false },
  diagnostics: [],
  next_action: "running",
  as_of: "2026-04-11T09:00:00Z",
};

const timelineFixture = {
  actor: { kind: "human" as const, ref: "op" },
  event_id: "evt_001",
  event_type: "task.updated",
  origin: { kind: "web" as const, ref: "op" },
  sequence: 1,
  task: {
    id: taskFixture.id,
    title: taskFixture.title,
    status: taskFixture.status,
    scope: taskFixture.scope,
  },
  timestamp: "2026-04-11T09:00:00Z",
};

const treeFixture = {
  root: {
    depth: 0,
    last_activity_at: "2026-04-11T09:00:00Z",
    task: {
      id: taskFixture.id,
      title: taskFixture.title,
      status: taskFixture.status,
      scope: taskFixture.scope,
    },
  },
};

const dashboardFixture = {
  active_runs: { claimed: 0, queued: 0, running: 0, starting: 0, total: 0 },
  cards: {
    blocked: { awaiting_approval: 0, awaiting_dependencies: 0, health_status: "ok", tasks: 0 },
    failed: { failed_runs: 0, forced_stops: 0, health_status: "ok", tasks: 0 },
    in_progress: {
      active_runs: 0,
      claimed_runs: 0,
      health_status: "ok",
      queued_runs: 0,
      running_runs: 0,
      starting_runs: 0,
      tasks: 0,
    },
    latency: {
      claim_latency_ms: { average_ms: 0, maximum_ms: 0, samples: 0 },
      start_latency_ms: { average_ms: 0, maximum_ms: 0, samples: 0 },
    },
  },
  freshness: {
    age_ms: 0,
    has_live_work: false,
    latest_activity_at: "2026-04-11T09:00:00Z",
    observed_at: "2026-04-11T09:00:00Z",
    stale: false,
    stale_after_ms: 10000,
    status: "fresh",
  },
  health: { active_orphan_runs: 0, queue_backlog: false, status: "ok", stuck_runs: 0 },
  queue: {
    backlog_status: "idle",
    backlog_threshold_ms: 0,
    backlog_warning: false,
    oldest_queue_age_ms: 0,
    oldest_queued_at: "2026-04-11T09:00:00Z",
    total: 0,
  },
  totals: {
    active_runs: 0,
    awaiting_approval_tasks: 0,
    blocked_tasks: 0,
    canceled_runs: 0,
    canceled_tasks: 0,
    claimed_runs: 0,
    completed_runs: 0,
    completed_tasks: 0,
    dependency_blocked_tasks: 0,
    draft_tasks: 0,
    failed_runs: 0,
    failed_tasks: 0,
    in_progress_tasks: 0,
    pending_tasks: 0,
    queued_runs: 0,
    ready_tasks: 0,
    running_runs: 0,
    runs_total: 0,
    starting_runs: 0,
    tasks_total: 0,
  },
};

const inboxFixture = {
  archived_total: 0,
  facets: { priorities: [], statuses: [] },
  groups: [],
  page: { has_more: true, limit: 1, next_cursor: "inbox-next", total: 4 },
  unread_total: 0,
};

const taskListPageFixture = {
  facets: {
    owners: [],
    statuses: [{ count: 4, status: "ready" as const }],
  },
  page: { has_more: true, limit: 1, next_cursor: "tasks-next", total: 4 },
  tasks: [taskFixture],
};

const triageFixture = {
  actor: { kind: "human" as const, ref: "op" },
  archived: false,
  dismissed: false,
  read: true,
  task_id: taskFixture.id,
  updated_at: "2026-04-11T09:00:00Z",
};

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("listTasks", () => {
  it("preserves the counted envelope and forwards every normalized catalog parameter", async () => {
    mockJsonResponse(taskListPageFixture);

    const result = await listTasks({
      scope: "workspace",
      workspace: "  ws_alpha  ",
      status: "ready",
      priority: "high",
      include_drafts: true,
      query: " review ",
      sort: "priority",
      cursor: "tasks-cursor",
      limit: 25,
    });

    expect(result).toEqual(taskListPageFixture);
    await expectFetchRequest({
      path: "/api/tasks?scope=workspace&workspace=ws_alpha&status=ready&priority=high&include_drafts=true&query=review&sort=priority&cursor=tasks-cursor&limit=25",
    });
  });

  it("omits empty string filters", async () => {
    mockJsonResponse({ tasks: [] });

    await listTasks({ workspace: "   ", owner_ref: "" });

    await expectFetchRequest({ path: "/api/tasks" });
  });

  it("throws TasksApiError on non-2xx", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(listTasks()).rejects.toThrow(TasksApiError);
    await expect(listTasks()).rejects.toThrow("Failed to fetch tasks: 500");
  });
});

describe("getTask", () => {
  it("fetches task detail by id", async () => {
    mockJsonResponse({ task: taskDetailFixture });

    const result = await getTask("task_001");

    expect(result).toEqual(taskDetailFixture);
    await expectFetchRequest({ path: "/api/tasks/task_001" });
  });

  it("throws not-found for 404", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(getTask("missing")).rejects.toThrow("Task not found: missing");
  });
});

describe("inspectTask", () => {
  it("fetches task inspect snapshots by id", async () => {
    mockJsonResponse({ inspect: inspectFixture });

    const result = await inspectTask("task_001");

    expect(result).toEqual(inspectFixture);
    await expectFetchRequest({ path: "/api/tasks/task_001/inspect" });
  });

  it("throws not-found for missing task inspect targets", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(inspectTask("missing")).rejects.toThrow("Task not found: missing");
  });
});

describe("deleteTask", () => {
  it("calls DELETE /api/tasks/{id}", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 204 }));

    await deleteTask("task_001");

    await expectFetchRequest({ method: "DELETE", path: "/api/tasks/task_001" });
  });

  it("throws not-found for 404", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    const error = await deleteTask("missing").catch(err => err);

    expect(error).toBeInstanceOf(TasksApiError);
    expect(error).toMatchObject({
      message: "Task not found: missing",
      status: 404,
    });
  });
});

describe("task mutations", () => {
  it("creates a task", async () => {
    mockJsonResponse({ task: taskFixture }, { status: 201 });

    const body = { title: "Review", scope: "workspace" as const };
    const result = await createTask(body);

    expect(result).toEqual(taskFixture);
    await expectFetchRequest({ body, method: "POST", path: "/api/tasks" });
  });

  it("updates a task", async () => {
    mockJsonResponse({ task: { ...taskFixture, title: "Updated" } });

    const result = await updateTask("task_001", { title: "Updated" });

    expect(result.title).toBe("Updated");
    await expectFetchRequest({
      body: { title: "Updated" },
      method: "PATCH",
      path: "/api/tasks/task_001",
    });
  });

  it("publishes a draft task", async () => {
    mockJsonResponse({ task: taskFixture, run: { ...runFixture, status: "queued" } });

    const result = await publishTask("task_001");

    expect(result).toEqual(taskFixture);
    await expectFetchRequest({
      body: {},
      method: "POST",
      path: "/api/tasks/task_001/publish",
    });
  });

  it("cancels a task with default body", async () => {
    mockJsonResponse({ task: taskFixture });

    const result = await cancelTask("task_001");

    expect(result).toEqual(taskFixture);
    await expectFetchRequest({ body: {}, method: "POST", path: "/api/tasks/task_001/cancel" });
  });

  it("approves and rejects approval-gated tasks", async () => {
    mockJsonSequence({ task: taskFixture, run: { ...runFixture, status: "queued" } });

    await approveTask("task_001");
    await rejectTask("task_001");

    await expectFetchRequest({ body: {}, method: "POST", path: "/api/tasks/task_001/approve" });
    await expectFetchRequest({
      callIndex: 1,
      method: "POST",
      path: "/api/tasks/task_001/reject",
    });
  });

  it("creates a child task", async () => {
    mockJsonResponse({ task: taskFixture }, { status: 201 });

    const body = { title: "Child", scope: "workspace" as const };
    await createChildTask("task_001", body);

    await expectFetchRequest({
      body,
      method: "POST",
      path: "/api/tasks/task_001/children",
    });
  });

  it("maps task recover conflict to a needs_attention error", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(
      new Response(JSON.stringify({ error: "Task task_001 is not in needs_attention" }), {
        status: 409,
        headers: { "Content-Type": "application/json" },
      })
    );

    const error = await recoverTask("task_001").catch(err => err);

    expect(error).toBeInstanceOf(TasksApiError);
    expect(error).toMatchObject({
      message: "Task task_001 is not in needs_attention",
      status: 409,
    });
  });

  it("recovers one needs_attention run through the continuation contract", async () => {
    const previousRun = { ...runFixture, status: "needs_attention" as const };
    const continuation = {
      ...runFixture,
      id: "run_002",
      attempt: 2,
      previous_run_id: "run_001",
      status: "queued" as const,
    };
    mockJsonResponse({ previous_run: previousRun, run: continuation }, { status: 201 });

    const result = await recoverTaskRun("run_001", { reason: "scheduler starvation" });

    expect(result).toEqual({ previous_run: previousRun, run: continuation });
    await expectFetchRequest({
      body: { reason: "scheduler starvation" },
      method: "POST",
      path: "/api/runs/run_001/recover",
    });
  });

  it("preserves the actionable API error when a run cannot be recovered", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(
      new Response(
        JSON.stringify({
          error: "Run run_001 is running; only needs_attention runs can be recovered.",
        }),
        { status: 409, headers: { "Content-Type": "application/json" } }
      )
    );

    const error = await recoverTaskRun("run_001").catch(err => err);

    expect(error).toBeInstanceOf(TasksApiError);
    expect(error).toMatchObject({
      message: "Run run_001 is running; only needs_attention runs can be recovered.",
      status: 409,
    });
  });

  it("adds and removes task dependencies", async () => {
    mockJsonSequence({ task: taskDetailFixture });

    await addTaskDependency("task_001", { depends_on_task_id: "task_002" });
    await removeTaskDependency("task_001", "task_002");

    await expectFetchRequest({
      body: { depends_on_task_id: "task_002" },
      method: "POST",
      path: "/api/tasks/task_001/dependencies",
    });
    await expectFetchRequest({
      callIndex: 1,
      method: "DELETE",
      path: "/api/tasks/task_001/dependencies/task_002",
    });
  });
});

describe("task runs", () => {
  it("lists task runs with filters", async () => {
    mockJsonResponse({ runs: [runFixture] });

    const result = await listTaskRuns("task_001", {
      status: "running",
      session_id: " sess_a ",
      limit: 5,
    });

    expect(result).toEqual([runFixture]);
    await expectFetchRequest({
      path: "/api/tasks/task_001/runs?status=running&session_id=sess_a&limit=5",
    });
  });

  it("enqueues a task run", async () => {
    mockJsonResponse({ run: runFixture }, { status: 201 });

    await enqueueTaskRun("task_001", { idempotency_key: "idem_1" });

    await expectFetchRequest({
      body: { idempotency_key: "idem_1" },
      method: "POST",
      path: "/api/tasks/task_001/runs",
    });
  });

  it("fetches task timeline", async () => {
    mockJsonResponse({ timeline: [timelineFixture] });

    await getTaskTimeline("task_001", { after_sequence: 10, limit: 20 });

    await expectFetchRequest({
      path: "/api/tasks/task_001/timeline?after_sequence=10&limit=20",
    });
  });

  it("fetches task tree", async () => {
    mockJsonResponse({ tree: treeFixture });

    const result = await getTaskTree("task_001");

    expect(result).toEqual(treeFixture);
    await expectFetchRequest({ path: "/api/tasks/task_001/tree" });
  });

  it("fetches task-run detail", async () => {
    mockJsonResponse({ run: runDetailFixture });

    const result = await getTaskRun("run_001");

    expect(result).toEqual(runDetailFixture);
    await expectFetchRequest({ path: "/api/task-runs/run_001" });
  });

  it("accepts a taskless network-wake run detail", async () => {
    const tasklessRun = {
      ...runDetailFixture,
      run: { ...runFixture, kind: "network_wake", task_id: undefined },
      task: null,
    };
    mockJsonResponse({ run: tasklessRun });

    await expect(getTaskRun("run_wake")).resolves.toEqual(tasklessRun);
    await expectFetchRequest({ path: "/api/task-runs/run_wake" });
  });

  it("fetches run inspect snapshots", async () => {
    mockJsonResponse({ inspect: { ...inspectFixture, target: "run" } });

    const result = await inspectRun("run_001");

    expect(result.target).toBe("run");
    await expectFetchRequest({ path: "/api/runs/run_001/inspect" });
  });

  it("runs lifecycle commands against /api/task-runs/{id}/*", async () => {
    mockJsonSequence({ run: runFixture });

    await startTaskRun("run_001");
    await completeTaskRun("run_001", { result: { ok: true } });
    await failTaskRun("run_001", { error: "boom" });
    await cancelTaskRun("run_001");
    await attachTaskRunSession("run_001", { session_id: "sess_a" });

    await expectFetchRequest({
      body: {},
      method: "POST",
      path: "/api/task-runs/run_001/start",
    });
    await expectFetchRequest({
      body: { result: { ok: true } },
      callIndex: 1,
      method: "POST",
      path: "/api/task-runs/run_001/complete",
    });
    await expectFetchRequest({
      body: { error: "boom" },
      callIndex: 2,
      method: "POST",
      path: "/api/task-runs/run_001/fail",
    });
    await expectFetchRequest({
      body: {},
      callIndex: 3,
      method: "POST",
      path: "/api/task-runs/run_001/cancel",
    });
    await expectFetchRequest({
      body: { session_id: "sess_a" },
      callIndex: 4,
      method: "POST",
      path: "/api/task-runs/run_001/attach-session",
    });
  });

  it("runs force recovery commands against /api/runs/{id}/*", async () => {
    mockJsonSequence({ run: runFixture });

    await forceReleaseTaskRun("run_001", { reason: "handoff" });
    await forceFailTaskRun("run_001", { reason: "operator recovery" });
    const retry = await retryTaskRun("run_001", { metadata: { source: "operator" } });

    expect(retry.run).toEqual(runFixture);
    await expectFetchRequest({
      body: { reason: "handoff" },
      method: "POST",
      path: "/api/runs/run_001/release",
    });
    await expectFetchRequest({
      body: { reason: "operator recovery" },
      callIndex: 1,
      method: "POST",
      path: "/api/runs/run_001/fail",
    });
    await expectFetchRequest({
      body: { metadata: { source: "operator" } },
      callIndex: 2,
      method: "POST",
      path: "/api/runs/run_001/retry",
    });
  });
});

describe("dashboard and inbox", () => {
  it("fetches dashboard payload with filter normalization", async () => {
    mockJsonResponse({ dashboard: dashboardFixture });

    await getTaskDashboard({ scope: "workspace", workspace: "  ws_a  " });

    await expectFetchRequest({
      path: "/api/observe/tasks/dashboard?scope=workspace&workspace=ws_a",
    });
  });

  it("preserves inbox totals/facets and forwards server-owned filters and cursor", async () => {
    mockJsonResponse({ inbox: inboxFixture });

    const result = await getTaskInbox({
      scope: "workspace",
      workspace: "ws_a",
      lane: "my_work",
      status: "ready",
      priority: "high",
      unread: true,
      cursor: "inbox-cursor",
      limit: 10,
    });

    expect(result).toEqual(inboxFixture);
    await expectFetchRequest({
      path: "/api/observe/tasks/inbox?scope=workspace&workspace=ws_a&lane=my_work&status=ready&priority=high&unread=true&cursor=inbox-cursor&limit=10",
    });
  });
});

describe("triage mutations", () => {
  it("marks a task read", async () => {
    mockJsonResponse({ triage: triageFixture });

    const result = await markTaskRead("task_001");

    expect(result).toEqual(triageFixture);
    await expectFetchRequest({
      method: "POST",
      path: "/api/tasks/task_001/triage/read",
    });
  });

  it("archives and dismisses tasks", async () => {
    mockJsonSequence({ triage: triageFixture });

    await archiveTask("task_001");
    await dismissTask("task_001");

    await expectFetchRequest({
      method: "POST",
      path: "/api/tasks/task_001/triage/archive",
    });
    await expectFetchRequest({
      callIndex: 1,
      method: "POST",
      path: "/api/tasks/task_001/triage/dismiss",
    });
  });
});

describe("TasksApiError", () => {
  it("stores status", () => {
    const err = new TasksApiError("boom", 422);

    expect(err.name).toBe("TasksApiError");
    expect(err.status).toBe(422);
    expect(err.message).toBe("boom");
  });
});
