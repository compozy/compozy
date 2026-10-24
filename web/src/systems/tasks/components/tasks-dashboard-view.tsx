import { useState, type ReactNode } from "react";
import { AlertCircle } from "lucide-react";

import {
  cn,
  Disclosure,
  Empty,
  Eyebrow,
  Skeleton,
  SkeletonRegion,
  SkeletonText,
} from "@compozy/ui";

import type { TaskDashboardView } from "../types";
import {
  TasksDashboardActiveRuns,
  TasksDashboardActiveRunsSkeleton,
  type TasksDashboardActiveRunsProps,
} from "./tasks-dashboard-active-runs";
import { TasksDashboardCards, TasksDashboardCardsSkeleton } from "./tasks-dashboard-cards";
import {
  TasksDashboardQueueHealth,
  TasksDashboardQueueHealthSkeleton,
} from "./tasks-dashboard-queue-health";
import {
  TasksDashboardStatusBreakdown,
  TasksDashboardStatusBreakdownSkeleton,
} from "./tasks-dashboard-status-breakdown";
import {
  type SchedulerBacklog,
  SchedulerControlsPanel,
  type SchedulerStatus,
} from "@/systems/scheduler";

/** The dashboard's frame, shared by the live view and its skeleton so the sections stay put. */
const BODY_CLASS = "@container flex min-h-0 flex-1 flex-col gap-4 p-5";
const PANEL_PAIR_CLASS = "grid grid-cols-1 gap-4 @5xl:grid-cols-[2fr_1fr]";
const QUEUE_CONTROLS_CLASS = "border-t border-line-soft pt-1";
const TOTALS_CLASS = "flex items-center justify-end gap-2 border-t border-line-soft pt-3";

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
  /** Each active run's project under All projects; absent while one scope is shown. */
  projectBadgeOf?: TasksDashboardActiveRunsProps["projectBadgeOf"];
}

/** The queue needs a person when it is paused or holding stuck/attention work. */
function schedulerNeedsYou(status: SchedulerStatus | null): boolean {
  if (!status) return false;
  return (
    status.paused ||
    (status.starved_run_count ?? 0) > 0 ||
    (status.needs_attention_run_count ?? 0) > 0
  );
}

/**
 * Operator dispatch controls, folded closed by default and opened on their own
 * when the queue needs a person, so the dashboard leads with the numbers.
 */
function DashboardQueueControls({ forceOpen, panel }: { forceOpen: boolean; panel: ReactNode }) {
  const [open, setOpen] = useState(false);
  const isOpen = open || forceOpen;
  return (
    <Disclosure
      className={QUEUE_CONTROLS_CLASS}
      data-testid="tasks-dashboard-queue-controls"
      label="Queue controls"
      size="md"
      onOpenChange={setOpen}
      open={isOpen}
      triggerProps={{
        "data-testid": "tasks-dashboard-queue-controls-toggle",
        className: "w-full py-2.5 text-fg",
      }}
      contentProps={{ className: "pt-0" }}
    >
      {panel}
    </Disclosure>
  );
}

/**
 * Tasks dashboard composition: KPI strip → active runs → queue health + status
 * breakdown → folded queue controls → trailing totals eyebrow. Section gap is 16 px to match the
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
  projectBadgeOf,
}: TasksDashboardViewProps) {
  if (dashboardStatus === "loading" && !dashboard) {
    return <TasksDashboardLoading />;
  }

  if (errorMessage && !dashboard) {
    return (
      <Empty
        icon={AlertCircle}
        title="Couldn't load the dashboard"
        description={errorMessage}
        data-testid="tasks-dashboard-error"
      />
    );
  }

  if (!dashboard) {
    return (
      <Empty
        description="Create or run tasks to see what's running and what's waiting."
        icon={AlertCircle}
        title="No dashboard data yet"
        data-testid="tasks-dashboard-empty"
      />
    );
  }

  return (
    <div className={cn(BODY_CLASS, "overflow-y-auto")} data-testid="tasks-dashboard-view">
      <TasksDashboardCards dashboard={dashboard} />

      <TasksDashboardActiveRuns dashboard={dashboard} projectBadgeOf={projectBadgeOf} />

      <div className={PANEL_PAIR_CLASS}>
        <TasksDashboardQueueHealth dashboard={dashboard} />
        <TasksDashboardStatusBreakdown dashboard={dashboard} />
      </div>

      <DashboardQueueControls
        forceOpen={schedulerNeedsYou(scheduler)}
        panel={
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
        }
      />

      <div className={TOTALS_CLASS} data-testid="tasks-dashboard-totals">
        <Eyebrow className="text-muted">
          {dashboard.totals.tasks_total} tasks · {dashboard.totals.runs_total} runs
        </Eyebrow>
      </div>
    </div>
  );
}

/**
 * The dashboard while its read is out: every section in its place with its real heading, and
 * the figures left as blocks. The scheduler fold and the totals line are placeholders too, so
 * nothing interactive sits inside the loading region.
 */
function TasksDashboardLoading() {
  return (
    <SkeletonRegion
      className={cn(BODY_CLASS, "overflow-hidden")}
      data-testid="tasks-dashboard-loading"
      label="Loading tasks dashboard"
    >
      <TasksDashboardCardsSkeleton />
      <TasksDashboardActiveRunsSkeleton />
      <div className={PANEL_PAIR_CLASS}>
        <TasksDashboardQueueHealthSkeleton />
        <TasksDashboardStatusBreakdownSkeleton />
      </div>
      <div aria-hidden="true" className={QUEUE_CONTROLS_CLASS}>
        <div className="flex items-center gap-2 py-2.5 text-small-body">
          <Skeleton className="size-3.5 shrink-0 rounded-full" />
          <SkeletonText className="w-24" />
        </div>
      </div>
      <div aria-hidden="true" className={TOTALS_CLASS}>
        <SkeletonText className="w-32 text-eyebrow" />
      </div>
    </SkeletonRegion>
  );
}
