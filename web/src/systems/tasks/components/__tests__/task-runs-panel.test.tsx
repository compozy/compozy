import { fireEvent, render, screen, within } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({ to, params, children, ...props }: Record<string, unknown>) => (
      <a
        data-params={JSON.stringify(params)}
        href={typeof to === "string" ? to : "#"}
        {...(props as Record<string, unknown>)}
      >
        {children as ReactNode}
      </a>
    ),
  };
});

import { buildTaskRunRecordFixture, buildTaskRunReviewFixture } from "../../mocks/fixtures";
import { TaskRunsPanel } from "../task-runs-panel";

describe("TaskRunsPanel", () => {
  it("Should render newest attempts first with lineage and review summary", () => {
    const first = buildTaskRunRecordFixture({ id: "run_1", attempt: 1, status: "failed" });
    const second = buildTaskRunRecordFixture({
      id: "run_2",
      attempt: 2,
      previous_run_id: "run_1",
      status: "running",
    });
    const reviews = new Map([
      [
        "run_2",
        [
          buildTaskRunReviewFixture({
            outcome: "approved",
            reason: "All checks passed",
            run_id: "run_2",
          }),
        ],
      ],
    ]);

    render(<TaskRunsPanel reviewsByRun={reviews} runs={[first, second]} taskId="task_001" />);

    const rows = screen.getAllByTestId(/tasks-runs-row-/);
    expect(rows[0]).toHaveAttribute("data-testid", "tasks-runs-row-run_2");
    expect(within(rows[0]).getByText("retried from attempt 1")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-runs-review-review_001")).toHaveTextContent(
      "Review: approved · All checks passed"
    );
  });

  it("Should preserve rows while exposing background loading and error state", () => {
    render(
      <TaskRunsPanel
        errorMessage="Refresh failed"
        isLoading
        runs={[buildTaskRunRecordFixture()]}
        taskId="task_001"
      />
    );

    expect(screen.getByTestId("tasks-runs-panel")).toHaveAttribute("aria-busy", "true");
    expect(screen.getByRole("alert")).toHaveTextContent("Refresh failed");
    expect(screen.getByTestId("tasks-runs-row-run_001")).toBeInTheDocument();
  });

  it("Should expose pending state on the empty Start run action", () => {
    const onStartRun = vi.fn();
    render(<TaskRunsPanel isStartPending onStartRun={onStartRun} runs={[]} taskId="task_001" />);

    const start = screen.getByTestId("tasks-runs-start");
    expect(start).toBeDisabled();
    expect(start).toHaveAttribute("aria-busy", "true");
    expect(start).toHaveTextContent("Starting…");
    fireEvent.click(start);
    expect(onStartRun).not.toHaveBeenCalled();
  });
});
