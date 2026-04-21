import type { DashboardPayload, WorkflowCard } from "../types";
import { workspaceFixture } from "@/systems/app-shell/mocks";

export const dashboardWorkflowCardFixture: WorkflowCard = {
  workflow: {
    id: "wf-alpha",
    slug: "alpha",
    workspace_id: workspaceFixture.id,
  },
  active_runs: 1,
  review_round_count: 0,
  task_completed: 4,
  task_pending: 2,
  task_total: 6,
};

export function buildDashboardFixture(overrides: Partial<DashboardPayload> = {}): DashboardPayload {
  return {
    workspace: workspaceFixture,
    daemon: {
      pid: 42,
      started_at: "2026-04-20T00:00:00Z",
      workspace_count: 1,
      active_run_count: 1,
      http_port: 2123,
      version: "0.1.12",
    },
    health: {
      ready: true,
      degraded: false,
      details: [],
    },
    pending_reviews: 0,
    queue: {
      active: 1,
      completed: 5,
      failed: 1,
      canceled: 0,
      total: 7,
    },
    workflows: [dashboardWorkflowCardFixture],
    active_runs: [],
    ...overrides,
  } as DashboardPayload;
}

export const dashboardFixture = buildDashboardFixture();

export const emptyDashboardFixture = buildDashboardFixture({
  pending_reviews: 0,
  workflows: [],
  active_runs: [],
  queue: {
    active: 0,
    completed: 0,
    failed: 0,
    canceled: 0,
    total: 0,
  },
});

export const degradedDashboardFixture = buildDashboardFixture({
  health: {
    ready: true,
    degraded: true,
    details: [
      {
        code: "sse_backlog",
        message: "SSE replay backlog is elevated.",
        severity: "warning",
      },
    ],
  } as DashboardPayload["health"],
});

export const reviewsDashboardFixture = buildDashboardFixture({
  pending_reviews: 1,
  workflows: [
    {
      ...dashboardWorkflowCardFixture,
      review_round_count: 1,
      latest_review: {
        workflow_slug: "alpha",
        round_number: 2,
        pr_ref: "PR-42",
        provider: "coderabbit",
        resolved_count: 1,
        unresolved_count: 3,
        updated_at: "2026-04-20T02:00:00Z",
      },
    } as WorkflowCard,
  ],
});
