// Suite: fan-out run results
// Invariant: accepted run ids retain their row while live task truth replaces provisional attribution.
// Boundary IN: fan-out response snapshots plus task/worktree query rows.
// Boundary OUT: task execution and worktree materialization, owned by the daemon.
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { buildWorktreeFixture } from "@/systems/workspace/mocks/worktree-fixtures";

import { TaskFanOutRunResults } from "../task-fan-out-run-results";
import type { FanOutTaskRunsResponse, TaskRun } from "../../types";

describe("TaskFanOutRunResults", () => {
  it("Should replace provisional attribution with the matching live run and worktree", () => {
    const accepted = {
      designation_group_id: "dg_1",
      runs: [
        {
          id: "run_1",
          designation: { brief: "Investigate checkout", index: 0 },
          resolved_worktree_mode: "per_run",
          status: "queued",
        },
      ],
    } as FanOutTaskRunsResponse;
    const live = {
      ...accepted.runs[0],
      status: "completed",
      worktree_id: "wt_checkout",
    } as TaskRun;

    render(
      <TaskFanOutRunResults
        liveRuns={[live]}
        runs={accepted.runs}
        worktrees={[
          buildWorktreeFixture({
            branch: "run/checkout-fix",
            id: "wt_checkout",
            name: "checkout-fix",
            origin: "per_run",
          }),
        ]}
      />
    );

    const row = screen
      .getByText("Investigate checkout")
      .closest('[data-slot="task-fan-out-run-result"]');
    expect(row).toHaveAttribute("data-status", "completed");
    expect(row).not.toHaveAttribute("data-unattributed");
    expect(screen.getByText("checkout-fix")).toBeInTheDocument();
    expect(screen.getByText("per-run")).toBeInTheDocument();
    expect(screen.queryByText("Worktree pending")).toBeNull();
    expect(screen.queryByText("Worktree unavailable")).toBeNull();
  });

  it("Should show an unattributed queued run as its id only", () => {
    const accepted = {
      designation_group_id: "dg_1",
      runs: [
        {
          id: "run_queued",
          designation: { brief: "Validate staging", index: 1 },
          resolved_worktree_mode: "per_run",
          status: "queued",
        },
      ],
    } as FanOutTaskRunsResponse;

    render(<TaskFanOutRunResults runs={accepted.runs} />);

    const row = screen
      .getByText("Validate staging")
      .closest('[data-slot="task-fan-out-run-result"]');
    expect(row).toHaveAttribute("data-unattributed", "");
    expect(screen.getByText("run_queued")).toBeInTheDocument();
    expect(screen.queryByText("Worktree pending")).toBeNull();
    expect(screen.queryByText("Worktree unavailable")).toBeNull();
  });

  it("Should retry a failed run through the provided action", () => {
    const onRetry = vi.fn();
    const accepted = {
      designation_group_id: "dg_1",
      runs: [
        {
          id: "run_failed",
          designation: { brief: "Backfill report", index: 2 },
          error: "Worktree setup failed.",
          resolved_worktree_mode: "per_run",
          status: "failed",
        },
      ],
    } as FanOutTaskRunsResponse;

    render(<TaskFanOutRunResults onRetry={onRetry} runs={accepted.runs} />);

    fireEvent.click(screen.getByRole("button", { name: "Retry run" }));
    expect(onRetry).toHaveBeenCalledWith("run_failed");
  });
});
