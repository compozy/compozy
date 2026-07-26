import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { TasksDashboardCards } from "../tasks-dashboard-cards";
import { buildDashboardFixture } from "../test-fixtures";

describe("TasksDashboardCards", () => {
  it("Should render four Metric primitives labeled Active runs, Success rate, Average duration, Queue depth", () => {
    render(<TasksDashboardCards dashboard={buildDashboardFixture()} />);

    const container = screen.getByTestId("tasks-dashboard-cards");
    const cards = container.querySelectorAll("[data-slot=metric]");
    expect(cards).toHaveLength(4);

    expect(screen.getByTestId("tasks-dashboard-card-active-runs")).toHaveTextContent(
      /Active runs/i
    );
    expect(screen.getByTestId("tasks-dashboard-card-success-rate")).toHaveTextContent(
      /Success rate/i
    );
    expect(screen.getByTestId("tasks-dashboard-card-average-duration")).toHaveTextContent(
      /Average duration/i
    );
    expect(screen.getByTestId("tasks-dashboard-card-queue-depth")).toHaveTextContent(
      /Queue depth/i
    );
  });

  it("Should show the active run count and queue depth from the dashboard payload", () => {
    const dashboard = buildDashboardFixture({
      active_runs: {
        claimed: 2,
        queued: 3,
        running: 5,
        starting: 0,
        total: 10,
        items: [],
      },
      queue: {
        backlog_status: "ok",
        backlog_threshold_ms: 60000,
        backlog_warning: false,
        oldest_queue_age_ms: 0,
        oldest_queued_at: "2026-04-17T10:00:00Z",
        total: 7,
      },
    });

    render(<TasksDashboardCards dashboard={dashboard} />);

    const active = screen.getByTestId("tasks-dashboard-card-active-runs");
    expect(within(active).getByText("5")).toBeInTheDocument();
    const queue = screen.getByTestId("tasks-dashboard-card-queue-depth");
    expect(within(queue).getByText("7")).toBeInTheDocument();
  });

  it("Should compute success rate from completed vs. terminal runs", () => {
    const dashboard = buildDashboardFixture({
      totals: buildDashboardFixture().totals,
    });
    Object.assign(dashboard.totals, {
      completed_runs: 9,
      failed_runs: 1,
      canceled_runs: 0,
    });

    render(<TasksDashboardCards dashboard={dashboard} />);
    const successCard = screen.getByTestId("tasks-dashboard-card-success-rate");
    expect(successCard).toHaveTextContent(/90%/);
  });

  it("Should render an em-dash for success rate when no runs have concluded", () => {
    const dashboard = buildDashboardFixture();
    Object.assign(dashboard.totals, {
      completed_runs: 0,
      failed_runs: 0,
      canceled_runs: 0,
    });

    render(<TasksDashboardCards dashboard={dashboard} />);
    expect(screen.getByTestId("tasks-dashboard-card-success-rate")).toHaveTextContent("--");
  });
});
