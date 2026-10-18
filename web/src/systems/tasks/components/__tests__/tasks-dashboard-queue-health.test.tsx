import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { TasksDashboardQueueHealth } from "../tasks-dashboard-queue-health";
import { buildDashboardFixture } from "../test-fixtures";

describe("TasksDashboardQueueHealth", () => {
  // Regression: the panel used to derive 24 equal "hourly" buckets from
  // runs_total / 24, which the sparkline scaled to 24 full-height bars — a
  // queue-depth history the read model never reported.
  it("Should report the current queue snapshot and never draw an invented history", () => {
    const dashboard = buildDashboardFixture();
    Object.assign(dashboard, { totals: { ...dashboard.totals, runs_total: 11 } });

    render(<TasksDashboardQueueHealth dashboard={dashboard} />);

    const panel = screen.getByTestId("tasks-dashboard-queue-health");
    expect(panel.querySelector("[data-slot=queue-health-sparkline]")).toBeNull();
    expect(panel).not.toHaveTextContent("24h");
    expect(screen.getByTestId("tasks-dashboard-queue-depth")).toHaveTextContent("1");
    expect(screen.getByTestId("tasks-dashboard-queue-oldest-wait")).toHaveTextContent("42s");
  });

  it("Should not name an oldest wait when nothing is waiting", () => {
    const dashboard = buildDashboardFixture();
    Object.assign(dashboard, { queue: { ...dashboard.queue, total: 0, oldest_queue_age_ms: 0 } });

    render(<TasksDashboardQueueHealth dashboard={dashboard} />);

    expect(screen.getByTestId("tasks-dashboard-queue-depth")).toHaveTextContent("0");
    expect(screen.getByTestId("tasks-dashboard-queue-oldest-wait")).toHaveTextContent("—");
    expect(screen.getByTestId("tasks-dashboard-ok")).toHaveTextContent("Queue is healthy.");
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

    render(<TasksDashboardQueueHealth dashboard={dashboard} />);

    expect(screen.getByTestId("tasks-dashboard-warning")).toBeInTheDocument();
  });
});
