import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockJsonResponse } from "@/test/fetch-test-utils";
import {
  TasksApiError,
  clearTaskBlock,
  createTaskBridgeNotificationSubscription,
  deleteTaskBridgeNotificationSubscription,
  deleteTaskExecutionProfile,
  getAgentContext,
  getTaskBridgeNotificationSubscription,
  getTaskContextBundle,
  getTaskExecutionProfile,
  getTaskRunReview,
  listTaskBridgeNotificationSubscriptions,
  listTaskReviews,
  listTaskRunReviews,
  requestTaskRunReview,
  setTaskExecutionProfile,
  submitTaskRunReviewVerdict,
} from "@/systems/tasks/adapters/tasks-api";
import {
  agentContextFixture,
  taskBridgeNotificationSubscriptionFixture,
  taskBridgeNotificationSubscriptionsFixture,
  taskContextBundleFixture,
  taskExecutionProfileFixture,
  taskRunReviewFixture,
  taskRunReviewListFixture,
  taskRunReviewVerdictResultFixture,
} from "@/systems/tasks/mocks/fixtures";
import type {
  TaskExecutionProfileSetRequest,
  TaskRunReviewRequest,
  TaskRunReviewVerdictRequest,
} from "@/systems/tasks/types";

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("execution profile adapters", () => {
  it("Should fetch execution profile by task id", async () => {
    mockJsonResponse({ profile: taskExecutionProfileFixture });

    const result = await getTaskExecutionProfile("task_001");

    expect(result).toEqual(taskExecutionProfileFixture);
    await expectFetchRequest({ path: "/api/tasks/task_001/execution-profile" });
  });

  it("Should map task-not-found to TasksApiError 404", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(getTaskExecutionProfile("missing")).rejects.toThrow("Task not found: missing");
  });

  it("Should propagate AbortSignal through profile reads", async () => {
    mockJsonResponse({ profile: taskExecutionProfileFixture });
    const controller = new AbortController();

    await getTaskExecutionProfile("task_001", controller.signal);

    await expectFetchRequest({
      path: "/api/tasks/task_001/execution-profile",
      signal: controller.signal,
    });
  });

  it("Should PUT a profile update with the provided body", async () => {
    mockJsonResponse({ profile: taskExecutionProfileFixture });

    const body: TaskExecutionProfileSetRequest = {
      ...taskExecutionProfileFixture,
    };

    await setTaskExecutionProfile("task_001", body);

    await expectFetchRequest({
      body,
      method: "PUT",
      path: "/api/tasks/task_001/execution-profile",
    });
  });

  it("Should classify 409 conflicts on profile updates", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(
      new Response(JSON.stringify({ error: "active run" }), {
        status: 409,
        headers: { "Content-Type": "application/json" },
      })
    );

    const error = await setTaskExecutionProfile("task_001", taskExecutionProfileFixture).catch(
      err => err
    );

    expect(error).toBeInstanceOf(TasksApiError);
    expect(error).toMatchObject({ status: 409 });
  });

  it("Should DELETE a profile and resolve with no body", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 204 }));

    await deleteTaskExecutionProfile("task_001");

    await expectFetchRequest({
      method: "DELETE",
      path: "/api/tasks/task_001/execution-profile",
    });
  });
});

describe("task block adapters", () => {
  it("Should clear the exact block through the task-scoped HTTP route", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 204 }));

    await clearTaskBlock("task_001", "block_002", "Operator supplied evidence");

    await expectFetchRequest({
      body: { note: "Operator supplied evidence" },
      method: "POST",
      path: "/api/tasks/task_001/blocks/block_002/clear",
    });
  });

  it("Should identify both task and block when the scoped resource is missing", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(clearTaskBlock("task_missing", "block_missing")).rejects.toThrow(
      'Task "task_missing" or block "block_missing" was not found'
    );
  });
});

describe("review adapters", () => {
  it("Should list task run reviews with normalized filters", async () => {
    mockJsonResponse({ reviews: taskRunReviewListFixture });

    const result = await listTaskRunReviews("run_001", {
      status: "in_review",
      reviewer_session_id: "  sess_reviewer  ",
      limit: 25,
    });

    expect(result).toEqual(taskRunReviewListFixture);
    await expectFetchRequest({
      path: "/api/task-runs/run_001/reviews?status=in_review&reviewer_session_id=sess_reviewer&limit=25",
    });
  });

  it("Should list task-level reviews", async () => {
    mockJsonResponse({ reviews: taskRunReviewListFixture });

    await listTaskReviews("task_001", { status: "recorded" });

    await expectFetchRequest({
      path: "/api/tasks/task_001/reviews?status=recorded",
    });
  });

  it("Should request a run review with body and idempotent expectations", async () => {
    mockJsonResponse({ review: taskRunReviewFixture, created: true }, { status: 201 });

    const body: TaskRunReviewRequest = {
      run_id: "run_001",
      task_id: "task_001",
      review_round: 1,
      attempt: 1,
      policy: "on_success",
      reason: "Validate launch invariants",
      deadline_at: "2026-04-17T19:00:00Z",
    };

    const result = await requestTaskRunReview("run_001", body);

    expect(result.review).toEqual(taskRunReviewFixture);
    expect(result.created).toBe(true);
    await expectFetchRequest({
      body,
      method: "POST",
      path: "/api/task-runs/run_001/reviews",
    });
  });

  it("Should classify 409 conflicts on review requests", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(
      new Response(JSON.stringify({ error: "review already exists" }), {
        status: 409,
        headers: { "Content-Type": "application/json" },
      })
    );

    const error = await requestTaskRunReview("run_001", {
      run_id: "run_001",
      task_id: "task_001",
      deadline_at: "2026-04-17T19:00:00Z",
    }).catch(err => err);

    expect(error).toBeInstanceOf(TasksApiError);
    expect(error).toMatchObject({ status: 409 });
  });

  it("Should fetch a review by id", async () => {
    mockJsonResponse({ review: taskRunReviewFixture });

    const result = await getTaskRunReview("review_001");

    expect(result).toEqual(taskRunReviewFixture);
    await expectFetchRequest({ path: "/api/task-reviews/review_001" });
  });

  it("Should map review-not-found errors to TasksApiError 404", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(getTaskRunReview("missing")).rejects.toThrow("Task review not found: missing");
  });

  it("Should submit a review verdict and return continuation metadata", async () => {
    mockJsonResponse(taskRunReviewVerdictResultFixture);

    const body: TaskRunReviewVerdictRequest = {
      run_id: "run_001",
      verdict: {
        outcome: "rejected",
        reason: "missing partner-bank reconciliation evidence",
        confidence: 0.62,
        delivery_id: "delivery_review_001",
        next_round_guidance:
          "Attach the partner-bank reconciliation artifacts before the next round.",
      },
    };

    const result = await submitTaskRunReviewVerdict("review_001", body);

    expect(result).toEqual(taskRunReviewVerdictResultFixture);
    await expectFetchRequest({
      body,
      method: "POST",
      path: "/api/task-reviews/review_001/verdict",
    });
  });
});

describe("agent context adapters", () => {
  it("Should fetch the full agent context", async () => {
    mockJsonResponse({ context: agentContextFixture });

    const result = await getAgentContext();

    expect(result).toEqual(agentContextFixture);
    await expectFetchRequest({ path: "/api/agent/context" });
  });

  it("Should extract the task bundle from agent context", async () => {
    mockJsonResponse({ context: agentContextFixture });

    const result = await getTaskContextBundle();

    expect(result).toEqual(taskContextBundleFixture);
  });

  it("Should return null when the agent context has no task bundle", async () => {
    mockJsonResponse({
      context: {
        ...agentContextFixture,
        task: { ...agentContextFixture.task, bundle: undefined },
      },
    });

    const result = await getTaskContextBundle();
    expect(result).toBeNull();
  });
});

describe("bridge notification subscription adapters", () => {
  it("Should list subscriptions with normalized filters", async () => {
    mockJsonResponse({ subscriptions: taskBridgeNotificationSubscriptionsFixture });

    const result = await listTaskBridgeNotificationSubscriptions("task_001", {
      bridge_instance_id: "  bridge_instance_alpha  ",
      scope: "workspace",
      workspace_id: "ws_alpha",
      limit: 10,
    });

    expect(result).toEqual(taskBridgeNotificationSubscriptionsFixture);
    await expectFetchRequest({
      path: "/api/tasks/task_001/notifications/bridges?bridge_instance_id=bridge_instance_alpha&scope=workspace&workspace_id=ws_alpha&limit=10",
    });
  });

  it("Should create a subscription and return cursor diagnostics", async () => {
    mockJsonResponse({ subscription: taskBridgeNotificationSubscriptionFixture }, { status: 201 });

    const result = await createTaskBridgeNotificationSubscription("task_001", {
      bridge_instance_id: "bridge_instance_alpha",
      delivery_mode: "direct-send",
      scope: "workspace",
      workspace_id: "ws_alpha",
      peer_id: "peer_launch_observer",
    });

    expect(result.cursor.consumer_id).toContain("bridge_task_subscription:");
    await expectFetchRequest({
      method: "POST",
      path: "/api/tasks/task_001/notifications/bridges",
    });
  });

  it("Should fetch a subscription by id", async () => {
    mockJsonResponse({ subscription: taskBridgeNotificationSubscriptionFixture });

    const result = await getTaskBridgeNotificationSubscription("task_001", "bsub_001");

    expect(result).toEqual(taskBridgeNotificationSubscriptionFixture);
    await expectFetchRequest({
      path: "/api/tasks/task_001/notifications/bridges/bsub_001",
    });
  });

  it("Should map subscription-not-found to TasksApiError 404", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(getTaskBridgeNotificationSubscription("task_001", "missing")).rejects.toThrow(
      "Bridge notification subscription not found: missing"
    );
  });

  it("Should DELETE a subscription and resolve with no body", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 204 }));

    await deleteTaskBridgeNotificationSubscription("task_001", "bsub_001");

    await expectFetchRequest({
      method: "DELETE",
      path: "/api/tasks/task_001/notifications/bridges/bsub_001",
    });
  });
});
