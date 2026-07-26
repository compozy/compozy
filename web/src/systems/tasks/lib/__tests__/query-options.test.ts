import { describe, expect, it } from "vitest";

import {
  agentContextOptions,
  taskBridgeNotificationSubscriptionOptions,
  taskBridgeNotificationSubscriptionsOptions,
  taskContextBundleOptions,
  taskDashboardOptions,
  taskDetailOptions,
  taskExecutionProfileOptions,
  taskInboxBadgeOptions,
  taskInboxOptions,
  taskReviewsOptions,
  taskRunDetailOptions,
  taskRunReviewDetailOptions,
  taskRunReviewsOptions,
  taskRunsOptions,
  taskTimelineOptions,
  taskTreeOptions,
  tasksListOptions,
} from "../query-options";

describe("tasks list options", () => {
  it("uses infinite cursor pages without automatic catalog polling", () => {
    const options = tasksListOptions();

    expect(options.staleTime).toBe(15_000);
    expect(options.refetchInterval).toBeUndefined();
    expect(options.initialPageParam).toBeUndefined();
    const page = {
      facets: { owners: [], statuses: [] },
      page: { has_more: true, limit: 1, next_cursor: "tasks-next", total: 2 },
      tasks: [],
    };
    expect(options.getNextPageParam?.(page, [page], undefined, [undefined])).toBe("tasks-next");
    expect(options.enabled).toBe(true);
  });

  it("carries filters into the query key", () => {
    const options = tasksListOptions({
      scope: "workspace",
      workspace: "ws_alpha",
      status: "ready",
      limit: 20,
    });

    expect(options.queryKey).toContain("workspace");
    expect(options.queryKey).toContain("ws_alpha");
    expect(options.queryKey).toContain("ready");
    expect(options.queryKey).toContain("20");
  });

  it("supports an explicit disabled state for the list query", () => {
    expect(tasksListOptions({}, false).enabled).toBe(false);
  });
});

describe("tasks detail and run options", () => {
  it("disables detail queries for empty ids", () => {
    expect(taskDetailOptions("").enabled).toBe(false);
    expect(taskDetailOptions("task_1", false).enabled).toBe(false);
    expect(taskDetailOptions("task_1").enabled).toBe(true);
  });

  it("uses the live cadence for runs, timeline, tree, and run detail", () => {
    expect(taskRunsOptions("task_1").refetchInterval).toBe(15_000);
    expect(taskTimelineOptions("task_1").refetchInterval).toBe(15_000);
    expect(taskTimelineOptions("task_1").staleTime).toBe(5_000);
    expect(taskTreeOptions("task_1").refetchInterval).toBe(15_000);
    expect(taskRunDetailOptions("run_1").refetchInterval).toBe(15_000);
  });

  it("disables live queries when ids are missing", () => {
    expect(taskRunsOptions("").enabled).toBe(false);
    expect(taskTimelineOptions("").enabled).toBe(false);
    expect(taskTreeOptions("").enabled).toBe(false);
    expect(taskRunDetailOptions("").enabled).toBe(false);
  });

  it("carries timeline filters into the query key", () => {
    const options = taskTimelineOptions("task_1", { after_sequence: 12, limit: 30 });

    expect(options.queryKey).toEqual(["tasks", "timeline", "task_1", "12", "30"]);
  });
});

describe("tasks dashboard and inbox options", () => {
  it("keeps dashboard polling separate from infinite inbox pages", () => {
    const dashboardOptions = taskDashboardOptions({ scope: "workspace" });
    const inboxOptions = taskInboxOptions({ lane: "approvals" });
    const inboxBadgeOptions = taskInboxBadgeOptions({ limit: 1 });

    expect(dashboardOptions.staleTime).toBe(15_000);
    expect(dashboardOptions.refetchInterval).toBe(30_000);
    expect(inboxOptions.staleTime).toBe(0);
    expect(inboxOptions.refetchInterval).toBeUndefined();
    expect(inboxBadgeOptions.staleTime).toBe(15_000);
    expect(inboxOptions.initialPageParam).toBeUndefined();
    const page = {
      archived_total: 0,
      facets: { priorities: [], statuses: [] },
      groups: [],
      page: { has_more: true, limit: 1, next_cursor: "inbox-next", total: 2 },
      unread_total: 0,
    };
    expect(inboxOptions.getNextPageParam?.(page, [page], undefined, [undefined])).toBe(
      "inbox-next"
    );
    expect(dashboardOptions.queryKey).toContain("workspace");
    expect(inboxOptions.queryKey).toContain("approvals");
  });

  it("supports explicit disabled state for aggregate reads", () => {
    expect(taskDashboardOptions({}, false).enabled).toBe(false);
    expect(taskInboxOptions({}, false).enabled).toBe(false);
  });
});

describe("orchestration options", () => {
  it("Should disable profile and review queries for empty ids", () => {
    expect(taskExecutionProfileOptions("").enabled).toBe(false);
    expect(taskRunReviewsOptions("").enabled).toBe(false);
    expect(taskReviewsOptions("").enabled).toBe(false);
    expect(taskRunReviewDetailOptions("").enabled).toBe(false);
  });

  it("Should use the default cadence for execution profile reads", () => {
    const options = taskExecutionProfileOptions("task_1");
    expect(options.staleTime).toBe(15_000);
    expect(options.refetchInterval).toBe(30_000);
  });

  it("Should use the live cadence for review reads", () => {
    expect(taskRunReviewsOptions("run_1").refetchInterval).toBe(15_000);
    expect(taskRunReviewsOptions("run_1").staleTime).toBe(5_000);
    expect(taskReviewsOptions("task_1").refetchInterval).toBe(15_000);
    expect(taskRunReviewDetailOptions("review_1").refetchInterval).toBe(15_000);
  });

  it("Should carry review filters into the query key", () => {
    const options = taskRunReviewsOptions("run_1", {
      status: "in_review",
      reviewer_session_id: "sess_a",
      limit: 25,
    });
    expect(options.queryKey).toEqual([
      "tasks",
      "reviews",
      "run",
      "run_1",
      "in_review",
      "sess_a",
      "25",
    ]);
  });

  it("Should expose enabled toggle for context queries", () => {
    expect(agentContextOptions(false).enabled).toBe(false);
    expect(agentContextOptions().enabled).toBe(true);
    expect(taskContextBundleOptions(false).enabled).toBe(false);
    expect(taskContextBundleOptions().enabled).toBe(true);
  });

  it("Should disable bridge notification queries when ids are missing", () => {
    expect(taskBridgeNotificationSubscriptionsOptions("").enabled).toBe(false);
    expect(taskBridgeNotificationSubscriptionOptions("", "bsub_1").enabled).toBe(false);
    expect(taskBridgeNotificationSubscriptionOptions("task_1", "").enabled).toBe(false);
    expect(taskBridgeNotificationSubscriptionOptions("task_1", "bsub_1").enabled).toBe(true);
  });
});
