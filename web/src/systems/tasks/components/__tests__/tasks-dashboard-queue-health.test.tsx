import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { TasksDashboardQueueHealth, type QueueBucket } from "../tasks-dashboard-queue-health";
import { buildDashboardFixture } from "../test-fixtures";

function buildBuckets(count: number): QueueBucket[] {
  return Array.from({ length: count }, (_unused, index) => ({
    label: `${count - index}h`,
    value: (index + 1) * 2,
    stuck: index >= count - 2,
  }));
}

describe("TasksDashboardQueueHealth", () => {
  it("Should render QueueHealthSparkline when buckets carry positive samples", () => {
    const buckets = buildBuckets(24);
    render(<TasksDashboardQueueHealth buckets={buckets} dashboard={buildDashboardFixture()} />);

    const chart = screen.getByTestId("tasks-dashboard-queue-chart");
    expect(chart).toBeInTheDocument();
    expect(chart.querySelector("[data-slot=queue-health-sparkline]")).not.toBeNull();
  });

  it("Should NOT render the deprecated 6-cell Metric sub-grid", () => {
    render(
      <TasksDashboardQueueHealth buckets={buildBuckets(24)} dashboard={buildDashboardFixture()} />
    );

    // The legacy sub-grid emitted `data-testid="tasks-dashboard-queue-total"`
    // etc. through `<Metric>` primitives. None of those testids should resolve.
    expect(screen.queryByTestId("tasks-dashboard-queue-total")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-dashboard-queue-oldest")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-dashboard-stuck-runs")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-dashboard-orphan-runs")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-dashboard-backlog-status")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-dashboard-queue-backlog")).not.toBeInTheDocument();

    const panel = screen.getByTestId("tasks-dashboard-queue-health");
    expect(panel.querySelectorAll("[data-slot=metric]").length).toBe(0);
  });

  it("Should render the Empty primitive when no buckets are available", () => {
    const dashboard = buildDashboardFixture();
    Object.assign(dashboard, {
      active_runs: { ...dashboard.active_runs, running: 0, total: 0 },
      queue: { ...dashboard.queue, total: 0 },
      totals: { ...dashboard.totals, runs_total: 0 },
    });

    render(<TasksDashboardQueueHealth buckets={[]} dashboard={dashboard} />);

    expect(screen.getByTestId("tasks-dashboard-queue-chart-empty")).toBeInTheDocument();
  });

  it("Should surface the queue warning banner when backlog_warning is true", () => {
    const dashboard = buildDashboardFixture({
      queue: {
        backlog_status: "warning",
        backlog_threshold_ms: 60000,
        backlog_warning: true,
        oldest_queue_age_ms: 180000,
        oldest_queued_at: "2026-04-17T09:57:00Z",
        total: 4,
      },
    });

    render(<TasksDashboardQueueHealth buckets={buildBuckets(24)} dashboard={dashboard} />);

    expect(screen.getByTestId("tasks-dashboard-warning")).toBeInTheDocument();
  });
});
