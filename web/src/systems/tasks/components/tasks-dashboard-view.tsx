import { AlertCircle } from "lucide-react";

import { Empty, Eyebrow } from "@agh/ui";
import type { SchedulerBacklog, SchedulerStatus } from "@/systems/scheduler";
import { SchedulerControlsPanel } from "@/systems/scheduler";

import type { TaskDashboardView } from "../types";
import { TasksDashboardActiveRuns } from "./tasks-dashboard-active-runs";
import { TasksDashboardCards } from "./tasks-dashboard-cards";
import { TasksDashboardQueueHealth } from "./tasks-dashboard-queue-health";
import { TasksDashboardStatusBreakdown } from "./tasks-dashboard-status-breakdown";
import { TasksDashboardLoadingSkeleton } from "./task-loading-skeletons";

export interface TasksDashboardViewProps {
  dashboard: TaskDashboardView | null;
  dashboardStatus?: "loading" | "ready";
  errorMessage?: string | null;
  scheduler?: SchedulerStatus | null;
  schedulerBacklog?: SchedulerBacklog | null;
  schedulerStatus?: "loading" | "ready";
  schedulerBacklogStatus?: "loading" | "ready";
  schedulerErrorMessage?: string | null;
  schedulerBacklogErrorMessage?: string | null;
  schedulerPendingActions?: ReadonlySet<"pause" | "resume" | "drain">;
  onPauseScheduler?: (reason: string) => void | Promise<void>;
  onResumeScheduler?: () => void | Promise<void>;
  onDrainScheduler?: (input: { reason?: string; timeoutSeconds?: number }) => void | Promise<void>;
}

/**
 * Tasks dashboard composition: KPI strip → queue health + status breakdown
 * → active runs → trailing totals eyebrow. Section gap is 16 px to match the
 * runtime section rhythm; the live/stale freshness pill lives in the window topbar,
 * never inside this view.
 */
export function TasksDashboardView({
  dashboard,
  dashboardStatus = "ready",
  errorMessage = null,
  scheduler = null,
  schedulerBacklog = null,
  schedulerStatus = "ready",
  schedulerBacklogStatus = "ready",
  schedulerErrorMessage = null,
  schedulerBacklogErrorMessage = null,
  schedulerPendingActions,
  onPauseScheduler,
  onResumeScheduler,
  onDrainScheduler,
}: TasksDashboardViewProps) {
  if (dashboardStatus === "loading" && !dashboard) {
    return <TasksDashboardLoadingSkeleton />;
  }

  if (errorMessage && !dashboard) {
    return (
      <Empty
        icon={AlertCircle}
        title="Unable to load dashboard"
        description={errorMessage}
        data-testid="tasks-dashboard-error"
      />
    );
  }

  if (!dashboard) {
    return (
      <Empty
        description="Create or run tasks to see queue depth, health, freshness, and live work in one place."
        icon={AlertCircle}
        title="No dashboard data yet"
        data-testid="tasks-dashboard-empty"
      />
    );
  }

  return (
    <div
      className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto p-5"
      data-testid="tasks-dashboard-view"
    >
      <SchedulerControlsPanel
        backlog={schedulerBacklog}
        backlogErrorMessage={schedulerBacklogErrorMessage}
        errorMessage={schedulerErrorMessage}
        isBacklogLoading={schedulerBacklogStatus === "loading"}
        isLoading={schedulerStatus === "loading"}
        onDrain={onDrainScheduler ? () => onDrainScheduler({ timeoutSeconds: 60 }) : undefined}
        onPause={onPauseScheduler}
        onResume={onResumeScheduler}
        pending={{
          drain: schedulerPendingActions?.has("drain") ?? false,
          pause: schedulerPendingActions?.has("pause") ?? false,
          resume: schedulerPendingActions?.has("resume") ?? false,
        }}
        status={scheduler}
      />

      <TasksDashboardCards dashboard={dashboard} />

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[2fr_1fr]">
        <TasksDashboardQueueHealth dashboard={dashboard} />
        <TasksDashboardStatusBreakdown dashboard={dashboard} />
      </div>

      <TasksDashboardActiveRuns dashboard={dashboard} />

      <div
        className="flex items-center justify-end gap-2 border-t border-line-soft pt-3"
        data-testid="tasks-dashboard-totals"
      >
        <Eyebrow className="text-muted">
          {dashboard.totals.tasks_total} tasks · {dashboard.totals.runs_total} runs
        </Eyebrow>
      </div>
    </div>
  );
}
