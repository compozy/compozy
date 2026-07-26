import { HttpResponse, type HttpHandler } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";

import type {
  CreateTaskRequest,
  FanOutTaskRunsRequest,
  TaskBridgeNotificationSubscription,
  TaskBridgeNotificationSubscriptionCreateRequest,
  TaskExecutionProfileSetRequest,
  TaskInspectView,
  TaskListItem,
  TaskRecord,
  TaskRun,
  TaskRunReviewRequest,
  TaskRunReviewVerdictRequest,
  TaskSummary,
  TaskTriageState,
  UpdateTaskRequest,
} from "../types";
import {
  TASK_CATALOG_FIXTURES,
  TASK_FIXTURES,
  agentContextFixture,
  buildBridgeNotificationCursorFixture,
  buildCreatedTaskFixture,
  buildDetailFixture,
  buildTaskInspectFixture,
  buildTaskBridgeNotificationSubscriptionFixture,
  buildTaskExecutionProfileFixture,
  buildTaskRunRecordFixture,
  buildTaskRunDetailFixture,
  buildTaskRunReviewFixture,
  buildTaskRunReviewVerdictResultFixture,
  buildTaskTreeFixture,
  taskBridgeNotificationSubscriptionsFixture,
  taskDashboardFixture,
  taskDetailFixture,
  taskExecutionProfileFixture,
  taskInboxFixture,
  taskRunDetailFixture,
  taskRunReviewFixture,
  taskRunReviewListFixture,
  taskTimelineFixture,
  taskTriageStateFixture,
} from "./fixtures";
import { buildTaskCatalogResponse, buildTaskInboxResponse } from "./query-responses";

function resolveTask(id: string): TaskListItem | null {
  return TASK_FIXTURES.find(task => task.id === id) ?? null;
}

function resolveTaskRecord(id: string): TaskRecord | null {
  const task = resolveTask(id);
  return task ? ({ ...task } as TaskRecord) : null;
}

function summaryFromTask(task: TaskListItem): TaskSummary {
  return {
    ...(resolveTaskRecord(task.id) ?? (task as unknown as TaskRecord)),
    active_run: task.active_run ?? null,
    child_count: task.child_count ?? 0,
    dependency_count: task.dependency_count ?? 0,
  } as TaskSummary;
}

function runRecordFromActiveRun(task: TaskListItem): TaskRun | null {
  if (!task.active_run) {
    return null;
  }

  return buildTaskRunRecordFixture({
    id: task.active_run.id,
    task_id: task.active_run.task_id,
    attempt: task.active_run.attempt,
    recovery_count: task.active_run.recovery_count,
    status: task.active_run.status,
    queued_at: task.active_run.queued_at,
    started_at: task.active_run.started_at,
    ended_at: task.active_run.ended_at,
    claimed_by: task.active_run.claimed_by,
    error: task.active_run.error,
    session_id: task.active_run.session_id,
    resolved_network_participation: task.active_run.resolved_network_participation,
  });
}

function resolveTaskDetail(id: string) {
  const task = resolveTask(id);
  if (!task) {
    return null;
  }

  if (id === taskDetailFixture.task.id) {
    return taskDetailFixture;
  }

  return buildDetailFixture({
    task: {
      ...(resolveTaskRecord(task.id) ?? (task as unknown as TaskRecord)),
      description: `${task.title} detail for Storybook route coverage.`,
    } as TaskRecord,
    summary: summaryFromTask(task),
    children: [],
    dependency_references: [],
    runs: runRecordFromActiveRun(task) ? [runRecordFromActiveRun(task)!] : [],
  });
}

function resolveTaskRuns(taskId: string): TaskRun[] {
  if (taskId === taskDetailFixture.task.id) {
    return taskDetailFixture.runs ?? [];
  }

  const task = resolveTask(taskId);
  const run = task ? runRecordFromActiveRun(task) : null;
  return run ? [run] : [];
}

function resolveTaskTree(taskId: string) {
  if (taskId === taskDetailFixture.task.id) {
    return buildTaskTreeFixture();
  }

  const task = resolveTask(taskId);
  if (!task) {
    return null;
  }

  return buildTaskTreeFixture({
    root: {
      task: task as unknown as TaskRecord,
      active_run: task.active_run ?? null,
      depth: 0,
      parent_task_id: undefined,
      child_count: 0,
      last_activity_at: task.last_activity_at ?? task.updated_at,
    },
    descendants: [],
  });
}

function resolveTaskRun(runId: string) {
  if (runId === taskRunDetailFixture.run.id) {
    return taskRunDetailFixture;
  }

  const primaryRun = (taskDetailFixture.runs ?? []).find(run => run.id === runId);
  if (primaryRun) {
    return buildTaskRunDetailFixture({
      run: primaryRun,
      task: taskDetailFixture.task,
      summary: taskRunDetailFixture.summary,
      session:
        primaryRun.session_id === undefined
          ? null
          : {
              session_id: primaryRun.session_id,
              created_at: taskRunDetailFixture.session?.created_at ?? "2026-04-17T09:58:00Z",
              updated_at: taskRunDetailFixture.session?.updated_at ?? "2026-04-17T10:01:00Z",
              agent_name: taskRunDetailFixture.session?.agent_name,
              name: taskRunDetailFixture.session?.name,
              state: taskRunDetailFixture.session?.state,
              workspace_id: taskRunDetailFixture.session?.workspace_id,
            },
    });
  }

  for (const task of TASK_FIXTURES) {
    if (task.active_run?.id === runId) {
      const run = runRecordFromActiveRun(task);
      return buildTaskRunDetailFixture({
        run: run ?? undefined,
        task: task as unknown as TaskRecord,
        session: task.active_run.session_id
          ? {
              ...(taskRunDetailFixture.session ?? {
                created_at: "2026-04-17T09:58:00Z",
                updated_at: "2026-04-17T10:01:00Z",
              }),
              session_id: task.active_run.session_id,
            }
          : null,
      });
    }
  }

  return null;
}

function filterRuns(runs: TaskRun[], requestUrl: URL) {
  const status = requestUrl.searchParams.get("status");
  const sessionId = requestUrl.searchParams.get("session_id");

  return runs.filter(run => {
    if (status && run.status !== status) return false;
    if (sessionId && run.session_id !== sessionId) return false;
    return true;
  });
}

function withTriageState(taskId: string, overrides: Partial<TaskTriageState> = {}) {
  return {
    ...taskTriageStateFixture,
    task_id: taskId,
    updated_at: "2026-04-17T10:05:00Z",
    ...overrides,
  } as TaskTriageState;
}

function notFound(entity: string, id: string) {
  return { error: `${entity} not found: ${id}` };
}

export const handlers: HttpHandler[] = [
  aghApiMock.get("/api/tasks", ({ request }) =>
    HttpResponse.json(buildTaskCatalogResponse(TASK_CATALOG_FIXTURES, new URL(request.url)))
  ),
  aghApiMock.get("/api/tasks/{id}", ({ params, response }) => {
    const id = String(params.id);
    const detail = resolveTaskDetail(id);

    if (!detail) {
      return response(404).json(notFound("Task", id));
    }

    return HttpResponse.json({ task: detail });
  }),
  aghApiMock.get("/api/tasks/{id}/runs", ({ params, request, response }) => {
    const id = String(params.id);
    if (!resolveTask(id)) {
      return response(404).json(notFound("Task", id));
    }

    return HttpResponse.json({ runs: filterRuns(resolveTaskRuns(id), new URL(request.url)) });
  }),
  aghApiMock.get("/api/tasks/{id}/timeline", ({ params, request, response }) => {
    const id = String(params.id);
    if (!resolveTask(id)) {
      return response(404).json(notFound("Task", id));
    }

    const limit = Number(new URL(request.url).searchParams.get("limit") ?? "0");
    const timeline =
      Number.isFinite(limit) && limit > 0
        ? taskTimelineFixture.slice(0, limit)
        : taskTimelineFixture;

    return HttpResponse.json({ timeline });
  }),
  aghApiMock.get("/api/tasks/{id}/tree", ({ params, response }) => {
    const id = String(params.id);
    const tree = resolveTaskTree(id);

    if (!tree) {
      return response(404).json(notFound("Task", id));
    }

    return HttpResponse.json({ tree });
  }),
  aghApiMock.get("/api/tasks/{id}/inspect", ({ params, response }) => {
    const id = String(params.id);
    const task = resolveTask(id);

    if (!task) {
      return response(404).json(notFound("Task", id));
    }

    return HttpResponse.json({
      inspect: buildTaskInspectFixture({ task, target: "task" }),
    });
  }),
  aghApiMock.get("/api/task-runs/{id}", ({ params, response }) => {
    const id = String(params.id);
    const run = resolveTaskRun(id);

    if (!run) {
      return response(404).json(notFound("Task run", id));
    }

    return HttpResponse.json({ run });
  }),
  aghApiMock.get("/api/runs/{id}/inspect", ({ params, response }) => {
    const id = String(params.id);
    const run = resolveTaskRun(id);

    if (!run) {
      return response(404).json(notFound("Task run", id));
    }

    return HttpResponse.json({
      inspect: buildTaskInspectFixture({
        target: "run",
        task: run.task as TaskInspectView["task"],
        current_run: {
          run_id: run.run.id,
          task_id: run.run.task_id,
          status: run.run.status,
          claim_token_hash_truncated: "abcdef12",
          bound_session_id: run.run.session_id ?? undefined,
          queued_at: run.run.queued_at,
          started_at: run.run.started_at ?? undefined,
          ended_at: run.run.ended_at ?? undefined,
          attempt: run.run.attempt,
          retries: Math.max(0, run.run.attempt - 1),
          last_error_summary: run.run.error,
        },
      }),
    });
  }),
  aghApiMock.get("/api/observe/tasks/dashboard", () =>
    HttpResponse.json({ dashboard: taskDashboardFixture })
  ),
  aghApiMock.get("/api/observe/tasks/inbox", ({ request }) =>
    HttpResponse.json({ inbox: buildTaskInboxResponse(taskInboxFixture, new URL(request.url)) })
  ),
  aghApiMock.post("/api/tasks", async ({ request }) => {
    const body = (await request.json()) as Partial<CreateTaskRequest>;

    return HttpResponse.json({ task: buildCreatedTaskFixture(body) }, { status: 201 });
  }),
  aghApiMock.patch("/api/tasks/{id}", async ({ params, request, response }) => {
    const id = String(params.id);
    const task = resolveTaskRecord(id);
    if (!task) {
      return response(404).json(notFound("Task", id));
    }

    const body = (await request.json()) as Partial<UpdateTaskRequest>;
    return HttpResponse.json({
      task: {
        ...task,
        title: body.title ?? task.title,
        description: body.description ?? task.description,
        priority: body.priority ?? task.priority,
        owner: body.clear_owner ? null : (body.owner ?? task.owner),
        max_attempts: body.max_attempts ?? task.max_attempts,
        approval_policy:
          body.approval_policy === "none"
            ? undefined
            : (body.approval_policy ?? task.approval_policy),
      },
    });
  }),
  aghApiMock.post("/api/tasks/{id}/publish", ({ params, response }) => {
    const id = String(params.id);
    const task = resolveTaskRecord(id);
    if (!task) {
      return response(404).json(notFound("Task", id));
    }

    return HttpResponse.json({
      run: buildTaskRunRecordFixture({
        id: `run_publish_${id}`,
        task_id: id,
        attempt: 1,
        status: "queued",
        queued_at: "2026-04-17T10:05:00Z",
        started_at: null,
        session_id: undefined,
      }),
      task: {
        ...task,
        status: "ready",
      },
    });
  }),
  aghApiMock.post("/api/tasks/{id}/cancel", ({ params, response }) => {
    const id = String(params.id);
    const task = resolveTaskRecord(id);
    if (!task) {
      return response(404).json(notFound("Task", id));
    }

    return HttpResponse.json({
      task: {
        ...task,
        status: "canceled",
      },
    });
  }),
  aghApiMock.post("/api/tasks/{id}/pause", async ({ params, request, response }) => {
    const id = String(params.id);
    const task = resolveTaskRecord(id);
    if (!task) {
      return response(404).json(notFound("Task", id));
    }
    const body = (await request.json()) as { reason?: string };

    return HttpResponse.json({
      task: {
        ...task,
        paused: true,
        effective_paused: true,
        paused_by: "human:storybook",
        paused_at: "2026-04-17T10:05:00Z",
        paused_by_task_id: task.id,
        paused_reason: body.reason ?? "storybook pause",
      },
    });
  }),
  aghApiMock.post("/api/tasks/{id}/resume", ({ params, response }) => {
    const id = String(params.id);
    const task = resolveTaskRecord(id);
    if (!task) {
      return response(404).json(notFound("Task", id));
    }

    return HttpResponse.json({
      task: {
        ...task,
        paused: false,
        effective_paused: false,
        paused_by: "",
        paused_at: null,
        paused_by_task_id: "",
        paused_reason: "",
      },
    });
  }),
  aghApiMock.post("/api/tasks/{id}/recover", ({ params, response }) => {
    const id = String(params.id);
    const task = resolveTaskRecord(id);
    if (!task) {
      return response(404).json(notFound("Task", id));
    }
    if (task.status !== "needs_attention") {
      return response(409).json({ error: `Task ${id} is not in needs_attention` });
    }

    // Models the all-clear recovery path only (no other open blocking causes):
    // real recover reconciles through auto-enqueue and would derive `blocked`
    // when other open blocks remain.
    return HttpResponse.json({
      task: {
        ...task,
        status: "ready",
        needs_attention: false,
        needs_attention_at: null,
        needs_attention_by: null,
        needs_attention_reason: undefined,
        blocked_reasons: [],
      },
    });
  }),
  aghApiMock.post("/api/tasks/{id}/approve", ({ params, response }) => {
    const id = String(params.id);
    const task = resolveTaskRecord(id);
    if (!task) {
      return response(404).json(notFound("Task", id));
    }

    return HttpResponse.json(
      {
        run: buildTaskRunRecordFixture({
          id: `run_approve_${id}`,
          task_id: id,
          attempt: 1,
          status: "queued",
          queued_at: "2026-04-17T10:05:00Z",
          started_at: null,
          session_id: undefined,
        }),
        task: {
          ...task,
          status: "ready",
          approval_state: "approved",
        },
      },
      { status: 201 }
    );
  }),
  aghApiMock.post("/api/tasks/{id}/reject", ({ params, response }) => {
    const id = String(params.id);
    const task = resolveTaskRecord(id);
    if (!task) {
      return response(404).json(notFound("Task", id));
    }

    return HttpResponse.json({
      task: {
        ...task,
        status: "blocked",
        approval_state: "rejected",
      },
    });
  }),
  aghApiMock.post("/api/tasks/{id}/runs", ({ params, response }) => {
    const id = String(params.id);
    const task = resolveTask(id);
    if (!task) {
      return response(404).json(notFound("Task", id));
    }

    return HttpResponse.json(
      {
        run: buildTaskRunRecordFixture({
          id: "run_created",
          task_id: id,
          attempt: 1,
          status: "queued",
          queued_at: "2026-04-17T10:05:00Z",
          started_at: null,
          session_id: undefined,
        }),
      },
      { status: 201 }
    );
  }),
  aghApiMock.post("/api/tasks/{id}/runs/fan-out", async ({ params, request, response }) => {
    const id = String(params.id);
    const task = resolveTask(id);
    if (!task) {
      return response(404).json(notFound("Task", id));
    }

    const body = (await request.json()) as Partial<FanOutTaskRunsRequest>;
    const designations = body.designations ?? [];
    const runs = designations.map((designation, index) =>
      buildTaskRunRecordFixture({
        id: `run_fanout_${index + 1}`,
        task_id: id,
        attempt: index + 1,
        status: "queued",
        queued_at: "2026-04-17T10:05:00Z",
        started_at: null,
        session_id: undefined,
        designation: {
          index,
          brief: designation.brief,
        },
        designation_group_id: "desig_storybook",
      })
    );

    return HttpResponse.json(
      {
        designation_group_id: "desig_storybook",
        runs,
      },
      { status: 201 }
    );
  }),
  aghApiMock.post("/api/tasks/{id}/triage/read", ({ params }) =>
    HttpResponse.json({ triage: withTriageState(String(params.id), { read: true }) })
  ),
  aghApiMock.post("/api/tasks/{id}/triage/archive", ({ params }) =>
    HttpResponse.json({
      triage: withTriageState(String(params.id), { archived: true, read: true }),
    })
  ),
  aghApiMock.post("/api/tasks/{id}/triage/dismiss", ({ params }) =>
    HttpResponse.json({
      triage: withTriageState(String(params.id), { dismissed: true, read: true }),
    })
  ),

  // Execution profile
  aghApiMock.get("/api/tasks/{id}/execution-profile", ({ params, response }) => {
    const id = String(params.id);
    if (!resolveTask(id)) {
      return response(404).json(notFound("Task", id));
    }
    return HttpResponse.json({
      profile: { ...taskExecutionProfileFixture, task_id: id },
    });
  }),
  aghApiMock.put("/api/tasks/{id}/execution-profile", async ({ params, request, response }) => {
    const id = String(params.id);
    if (!resolveTask(id)) {
      return response(404).json(notFound("Task", id));
    }
    const body = (await request.json()) as TaskExecutionProfileSetRequest;
    return HttpResponse.json({
      profile: buildTaskExecutionProfileFixture({ ...body, task_id: id }),
    });
  }),
  aghApiMock.delete("/api/tasks/{id}/execution-profile", ({ params, response }) => {
    const id = String(params.id);
    if (!resolveTask(id)) {
      return response(404).json(notFound("Task", id));
    }
    return response(204).empty();
  }),

  // Run reviews
  aghApiMock.get("/api/task-runs/{id}/reviews", ({ params, request }) => {
    const runId = String(params.id);
    const url = new URL(request.url);
    const status = url.searchParams.get("status");
    const reviewerSessionId = url.searchParams.get("reviewer_session_id");
    const filtered = taskRunReviewListFixture.filter(review => {
      if (status && review.status !== status) return false;
      if (reviewerSessionId && review.reviewer_session_id !== reviewerSessionId) return false;
      return review.run_id === runId || true;
    });
    return HttpResponse.json({ reviews: filtered });
  }),
  aghApiMock.post("/api/task-runs/{id}/reviews", async ({ params, request }) => {
    const runId = String(params.id);
    const body = (await request.json()) as TaskRunReviewRequest;
    const review = buildTaskRunReviewFixture({
      review_id: "review_created",
      run_id: runId,
      task_id: body.task_id,
      review_round: body.review_round ?? 1,
      attempt: body.attempt ?? 1,
      policy: body.policy ?? "on_success",
      reason: body.reason,
      deadline_at: body.deadline_at,
    });
    return HttpResponse.json({ review, created: true }, { status: 201 });
  }),
  aghApiMock.get("/api/task-reviews/{id}", ({ params, response }) => {
    const reviewId = String(params.id);
    const review =
      taskRunReviewListFixture.find(item => item.review_id === reviewId) ??
      (reviewId === taskRunReviewFixture.review_id ? taskRunReviewFixture : null);
    if (!review) {
      return response(404).json(notFound("Task review", reviewId));
    }
    return HttpResponse.json({ review });
  }),
  aghApiMock.post("/api/task-reviews/{id}/verdict", async ({ params, request }) => {
    const reviewId = String(params.id);
    const body = (await request.json()) as TaskRunReviewVerdictRequest;
    const verdictResult = buildTaskRunReviewVerdictResultFixture({
      review: buildTaskRunReviewFixture({
        review_id: reviewId,
        run_id: body.run_id,
        status: "recorded",
        outcome: body.verdict.outcome,
        reason: body.verdict.reason,
        next_round_guidance: body.verdict.next_round_guidance,
        confidence: body.verdict.confidence ?? undefined,
        delivery_id: body.verdict.delivery_id,
        review_text: body.verdict.review_text,
      }),
    });
    return HttpResponse.json(verdictResult);
  }),

  // Task-level reviews
  aghApiMock.get("/api/tasks/{id}/reviews", ({ params, response }) => {
    const taskId = String(params.id);
    if (!resolveTask(taskId)) {
      return response(404).json(notFound("Task", taskId));
    }
    return HttpResponse.json({ reviews: taskRunReviewListFixture });
  }),

  // Bridge notification subscriptions
  aghApiMock.get("/api/tasks/{id}/notifications/bridges", ({ params, request, response }) => {
    const taskId = String(params.id);
    if (!resolveTask(taskId)) {
      return response(404).json(notFound("Task", taskId));
    }
    const url = new URL(request.url);
    const bridgeInstanceId = url.searchParams.get("bridge_instance_id");
    const scope = url.searchParams.get("scope");
    const workspaceId = url.searchParams.get("workspace_id");
    const filtered: TaskBridgeNotificationSubscription[] =
      taskBridgeNotificationSubscriptionsFixture.filter(sub => {
        if (bridgeInstanceId && sub.bridge_instance_id !== bridgeInstanceId) return false;
        if (scope && sub.scope !== scope) return false;
        if (workspaceId && sub.workspace_id !== workspaceId) return false;
        return true;
      });
    return HttpResponse.json({ subscriptions: filtered });
  }),
  aghApiMock.post(
    "/api/tasks/{id}/notifications/bridges",
    async ({ params, request, response }) => {
      const taskId = String(params.id);
      if (!resolveTask(taskId)) {
        return response(404).json(notFound("Task", taskId));
      }
      const body = (await request.json()) as TaskBridgeNotificationSubscriptionCreateRequest;
      const subscriptionId = body.subscription_id ?? "bsub_created";
      const subscription = buildTaskBridgeNotificationSubscriptionFixture({
        subscription_id: subscriptionId,
        task_id: taskId,
        bridge_instance_id: body.bridge_instance_id,
        delivery_mode: body.delivery_mode,
        scope: body.scope,
        workspace_id: body.workspace_id,
        peer_id: body.peer_id,
        group_id: body.group_id,
        thread_id: body.thread_id,
        cursor: buildBridgeNotificationCursorFixture({
          consumer_id: `bridge_task_subscription:${subscriptionId}`,
          subject_id: taskId,
          last_sequence: 0,
          last_delivery_id: undefined,
          last_delivered_at: null,
          updated_at: null,
        }),
      });
      return HttpResponse.json({ subscription }, { status: 201 });
    }
  ),
  aghApiMock.get(
    "/api/tasks/{id}/notifications/bridges/{subscription_id}",
    ({ params, response }) => {
      const taskId = String(params.id);
      const subscriptionId = String(params.subscription_id);
      const subscription = taskBridgeNotificationSubscriptionsFixture.find(
        item => item.subscription_id === subscriptionId
      );
      if (!subscription) {
        return response(404).json(notFound("Bridge notification subscription", subscriptionId));
      }
      return HttpResponse.json({ subscription: { ...subscription, task_id: taskId } });
    }
  ),
  aghApiMock.delete(
    "/api/tasks/{id}/notifications/bridges/{subscription_id}",
    ({ params, response }) => {
      const subscriptionId = String(params.subscription_id);
      const subscription = taskBridgeNotificationSubscriptionsFixture.find(
        item => item.subscription_id === subscriptionId
      );
      if (!subscription) {
        return response(404).json(notFound("Bridge notification subscription", subscriptionId));
      }
      return response(204).empty();
    }
  ),

  // Agent context (carries the task context bundle)
  aghApiMock.get("/api/agent/context", () => HttpResponse.json({ context: agentContextFixture })),
];
