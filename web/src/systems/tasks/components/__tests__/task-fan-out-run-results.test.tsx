// Suite: fan-out run results
// Invariant: accepted run ids retain their row while live task truth replaces provisional attribution.
// Boundary IN: fan-out response snapshots plus task/worktree query rows.
// Boundary OUT: task execution and worktree materialization, owned by the daemon.
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

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
      status: "failed",
      error: "Worktree setup failed.",
      worktree_id: "wt_checkout",
    } as TaskRun;

    render(
      <TaskFanOutRunResults
        liveRuns={[live]}
        runs={accepted.runs}
        worktrees={[buildWorktreeFixture({ id: "wt_checkout", name: "checkout-fix" })]}
      />
    );

    const row = screen
      .getByText("Investigate checkout")
      .closest('[data-slot="task-fan-out-run-result"]');
    expect(row).toHaveAttribute("data-status", "failed");
    expect(row).not.toHaveAttribute("data-unattributed");
    expect(screen.getByText("checkout-fix")).toBeInTheDocument();
    expect(screen.getByText("Worktree setup failed.")).toBeInTheDocument();
  });
});
