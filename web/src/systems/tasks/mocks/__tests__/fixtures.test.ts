// Suite: Tasks Storybook contract fixtures
// Invariant: Tasks MSW responses preserve the generated counted cursor contract after every server filter.
// Boundary IN: Tasks mock fixtures and their registered MSW handlers.
// Boundary OUT: production adapters, route loaders, and the real daemon contract suites.

import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";

import {
  storyAgentNames,
  storyCoordinatorAgentName,
  storyDefaultWorkspaceId,
  storyHeroNetworkChannel,
  storyPeople,
} from "@/storybook/fintech-scenario";
import { buildLocalNetworkParticipationFixture } from "@/test/network-participation-fixtures";
import { runIsCoordinated, taskLifecyclePhase } from "../../lib/task-formatters";
import type { TaskInboxView, TaskListPage, TaskRun } from "../../types";
import {
  agentContextFixture,
  awaitingApprovalTaskFixture,
  buildBridgeNotificationCursorFixture,
  buildTaskBridgeNotificationSubscriptionFixture,
  buildTaskContextBundleFixture,
  buildTaskExecutionProfileFixture,
  buildTaskRunFixture,
  buildTaskRunRecordFixture,
  buildTaskRunReviewFixture,
  buildTaskRunReviewVerdictResultFixture,
  coordinatorEnabledWorkspaceFixture,
  queuedCoordinatedTaskFixture,
  nonEscalatedRecoverTaskFixture,
  recoverableTaskFixture,
  savedIntentTaskFixture,
  taskBridgeNotificationSubscriptionFixture,
  taskBridgeNotificationSubscriptionsFixture,
  taskContextBundleFixture,
  taskExecutionProfileFixture,
  taskRunReviewFixture,
  taskRunReviewListFixture,
  taskRunReviewVerdictResultFixture,
  TASK_CATALOG_FIXTURES,
  TASK_FIXTURES,
} from "../fixtures";
import { handlers } from "../handlers";

const server = setupServer(...handlers);

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

describe("tasks fixtures cover the manual-first lifecycle states", () => {
  it("Should default generic run fixtures to a canonical Local snapshot", () => {
    for (const run of [buildTaskRunFixture(), buildTaskRunRecordFixture()]) {
      expect(run.resolved_network_participation).toEqual(buildLocalNetworkParticipationFixture());
    }
  });

  it("Should derive catalog participation from the active run and preserve it in run responses", async () => {
    const task = TASK_FIXTURES.find(candidate => candidate.id === "task_006");
    expect(task?.active_run).not.toBeNull();
    expect(task?.resolved_network_participation).toBe(
      task?.active_run?.resolved_network_participation
    );
    expect(task?.resolved_network_participation).toMatchObject({
      bounds: { max_wakes: 8 },
      channel_id: storyHeroNetworkChannel,
      channel_strategy: "named",
      mode: "live",
      source: "explicit_request",
      workspace_id: storyDefaultWorkspaceId,
    });

    const response = await fetch("http://localhost/api/tasks/task_006/runs");
    const body = (await response.json()) as { runs: TaskRun[] };
    expect(response.status).toBe(200);
    expect(body.runs[0]?.resolved_network_participation).toEqual(
      task?.active_run?.resolved_network_participation
    );
  });

  it("savedIntentTaskFixture is a draft with no run and resolves to publish", () => {
    expect(savedIntentTaskFixture.status).toBe("draft");
    expect(savedIntentTaskFixture.draft).toBe(true);
    expect(savedIntentTaskFixture.active_run).toBeNull();
    expect(taskLifecyclePhase(savedIntentTaskFixture)).toBe("saved_intent");
  });

  it("awaitingApprovalTaskFixture is agent-created, gated, and resolves to approve", () => {
    expect(awaitingApprovalTaskFixture.approval_policy).toBe("manual");
    expect(awaitingApprovalTaskFixture.approval_state).toBe("pending");
    expect(awaitingApprovalTaskFixture.active_run).toBeNull();
    expect(awaitingApprovalTaskFixture.created_by?.kind).toBe("agent_session");
    expect(taskLifecyclePhase(awaitingApprovalTaskFixture)).toBe("awaiting_approval");
  });

  it("queuedCoordinatedTaskFixture has a queued run bound to a coordination channel", () => {
    expect(queuedCoordinatedTaskFixture.active_run?.status).toBe("queued");
    expect(runIsCoordinated(queuedCoordinatedTaskFixture.active_run)).toBe(true);
    const participation = queuedCoordinatedTaskFixture.active_run?.resolved_network_participation;
    expect(participation?.mode).toBe("live");
    if (participation?.mode !== "live") {
      throw new Error("queued coordinated fixture must carry Live participation");
    }
    expect(participation.channel_id).toBe("coord-task-queued");
    expect(queuedCoordinatedTaskFixture.active_run).not.toHaveProperty("claim_token_hash");
    expect(queuedCoordinatedTaskFixture.active_run).not.toHaveProperty("coordination_channel");
    expect(taskLifecyclePhase(queuedCoordinatedTaskFixture)).toBe("queued");
  });

  it("recover fixtures cover escalated and non-escalated task states", () => {
    expect(recoverableTaskFixture.status).toBe("needs_attention");
    expect(taskLifecyclePhase(recoverableTaskFixture)).toBe("needs_attention");
    expect(nonEscalatedRecoverTaskFixture.status).toBe("ready");
    expect(taskLifecyclePhase(nonEscalatedRecoverTaskFixture)).toBe("ready_to_start");
  });

  it("TASK_FIXTURES still cover user-created, running, failed, and approval states", () => {
    const statuses = TASK_FIXTURES.map(task => task.status);
    const owners = TASK_FIXTURES.flatMap(task =>
      task.owner?.kind === "agent_session" ? [task.owner.ref] : []
    );
    expect(statuses).toEqual(
      expect.arrayContaining(["in_progress", "pending", "failed", "completed", "blocked", "ready"])
    );
    expect(TASK_FIXTURES.length).toBeGreaterThanOrEqual(15);
    expect(TASK_FIXTURES.some(task => task.approval_state === "pending")).toBe(true);
    expect(TASK_FIXTURES.some(task => task.active_run?.status === "running")).toBe(true);
    expect(TASK_FIXTURES.some(task => task.active_run?.status === "failed")).toBe(true);
    expect(owners).toEqual(
      expect.arrayContaining([
        storyAgentNames.product,
        storyAgentNames.frontend,
        storyAgentNames.cfo,
        storyAgentNames.marketing,
        storyAgentNames.copywriter,
      ])
    );
  });

  it("coordinatorEnabledWorkspaceFixture marks the workspace as coordinator-enabled", () => {
    expect(coordinatorEnabledWorkspaceFixture.coordinatorEnabled).toBe(true);
    expect(coordinatorEnabledWorkspaceFixture.coordinatorAgentName).toBe(storyCoordinatorAgentName);
    expect(coordinatorEnabledWorkspaceFixture.defaultChannelDisplayName).toMatch(/coordination/i);
  });
});

describe("tasks MSW handlers preserve counted query contracts", () => {
  it("Should apply every catalog filter before returning exact page metadata and facets", async () => {
    const query = new URLSearchParams({
      approval_state: "pending",
      include_drafts: "false",
      limit: "1",
      participation_channel: storyHeroNetworkChannel,
      owner_kind: "human",
      owner_ref: storyPeople.productLead,
      parent_task_id: "task_001",
      priority: "urgent",
      query: "public timeout",
      scope: "workspace",
      sort: "priority",
      status: "blocked",
      workspace: storyDefaultWorkspaceId,
    });

    const response = await fetch(`http://localhost/api/tasks?${query}`);
    const body: TaskListPage = await response.json();

    expect(response.status).toBe(200);
    expect(body.tasks.map(task => task.id)).toEqual(["task_006"]);
    expect(body.page).toEqual({ has_more: false, limit: 1, total: 1 });
    expect(body.facets).toEqual({
      owners: [{ count: 1, owner: { kind: "human", ref: storyPeople.productLead } }],
      statuses: [{ count: 1, status: "blocked" }],
    });

    const exclusions: [string, string][] = [
      ["approval_state", "approved"],
      ["participation_channel", "unrelated-channel"],
      ["owner_kind", "agent_session"],
      ["owner_ref", "another-owner"],
      ["parent_task_id", "task_other"],
      ["priority", "low"],
      ["query", "not present"],
      ["scope", "global"],
      ["status", "ready"],
      ["workspace", "workspace_other"],
    ];
    for (const [name, value] of exclusions) {
      const excludedQuery = new URLSearchParams(query);
      excludedQuery.set(name, value);
      const excludedResponse = await fetch(`http://localhost/api/tasks?${excludedQuery}`);
      const excluded: TaskListPage = await excludedResponse.json();
      expect(excluded.page.total, `${name} must exclude the fixture`).toBe(0);
      expect(excluded.tasks, `${name} must not leak a row`).toEqual([]);
    }
  });

  it("Should sort the complete catalog before cursor paging without duplicating rows", async () => {
    const firstResponse = await fetch(
      "http://localhost/api/tasks?scope=all&include_drafts=true&sort=priority&limit=2"
    );
    const first: TaskListPage = await firstResponse.json();

    expect(first.tasks.map(task => task.id)).toEqual(["task_006", "task_004"]);
    expect(first.page.total).toBe(TASK_CATALOG_FIXTURES.length);
    expect(first.page.has_more).toBe(true);
    expect(first.page.next_cursor).toBeTypeOf("string");

    const secondResponse = await fetch(
      `http://localhost/api/tasks?scope=all&include_drafts=true&sort=priority&limit=2&cursor=${encodeURIComponent(first.page.next_cursor ?? "")}`
    );
    const second: TaskListPage = await secondResponse.json();
    const firstIds = new Set(first.tasks.map(task => task.id));

    expect(second.tasks).toHaveLength(2);
    expect(second.tasks.every(task => !firstIds.has(task.id))).toBe(true);
    expect(second.page.total).toBe(first.page.total);
    expect(second.facets).toEqual(first.facets);
  });

  it("Should exclude drafts by default and include them only when requested", async () => {
    const defaultResponse = await fetch("http://localhost/api/tasks?scope=all&sort=recent");
    const defaultPage: TaskListPage = await defaultResponse.json();
    const draftResponse = await fetch(
      "http://localhost/api/tasks?scope=all&include_drafts=true&sort=recent"
    );
    const withDrafts: TaskListPage = await draftResponse.json();

    expect(defaultPage.tasks.some(task => task.status === "draft")).toBe(false);
    expect(withDrafts.tasks.some(task => task.status === "draft")).toBe(true);
    expect(withDrafts.page.total).toBe(defaultPage.page.total + 1);
  });

  it("Should keep exact inbox metadata stable while cursor pages distribute items by lane", async () => {
    const firstResponse = await fetch("http://localhost/api/observe/tasks/inbox?limit=2");
    const firstBody: { inbox: TaskInboxView } = await firstResponse.json();
    const first = firstBody.inbox;
    const firstIds = new Set(
      first.groups.flatMap(group => (group.items ?? []).map(item => item.task.id))
    );

    expect(first.page).toEqual({
      has_more: true,
      limit: 2,
      next_cursor: expect.any(String),
      total: 5,
    });
    expect(first.unread_total).toBe(4);
    expect(first.archived_total).toBe(1);
    expect(first.groups.map(group => [group.lane, group.count, group.unread_count])).toEqual([
      ["my_work", 2, 2],
      ["approvals", 1, 1],
      ["failed_runs", 1, 1],
      ["blocked", 0, 0],
      ["archived", 1, 0],
    ]);
    expect(firstIds).toEqual(new Set(["task_001", "task_006"]));

    const secondResponse = await fetch(
      `http://localhost/api/observe/tasks/inbox?limit=2&cursor=${encodeURIComponent(first.page.next_cursor ?? "")}`
    );
    const secondBody: { inbox: TaskInboxView } = await secondResponse.json();
    const second = secondBody.inbox;
    const secondIds = second.groups.flatMap(group => (group.items ?? []).map(item => item.task.id));

    expect(secondIds).toEqual(expect.arrayContaining(["task_002", "task_004"]));
    expect(secondIds.every(id => !firstIds.has(id))).toBe(true);
    expect(second.page.total).toBe(first.page.total);
    expect(second.facets).toEqual(first.facets);
    expect(second.groups.map(group => group.count)).toEqual(first.groups.map(group => group.count));
  });

  it("Should apply actor, lane, task, unread, scope, and search inbox filters before counting", async () => {
    const query = new URLSearchParams({
      lane: "approvals",
      limit: "1",
      owner_kind: "human",
      owner_ref: storyPeople.productLead,
      priority: "urgent",
      query: "public timeout",
      scope: "workspace",
      status: "blocked",
      unread: "true",
      workspace: storyDefaultWorkspaceId,
    });

    const response = await fetch(`http://localhost/api/observe/tasks/inbox?${query}`);
    const body: { inbox: TaskInboxView } = await response.json();
    const inbox = body.inbox;

    expect(inbox.page).toEqual({ has_more: false, limit: 1, total: 1 });
    expect(inbox.groups).toHaveLength(1);
    expect(inbox.groups[0]?.lane).toBe("approvals");
    expect(inbox.groups[0]?.items?.map(item => item.task.id)).toEqual(["task_006"]);
    expect(inbox.facets).toEqual({
      priorities: [{ count: 1, priority: "urgent" }],
      statuses: [{ count: 1, status: "blocked" }],
    });

    const exclusions: [string, string][] = [
      ["lane", "failed_runs"],
      ["owner_kind", "agent_session"],
      ["owner_ref", "another-owner"],
      ["priority", "low"],
      ["query", "not present"],
      ["scope", "global"],
      ["status", "ready"],
      ["unread", "false"],
      ["workspace", "workspace_other"],
    ];
    for (const [name, value] of exclusions) {
      const excludedQuery = new URLSearchParams(query);
      excludedQuery.set(name, value);
      const excludedResponse = await fetch(
        `http://localhost/api/observe/tasks/inbox?${excludedQuery}`
      );
      const excludedBody: { inbox: TaskInboxView } = await excludedResponse.json();
      expect(excludedBody.inbox.page.total, `${name} must exclude the fixture`).toBe(0);
      expect(
        excludedBody.inbox.groups.flatMap(group => group.items ?? []),
        `${name} must not leak an inbox row`
      ).toEqual([]);
    }
  });
});

describe("orchestration fixtures satisfy generated contract shape", () => {
  it("Should expose all execution profile selector branches", () => {
    expect(taskExecutionProfileFixture.task_id).toBeTypeOf("string");
    expect(taskExecutionProfileFixture.coordinator.mode).toMatch(/inherit|guided/);
    expect(taskExecutionProfileFixture.worker.mode).toMatch(/inherit|select/);
    expect(taskExecutionProfileFixture.sandbox.mode).toMatch(/inherit|none|ref/);
    expect(taskExecutionProfileFixture.created_at).toBeTypeOf("string");
    expect(taskExecutionProfileFixture.updated_at).toBeTypeOf("string");

    const overlay = buildTaskExecutionProfileFixture({ task_id: "task_42" });
    expect(overlay.task_id).toBe("task_42");
  });

  it("Should expose review fixtures with status, policy, and cursor diagnostics", () => {
    expect(taskRunReviewFixture.review_id).toBeTypeOf("string");
    expect(taskRunReviewFixture.status).toBe("in_review");
    expect(taskRunReviewListFixture).toHaveLength(2);
    expect(taskRunReviewListFixture[1]?.outcome).toBe("rejected");
    const built = buildTaskRunReviewFixture({ status: "recorded", outcome: "approved" });
    expect(built.status).toBe("recorded");
    expect(built.outcome).toBe("approved");
  });

  it("Should expose verdict fixture with continuation run lineage", () => {
    expect(taskRunReviewVerdictResultFixture.review.outcome).toBe("rejected");
    expect(taskRunReviewVerdictResultFixture.continuation_run?.attempt).toBeGreaterThan(0);
    expect(taskRunReviewVerdictResultFixture.continuation_run?.task_id).toBe(
      taskRunReviewVerdictResultFixture.review.task_id
    );
    const reset = buildTaskRunReviewVerdictResultFixture({ circuit_opened: true });
    expect(reset.circuit_opened).toBe(true);
  });

  it("Should expose bridge subscription fixtures with cursor diagnostics", () => {
    expect(taskBridgeNotificationSubscriptionFixture.cursor.consumer_id).toContain(
      "bridge_task_subscription:"
    );
    expect(taskBridgeNotificationSubscriptionFixture.cursor.stream_name).toBe("task_events");
    expect(taskBridgeNotificationSubscriptionFixture.cursor.last_sequence).toBeGreaterThanOrEqual(
      0
    );
    expect(taskBridgeNotificationSubscriptionsFixture).toHaveLength(2);

    const fresh = buildBridgeNotificationCursorFixture({ last_sequence: 0 });
    expect(fresh.last_sequence).toBe(0);

    const customSub = buildTaskBridgeNotificationSubscriptionFixture({
      subscription_id: "bsub_custom",
    });
    expect(customSub.cursor.consumer_id).toBe("bridge_task_subscription:bsub_custom");
  });

  it("Should expose task context bundle with latest_event_seq and execution profile", () => {
    expect(taskContextBundleFixture.latest_event_seq).toBeGreaterThanOrEqual(0);
    expect(taskContextBundleFixture.task.id).toBeTypeOf("string");
    expect(taskContextBundleFixture.execution_profile?.coordinator.mode).toMatch(/inherit|guided/);
    const variant = buildTaskContextBundleFixture({ latest_event_seq: 99 });
    expect(variant.latest_event_seq).toBe(99);
  });

  it("Should expose agent context fixture pointing at the task bundle", () => {
    expect(agentContextFixture.task.available).toBe(true);
    expect(agentContextFixture.task.bundle?.task.id).toBe("task_001");
    expect(agentContextFixture.task.bundle?.latest_event_seq).toBeGreaterThanOrEqual(0);
  });
});
