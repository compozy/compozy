import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, ...rest }: { children: ReactNode } & Record<string, unknown>) => {
    const { params: _params, to: _to, ...domRest } = rest as Record<string, unknown>;
    return <a {...domRest}>{children}</a>;
  },
}));

import { TasksDashboardView } from "../tasks-dashboard-view";
import { buildDashboardFixture } from "../test-fixtures";

describe("TasksDashboardView", () => {
  it("renders loading state when no dashboard is available", () => {
    render(<TasksDashboardView dashboard={null} dashboardStatus="loading" />);
    expect(screen.getByTestId("tasks-dashboard-loading")).toBeInTheDocument();
  });

  it("renders error state with message when no dashboard is available", () => {
    render(<TasksDashboardView dashboard={null} errorMessage="kaboom" />);
    expect(screen.getByTestId("tasks-dashboard-error")).toHaveTextContent("kaboom");
  });

  it("renders empty state when no dashboard data is available and not loading", () => {
    render(<TasksDashboardView dashboard={null} />);
    expect(screen.getByTestId("tasks-dashboard-empty")).toBeInTheDocument();
  });

  it("renders dashboard sections for a populated dashboard", () => {
    render(<TasksDashboardView dashboard={buildDashboardFixture()} />);

    expect(screen.getByTestId("tasks-dashboard-view")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-dashboard-cards")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-dashboard-queue-health")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-dashboard-status-breakdown")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-dashboard-active-runs")).toBeInTheDocument();
    // Live/stale pill is deferred — only the static totals
    // eyebrow renders at the bottom of the dashboard.
    expect(screen.queryByTestId("tasks-dashboard-freshness")).not.toBeInTheDocument();
    expect(screen.getByTestId("tasks-dashboard-totals")).toHaveTextContent(/runs/i);
  });

  it("surfaces the queue warning when backlog_warning is true", () => {
    const dashboard = buildDashboardFixture({
      queue: {
        backlog_status: "warning",
        backlog_threshold_ms: 60_000,
        backlog_warning: true,
        oldest_queue_age_ms: 180_000,
        oldest_queued_at: "2026-04-17T09:57:00Z",
        total: 5,
      },
      health: {
        active_orphan_runs: 0,
        queue_backlog: true,
        status: "warning",
        stuck_runs: 0,
      },
    });

    render(<TasksDashboardView dashboard={dashboard} />);
    expect(screen.getByTestId("tasks-dashboard-warning")).toBeInTheDocument();
    // 6-cell Metric sub-grid was deleted; queue total is exposed
    // through the KPI strip (`tasks-dashboard-card-queue-depth`) instead.
    expect(screen.getByTestId("tasks-dashboard-card-queue-depth")).toHaveTextContent("5");
  });

  it("renders active runs with identifier, link, and stuck badge", () => {
    const dashboard = buildDashboardFixture({
      active_runs: {
        claimed: 0,
        queued: 0,
        running: 1,
        starting: 0,
        total: 1,
        items: [
          {
            age_ms: 600_000,
            attempt: 2,
            error: "boom",
            health_status: "warning",
            last_activity_at: "2026-04-17T10:00:00Z",
            max_attempts: 3,
            run_id: "run_xyz",
            run_status: "running",
            scope: "workspace",
            stuck: true,
            task_id: "task_xyz",
            task_identifier: "TASK-42",
            task_status: "in_progress",
            task_title: "Stuck task",
          },
        ],
      },
    });

    render(<TasksDashboardView dashboard={dashboard} />);
    expect(screen.getByTestId("tasks-dashboard-active-run-run_xyz")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-dashboard-active-run-stuck-run_xyz")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-dashboard-active-run-error-run_xyz")).toHaveTextContent(
      "boom"
    );
    expect(screen.getByTestId("tasks-dashboard-active-run-link-run_xyz")).toBeInTheDocument();
  });
});
