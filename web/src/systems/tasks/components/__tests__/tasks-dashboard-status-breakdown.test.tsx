import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { TasksDashboardStatusBreakdown } from "../tasks-dashboard-status-breakdown";
import { buildDashboardFixture } from "../test-fixtures";

describe("TasksDashboardStatusBreakdown", () => {
  it("renders one row per status with its count, and the row sum matches the total badge", () => {
    const dashboard = buildDashboardFixture({
      status_breakdown: [
        { count: 5, share_percent: 50, status: "completed" },
        { count: 3, share_percent: 30, status: "in_progress" },
        { count: 2, share_percent: 20, status: "blocked" },
      ],
    });

    render(<TasksDashboardStatusBreakdown dashboard={dashboard} />);

    const rows = screen.getAllByTestId(/tasks-dashboard-status-row-/);
    expect(rows).toHaveLength(3);

    const counts = rows.map(row => {
      const count = within(row).getByTestId(/tasks-dashboard-status-count-/);
      return Number(count.textContent);
    });
    const sum = counts.reduce((acc, value) => acc + value, 0);
    expect(sum).toBe(10);
    expect(screen.getByTestId("tasks-dashboard-status-breakdown-total")).toHaveTextContent("10");
  });

  it("shows an empty state when no status rows are provided", () => {
    render(
      <TasksDashboardStatusBreakdown dashboard={buildDashboardFixture({ status_breakdown: [] })} />
    );

    expect(screen.getByTestId("tasks-dashboard-status-breakdown-empty")).toBeInTheDocument();
  });
});
