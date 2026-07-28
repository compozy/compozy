import { describe, expect, it } from "vitest";

import { tasksKeys } from "../query-keys";

describe("tasksKeys", () => {
  it("namespaces all keys under tasks", () => {
    expect(tasksKeys.all).toEqual(["tasks"]);
    expect(tasksKeys.lists()).toEqual(["tasks", "list"]);
    expect(tasksKeys.details()).toEqual(["tasks", "detail"]);
    expect(tasksKeys.runsRoot()).toEqual(["tasks", "runs"]);
    expect(tasksKeys.timelineRoot()).toEqual(["tasks", "timeline"]);
    expect(tasksKeys.treeRoot()).toEqual(["tasks", "tree"]);
    expect(tasksKeys.runDetails()).toEqual(["tasks", "run-detail"]);
    expect(tasksKeys.triageRoot()).toEqual(["tasks", "triage"]);
  });

  it("produces stable list keys from filter input", () => {
    expect(
      tasksKeys.list({
        scope: "workspace",
        workspace: "ws_alpha",
        status: "ready",
        priority: "high",
        include_drafts: true,
        approval_state: "pending",
        owner_kind: "human",
        owner_ref: "op",
        parent_task_id: "task_parent",
        participation_channel: "net",
        query: "review",
        sort: "priority",
        cursor: "ignored-cursor",
        limit: 50,
      })
    ).toEqual([
      "tasks",
      "list",
      "workspace",
      "ws_alpha",
      "ready",
      "high",
      "1",
      "pending",
      "human",
      "op",
      "task_parent",
      "net",
      "review",
      "priority",
      "50",
    ]);

    expect(tasksKeys.list()).toEqual([
      "tasks",
      "list",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
      "",
    ]);

    expect(tasksKeys.list({ scope: "workspace", sort: "recent", cursor: "first" })).toEqual(
      tasksKeys.list({ scope: "workspace", sort: "recent", cursor: "second" })
    );
    expect(tasksKeys.list({ sort: "recent" })).not.toEqual(tasksKeys.list({ sort: "priority" }));
  });

  it("distinguishes detail, run, timeline, tree, and run-detail keys by id", () => {
    expect(tasksKeys.detail("task_1")).toEqual(["tasks", "detail", "task_1"]);
    expect(tasksKeys.runs("task_1", { status: "running", limit: 5 })).toEqual([
      "tasks",
      "runs",
      "task_1",
      "running",
      "",
      "5",
    ]);
    expect(tasksKeys.timeline("task_1", { after_sequence: 12, limit: 50 })).toEqual([
      "tasks",
      "timeline",
      "task_1",
      "12",
      "50",
    ]);
    expect(tasksKeys.tree("task_1")).toEqual(["tasks", "tree", "task_1"]);
    expect(tasksKeys.runDetail("run_1")).toEqual(["tasks", "run-detail", "run_1"]);
  });

  it("serializes dashboard and inbox filters stably", () => {
    expect(tasksKeys.dashboard({ scope: "workspace", workspace: "ws_alpha" })).toEqual([
      "tasks",
      "dashboard",
      "workspace",
      "ws_alpha",
      "",
      "",
      "",
      "",
    ]);

    expect(
      tasksKeys.inbox({
        scope: "workspace",
        workspace: "ws_alpha",
        lane: "approvals",
        status: "ready",
        priority: "urgent",
        unread: true,
        cursor: "ignored-cursor",
        limit: 20,
      })
    ).toEqual([
      "tasks",
      "inbox",
      "workspace",
      "ws_alpha",
      "",
      "",
      "approvals",
      "ready",
      "urgent",
      "1",
      "",
      "20",
    ]);
    expect(tasksKeys.inbox({ lane: "approvals", cursor: "first" })).toEqual(
      tasksKeys.inbox({ lane: "approvals", cursor: "second" })
    );
  });

  it("Should namespace orchestration roots", () => {
    expect(tasksKeys.profilesRoot()).toEqual(["tasks", "profile"]);
    expect(tasksKeys.reviewsRoot()).toEqual(["tasks", "reviews"]);
    expect(tasksKeys.streamsRoot()).toEqual(["tasks", "stream"]);
    expect(tasksKeys.bridgeNotificationsRoot()).toEqual(["tasks", "bridge-notifications"]);
    expect(tasksKeys.agentContext()).toEqual(["tasks", "agent-context"]);
    expect(tasksKeys.contextBundle()).toEqual(["tasks", "context-bundle"]);
  });

  it("Should bind profile keys to a task id", () => {
    expect(tasksKeys.profile("task_1")).toEqual(["tasks", "profile", "task_1"]);
  });

  it("Should distinguish run, task, and detail review keys", () => {
    expect(
      tasksKeys.reviewsByRun("run_1", {
        status: "in_review",
        reviewer_session_id: "sess_a",
        limit: 5,
      })
    ).toEqual(["tasks", "reviews", "run", "run_1", "in_review", "sess_a", "5"]);

    expect(tasksKeys.reviewsByTask("task_1", { status: "recorded" })).toEqual([
      "tasks",
      "reviews",
      "task",
      "task_1",
      "recorded",
      "",
      "",
    ]);

    expect(tasksKeys.reviewDetail("review_1")).toEqual(["tasks", "reviews", "detail", "review_1"]);
  });

  it("Should encode stream resume cursor", () => {
    expect(tasksKeys.stream("task_1", { after_sequence: 12 })).toEqual([
      "tasks",
      "stream",
      "task_1",
      "12",
    ]);
    expect(tasksKeys.stream("task_1")).toEqual(["tasks", "stream", "task_1", ""]);
  });

  it("Should serialize bridge notification filters stably", () => {
    expect(
      tasksKeys.bridgeNotifications("task_1", {
        bridge_instance_id: "bridge_alpha",
        scope: "workspace",
        workspace_id: "ws_alpha",
        limit: 10,
      })
    ).toEqual([
      "tasks",
      "bridge-notifications",
      "task_1",
      "bridge_alpha",
      "workspace",
      "ws_alpha",
      "10",
    ]);

    expect(tasksKeys.bridgeNotification("task_1", "bsub_1")).toEqual([
      "tasks",
      "bridge-notifications",
      "task_1",
      "detail",
      "bsub_1",
    ]);
  });
});
