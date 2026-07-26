import type { Meta, StoryObj } from "@storybook/react-vite";

import { PanelSurface } from "@/storybook/story-layout";

import {
  buildDetailFixture,
  buildTaskRunRecordFixture,
  taskTimelineFixture,
} from "../../mocks/fixtures";
import type { TaskNowStripHandlers } from "../task-now-strip";
import { TaskActivityPanel } from "../task-activity-panel";
import { TaskOverviewPanel } from "../task-overview-panel";
import { TaskPropertiesRail } from "../task-properties-rail";
import { TaskRunsPanel } from "../task-runs-panel";
import { TasksDetailSubhead } from "../tasks-detail-subhead";

const meta: Meta<typeof TaskOverviewPanel> = {
  title: "systems/tasks/components/TaskDetailRedesign",
  component: TaskOverviewPanel,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Task-detail redesign surfaces: subhead, Now strip, overview sections, runs table, activity feed, and the 320px properties rail.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const noopHandlers: TaskNowStripHandlers = {
  onOpenRun: () => undefined,
  onOpenTask: () => undefined,
  onApprove: () => undefined,
  onReject: () => undefined,
  onResume: () => undefined,
  onRecover: () => undefined,
  onClearBlock: () => undefined,
  onViewResult: () => undefined,
};

const runningDetail = buildDetailFixture();
const runningRuns = [
  buildTaskRunRecordFixture({
    id: "run_002",
    attempt: 2,
    status: "running",
    previous_run_id: "run_001",
    claimed_by: { kind: "agent_session", ref: "product-manager-agent" },
    started_at: "2026-04-17T10:05:00Z",
  }),
  buildTaskRunRecordFixture({
    id: "run_001",
    attempt: 1,
    status: "failed",
    error: "Partner-bank replay data went stale before the checkpoint",
    started_at: "2026-04-17T09:01:00Z",
    ended_at: "2026-04-17T09:09:12Z",
  }),
];

/**
 * Overview tab while attempt 2 runs: active-run card, description, subtasks
 * with progress, collapsed resolved dependencies, recent activity.
 */
export const OverviewRunning: Story = {
  args: {},
  render: () => (
    <PanelSurface className="max-w-[860px] p-6">
      <TasksDetailSubhead detail={runningDetail} />
      <TaskOverviewPanel
        detail={runningDetail}
        isLive
        nowHandlers={noopHandlers}
        onViewAllActivity={() => undefined}
        runs={runningRuns}
        timeline={taskTimelineFixture}
      />
    </PanelSurface>
  ),
};

/**
 * Overview in the blocked + approval-pending state: resolving cards replace
 * banner prose; each block carries its own action.
 */
export const OverviewBlocked: Story = {
  args: {},
  render: () => {
    const detail = buildDetailFixture({
      task: {
        status: "blocked",
        approval_state: "pending",
        priority: "high",
        blocked_reasons: [
          { source: "approval" },
          { source: "dependency", depends_on_task_ids: ["task_dep_001"] },
        ],
      } as never,
      summary: { active_run: null } as never,
    });
    return (
      <PanelSurface className="max-w-[860px] p-6">
        <TasksDetailSubhead detail={detail} />
        <TaskOverviewPanel
          detail={detail}
          isLive={false}
          nowHandlers={noopHandlers}
          onViewAllActivity={() => undefined}
          runs={[]}
          timeline={taskTimelineFixture.slice(0, 3)}
        />
      </PanelSurface>
    );
  },
};

/**
 * Runs tab: attempts newest-first, lineage caret on retries, whole row links.
 */
export const RunsTable: Story = {
  args: {},
  render: () => (
    <PanelSurface className="max-w-[860px] p-6">
      <TaskRunsPanel runs={runningRuns} taskId="task_001" />
    </PanelSurface>
  ),
};

/**
 * Activity tab: humanized titles with the raw event type kept as microtext.
 */
export const ActivityFeed: Story = {
  args: {},
  render: () => (
    <PanelSurface className="max-w-[860px] p-6">
      <TaskActivityPanel isLive items={taskTimelineFixture} />
    </PanelSurface>
  ),
};

/**
 * Properties rail: tier a/b fields with inline priority + auto-enqueue
 * editors and the operator Inspect entry in the footer.
 */
export const PropertiesRail: Story = {
  args: {},
  render: () => (
    <PanelSurface className="max-w-[360px] p-5">
      <TaskPropertiesRail
        detail={runningDetail}
        onApprove={() => undefined}
        onAutoEnqueueChange={() => undefined}
        onEditSetup={() => undefined}
        onInspect={() => undefined}
        onPriorityChange={() => undefined}
        onReject={() => undefined}
        runs={runningRuns}
      />
    </PanelSurface>
  ),
};
