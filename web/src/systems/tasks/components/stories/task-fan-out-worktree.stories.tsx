import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { fn, userEvent, within } from "storybook/test";

import { StorySurface } from "@/storybook/story-layout";
import {
  worktreeBehindFixture,
  worktreeMergedFixture,
  worktreeReadyDirtyRunningFixture,
} from "@/systems/workspace/mocks";

import { buildTaskRunRecordFixture } from "../../mocks";
import { TaskFanOutDialog } from "../task-fan-out-dialog";
import { TaskFanOutIsolationRow } from "../task-fan-out-isolation-row";
import { TaskFanOutRunResults } from "../task-fan-out-run-results";

const WORKTREES = [worktreeReadyDirtyRunningFixture, worktreeBehindFixture, worktreeMergedFixture];

const ACCEPTED_RUNS = [
  buildTaskRunRecordFixture({
    id: "run_fanout_1",
    status: "queued",
    designation: { index: 0, brief: "Investigate the failing checkout path" },
    designation_group_id: "desig_storybook",
    resolved_worktree_mode: "per_run",
    worktree_id: undefined,
  }),
  buildTaskRunRecordFixture({
    id: "run_fanout_2",
    status: "queued",
    designation: { index: 1, brief: "Validate the fix on staging" },
    designation_group_id: "desig_storybook",
    resolved_worktree_mode: "per_run",
    worktree_id: undefined,
  }),
  buildTaskRunRecordFixture({
    id: "run_fanout_3",
    status: "queued",
    designation: { index: 2, brief: "Backfill the reconciliation report" },
    designation_group_id: "desig_storybook",
    resolved_worktree_mode: "per_run",
    worktree_id: undefined,
  }),
];

const LIVE_RUNS = [
  buildTaskRunRecordFixture({
    id: "run_fanout_1",
    status: "completed",
    designation: { index: 0, brief: "Investigate the failing checkout path" },
    designation_group_id: "desig_storybook",
    resolved_worktree_mode: "per_run",
    worktree_id: worktreeMergedFixture.id,
  }),
  buildTaskRunRecordFixture({
    id: "run_fanout_2",
    status: "running",
    designation: { index: 1, brief: "Validate the fix on staging" },
    designation_group_id: "desig_storybook",
    resolved_worktree_mode: "per_run",
    worktree_id: undefined,
  }),
  buildTaskRunRecordFixture({
    id: "run_fanout_3",
    status: "failed",
    designation: { index: 2, brief: "Backfill the reconciliation report" },
    designation_group_id: "desig_storybook",
    resolved_worktree_mode: "per_run",
    error: "Worktree creation rolled back: branch run/backfill-report already exists.",
  }),
];

function IsolationHarness({ initial, count }: { initial: boolean; count: number }) {
  const [checked, setChecked] = useState(initial);
  return (
    <StorySurface className="w-[520px]">
      <TaskFanOutIsolationRow
        checked={checked}
        designationCount={count}
        onCheckedChange={setChecked}
      />
    </StorySurface>
  );
}

const meta: Meta<typeof TaskFanOutIsolationRow> = {
  title: "systems/tasks/components/TaskFanOutWorktree",
  component: TaskFanOutIsolationRow,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Per-run isolation is opt-in, and its consequence is counted from the same brief lines the request will carry — so the number on screen can never disagree with the number of worktrees created.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-25 — off by default: nothing is promised, so nothing is stated. */
export const IsolationOff: Story = {
  args: { checked: false, designationCount: 3, onCheckedChange: fn() },
  render: () => <IsolationHarness count={3} initial={false} />,
};

/** VC-26 — on: the count comes from the parsed briefs, and branches land in run/. */
export const IsolationOnWithCount: Story = {
  args: { checked: true, designationCount: 3, onCheckedChange: fn() },
  render: () => (
    <TaskFanOutDialog
      gitBacked
      onFanOut={fn()}
      onOpenChange={fn()}
      open
      taskId="task_001"
      worktrees={WORKTREES}
    />
  ),
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    const designations = await body.findByTestId("tasks-fan-out-designations");
    await userEvent.type(
      designations,
      "Investigate the failing checkout path{Enter}Validate the fix on staging{Enter}Backfill the reconciliation report"
    );
    await userEvent.click(await body.findByTestId("tasks-fan-out-worktree-per-run"));
    await body.findByText("Creates 3 worktrees, one per run.");
    await body.findByText("Branches land in the run/ namespace.");
  },
};

/** VC-27 — queued snapshots plus live truth: attributed, unattributed, failed+retry. */
export const RunResultsAttributedFailure: Story = {
  args: IsolationOff.args,
  render: () => (
    <StorySurface className="w-[560px]">
      <TaskFanOutRunResults
        liveRuns={LIVE_RUNS}
        onRetry={fn()}
        runs={ACCEPTED_RUNS}
        worktrees={WORKTREES}
      />
    </StorySurface>
  ),
};
