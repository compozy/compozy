import type { Meta, StoryObj } from "@storybook/react-vite";

import { PanelSurface } from "@/storybook/story-layout";

import { buildTaskRunDetailFixture, buildTaskRunRecordFixture } from "../../mocks/fixtures";
import { TaskRunOutcome } from "../task-run-outcome";
import { TaskRunRail } from "../task-run-rail";
import { TaskRunSubhead } from "../task-run-subhead";

const meta: Meta<typeof TaskRunRail> = {
  title: "systems/tasks/components/TaskRunRedesign",
  component: TaskRunRail,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Run-detail redesign surfaces: demoted subhead, terminal outcome band, and the session/timing/lineage rail.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

type CostSummary = NonNullable<ReturnType<typeof buildTaskRunDetailFixture>["summary"]>;

function runWithCost(cost: Partial<CostSummary>) {
  const base = buildTaskRunDetailFixture();
  return buildTaskRunDetailFixture({ summary: { ...base.summary, ...cost } });
}

const failedRun = buildTaskRunDetailFixture({
  run: buildTaskRunRecordFixture({
    id: "run_001",
    attempt: 1,
    status: "failed",
    error: "Partner-bank replay data went stale before the checkpoint.",
    claimed_by: { kind: "agent_session", ref: "product-manager-agent" },
    started_at: "2026-04-17T09:01:00Z",
    ended_at: "2026-04-17T09:09:12Z",
  }),
});

const nextAttempt = buildTaskRunRecordFixture({
  id: "run_002",
  attempt: 2,
  status: "running",
  previous_run_id: "run_001",
});

/**
 * Failed attempt: subhead meta, danger outcome band with the queued follow-up
 * attempt, and the rail with best-effort metrics.
 */
export const FailedAttempt: Story = {
  args: {},
  render: () => (
    <PanelSurface className="max-w-[900px] p-6">
      <TaskRunSubhead run={failedRun} />
      <div className="grid items-start gap-8 lg:grid-cols-[minmax(0,1fr)_320px]">
        <TaskRunOutcome nextAttempt={nextAttempt} run={failedRun} />
        <TaskRunRail
          onInspect={() => undefined}
          run={failedRun}
          taskId="task_001"
          taskRuns={[failedRun.run, nextAttempt]}
        />
      </div>
    </PanelSurface>
  ),
};

/** A projected catalog charge remains visibly distinct from measured spend. */
export const EstimatedCost: Story = {
  args: {},
  render: () => (
    <PanelSurface className="max-w-[420px] p-6">
      <TaskRunRail
        onInspect={() => undefined}
        run={runWithCost({
          cost_status: "estimated",
          cost_source: "catalog_config",
          total_cost: 0.42,
          cost_currency: "USD",
        })}
        taskId="task_001"
        taskRuns={[]}
      />
    </PanelSurface>
  ),
};

/** Subscription-included usage carries no invented monetary amount. */
export const IncludedCost: Story = {
  args: {},
  render: () => (
    <PanelSurface className="max-w-[420px] p-6">
      <TaskRunRail
        onInspect={() => undefined}
        run={runWithCost({ cost_status: "included", cost_source: "none", total_cost: null })}
        taskId="task_001"
        taskRuns={[]}
      />
    </PanelSurface>
  ),
};

/** Unknown usage is explicit instead of masquerading as zero spend. */
export const UnknownCost: Story = {
  args: {},
  render: () => (
    <PanelSurface className="max-w-[420px] p-6">
      <TaskRunRail
        onInspect={() => undefined}
        run={runWithCost({ cost_status: "unknown", total_cost: null })}
        taskId="task_001"
        taskRuns={[]}
      />
    </PanelSurface>
  ),
};
