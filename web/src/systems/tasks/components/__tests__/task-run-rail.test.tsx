import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({ children, to, params, ...props }: Record<string, unknown>) => (
      <a data-params={JSON.stringify(params)} href={typeof to === "string" ? to : "#"} {...props}>
        {children as ReactNode}
      </a>
    ),
  };
});

import { buildTaskRunDetailFixture, buildTaskRunRecordFixture } from "../../mocks/fixtures";
import { TaskRunRail } from "../task-run-rail";

type CostSummary = NonNullable<ReturnType<typeof buildTaskRunDetailFixture>["summary"]>;

function renderRail(cost: Partial<CostSummary>) {
  const base = buildTaskRunDetailFixture();
  const run = buildTaskRunDetailFixture({
    run: { ...base.run, status: "completed", ended_at: "2026-04-17T10:10:00Z" },
    summary: { ...base.summary, ...cost },
  });
  render(<TaskRunRail onInspect={vi.fn()} run={run} taskId="task_001" taskRuns={[]} />);
}

// Invariant: task-run cost presentation preserves the daemon's provenance instead of
// presenting estimates, included usage, or unknown values as measured spend.
describe("TaskRunRail cost provenance", () => {
  it("Should render actual cost as measured spend with its source", () => {
    renderRail({
      cost_status: "actual",
      cost_source: "agent_reported",
      total_cost: 0.18,
      cost_currency: "USD",
    });

    const cost = screen.getByTestId("task-run-detail-cost");
    expect(cost).toHaveTextContent("$0.180");
    expect(cost).toHaveTextContent("Reported by agent");
    expect(cost).not.toHaveTextContent("≈");
  });

  it("Should identify estimated cost with both the approximation cue and catalog source", () => {
    renderRail({
      cost_status: "estimated",
      cost_source: "catalog_config",
      total_cost: 0.18,
      cost_currency: "USD",
    });

    const cost = screen.getByTestId("task-run-detail-cost");
    expect(cost).toHaveTextContent("Est. cost");
    expect(cost).toHaveTextContent("≈ $0.180");
    expect(cost).toHaveTextContent("Estimated · Catalog rate");
  });

  it("Should render included and unavailable costs without a monetary amount", () => {
    const base = buildTaskRunDetailFixture();
    const { rerender } = render(
      <TaskRunRail
        onInspect={vi.fn()}
        run={buildTaskRunDetailFixture({
          run: { ...base.run, status: "completed", ended_at: "2026-04-17T10:10:00Z" },
          summary: {
            ...base.summary,
            cost_status: "included",
            cost_source: "none",
            total_cost: null,
          },
        })}
        taskId="task_001"
        taskRuns={[]}
      />
    );
    expect(screen.getByTestId("task-run-detail-cost")).toHaveTextContent("Included");

    rerender(
      <TaskRunRail
        onInspect={vi.fn()}
        run={buildTaskRunDetailFixture({
          run: { ...base.run, status: "completed", ended_at: "2026-04-17T10:10:00Z" },
          summary: { ...base.summary, cost_status: "unknown", total_cost: null },
        })}
        taskId="task_001"
        taskRuns={[]}
      />
    );
    const cost = screen.getByTestId("task-run-detail-cost");
    expect(cost).toHaveTextContent("Unavailable");
    expect(cost).not.toHaveTextContent("$");
  });

  it("Should omit the cost row when the daemon reports no cost provenance", () => {
    renderRail({ cost_status: undefined, cost_source: undefined, total_cost: 0.18 });
    expect(screen.queryByTestId("task-run-detail-cost")).not.toBeInTheDocument();
  });
});

describe("TaskRunRail lineage", () => {
  const currentRecord = buildTaskRunRecordFixture({
    id: "run_current",
    attempt: 2,
    previous_run_id: "run_previous",
  });
  const currentRun = buildTaskRunDetailFixture({ run: currentRecord });
  const previousRun = buildTaskRunRecordFixture({
    id: "run_previous",
    attempt: 1,
    status: "failed",
  });

  it("Should distinguish loading, error, empty, and linked states", () => {
    const { rerender } = render(
      <TaskRunRail
        onInspect={vi.fn()}
        run={currentRun}
        taskId="task_001"
        taskRuns={[]}
        taskRunsLoading
      />
    );

    expect(screen.getByRole("status")).toHaveTextContent("Loading lineage");
    expect(screen.queryByText("No linked attempts")).toBeNull();

    rerender(
      <TaskRunRail
        onInspect={vi.fn()}
        run={currentRun}
        taskId="task_001"
        taskRuns={[]}
        taskRunsErrorMessage="Lineage unavailable"
      />
    );
    expect(screen.getByRole("alert")).toHaveTextContent("Lineage unavailable");
    expect(screen.queryByText("No linked attempts")).toBeNull();

    rerender(<TaskRunRail onInspect={vi.fn()} run={currentRun} taskId="task_001" taskRuns={[]} />);
    expect(screen.getByText("No linked attempts")).toBeInTheDocument();

    rerender(
      <TaskRunRail
        onInspect={vi.fn()}
        run={currentRun}
        taskId="task_001"
        taskRuns={[previousRun, currentRun.run]}
      />
    );
    expect(screen.getByText("Attempt 1 · Failed")).toBeInTheDocument();
    expect(screen.queryByText("No linked attempts")).toBeNull();
  });
});
