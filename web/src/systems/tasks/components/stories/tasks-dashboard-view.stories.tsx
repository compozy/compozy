import type { Meta, StoryObj } from "@storybook/react-vite";

import { PanelSurface } from "@/storybook/story-layout";
import { schedulerBacklogFixture, schedulerStatusFixture } from "@/systems/scheduler/mocks";
import type { TaskDashboardView } from "../../types";
import { TasksDashboardView } from "../tasks-dashboard-view";
import { buildDashboardFixture } from "../test-fixtures";

const meta: Meta<typeof TasksDashboardView> = {
  title: "systems/tasks/components/TasksDashboardView",
  component: TasksDashboardView,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function Frame({ children }: { children: React.ReactNode }) {
  return (
    <PanelSurface className="min-h-[720px] p-0">
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">{children}</div>
    </PanelSurface>
  );
}

const POPULATED: TaskDashboardView = buildDashboardFixture({
  active_runs: {
    claimed: 1,
    queued: 2,
    running: 4,
    starting: 0,
    total: 7,
    items: [
      {
        age_ms: 45_000,
        attempt: 2,
        health_status: "ok",
        last_activity_at: "2026-04-17T10:00:00Z",
        max_attempts: 3,
        run_id: "run_a1",
        run_status: "running",
        scope: "workspace",
        stuck: false,
        task_id: "task_a1",
        task_identifier: "TASK-42",
        task_status: "in_progress",
        task_title: "Summarize review feedback",
      },
      {
        age_ms: 420_000,
        attempt: 1,
        error: "rate limited",
        health_status: "warning",
        last_activity_at: "2026-04-17T09:53:00Z",
        max_attempts: 5,
        run_id: "run_a2",
        run_status: "running",
        scope: "workspace",
        stuck: true,
        task_id: "task_a2",
        task_identifier: "TASK-43",
        task_status: "in_progress",
        task_title: "Bridge health telemetry",
      },
    ],
  },
  queue: {
    backlog_status: "ok",
    backlog_threshold_ms: 60_000,
    backlog_warning: false,
    oldest_queue_age_ms: 12_000,
    oldest_queued_at: "2026-04-17T09:59:00Z",
    total: 3,
  },
  status_breakdown: [
    { count: 14, share_percent: 42, status: "completed" },
    { count: 6, share_percent: 18, status: "in_progress" },
    { count: 8, share_percent: 24, status: "pending" },
    { count: 2, share_percent: 6, status: "blocked" },
    { count: 3, share_percent: 10, status: "failed" },
  ],
});

POPULATED.totals = {
  ...POPULATED.totals,
  completed_runs: 48,
  failed_runs: 3,
  canceled_runs: 1,
  runs_total: 92,
  tasks_total: 33,
  active_runs: 7,
};

export const Populated: Story = {
  render: () => (
    <Frame>
      <TasksDashboardView
        dashboard={POPULATED}
        scheduler={schedulerStatusFixture}
        schedulerBacklog={schedulerBacklogFixture}
      />
    </Frame>
  ),
};

export const EmptyDashboard: Story = {
  name: "Empty",
  render: () => (
    <Frame>
      <TasksDashboardView dashboard={null} />
    </Frame>
  ),
};

export const Loading: Story = {
  render: () => (
    <Frame>
      <TasksDashboardView dashboard={null} dashboardStatus="loading" />
    </Frame>
  ),
};

export const ErrorState: Story = {
  name: "Error",
  render: () => (
    <Frame>
      <TasksDashboardView dashboard={null} errorMessage="Dashboard unavailable" />
    </Frame>
  ),
};

export const BacklogWarning: Story = {
  name: "Backlog warning",
  render: () => {
    const dashboard = buildDashboardFixture({
      queue: {
        backlog_status: "warning",
        backlog_threshold_ms: 60_000,
        backlog_warning: true,
        oldest_queue_age_ms: 320_000,
        oldest_queued_at: "2026-04-17T09:54:40Z",
        total: 12,
      },
      health: {
        active_orphan_runs: 0,
        queue_backlog: true,
        status: "warning",
        stuck_runs: 1,
      },
    });
    return (
      <Frame>
        <TasksDashboardView
          dashboard={dashboard}
          scheduler={schedulerStatusFixture}
          schedulerBacklog={schedulerBacklogFixture}
        />
      </Frame>
    );
  },
};
