import { useState, type ReactNode } from "react";
import { AlertCircle, ChevronRight } from "lucide-react";

import {
  cn,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Empty,
  Eyebrow,
} from "@compozy/ui";

import type { TaskDashboardView } from "../types";
import { TasksDashboardActiveRuns } from "./tasks-dashboard-active-runs";
import { TasksDashboardCards } from "./tasks-dashboard-cards";
import { TasksDashboardQueueHealth } from "./tasks-dashboard-queue-health";
import { TasksDashboardStatusBreakdown } from "./tasks-dashboard-status-breakdown";
import { TasksDashboardLoadingSkeleton } from "./task-loading-skeletons";
import {
  type SchedulerBacklog,
  SchedulerControlsPanel,
  type SchedulerStatus,
} from "@/systems/scheduler";

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
    <Collapsible
      className="border-t border-line-soft pt-1"
      data-testid="tasks-dashboard-queue-controls"
      onOpenChange={setOpen}
      open={isOpen}
    >
      <CollapsibleTrigger
        className="flex w-full items-center gap-2 rounded-sm py-2.5 text-left outline-none focus-visible:shadow-focus-ring"
        data-testid="tasks-dashboard-queue-controls-toggle"
        type="button"
      >
        <ChevronRight
          aria-hidden="true"
          className={cn(
            "size-4 text-muted transition-transform duration-base ease-out",
            isOpen && "rotate-90"
          )}
        />
        <span className="text-small-body font-medium text-fg">Queue controls</span>
      </CollapsibleTrigger>
      <CollapsibleContent>{panel}</CollapsibleContent>
    </Collapsible>
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
}: TasksDashboardViewProps) {
  if (dashboardStatus === "loading" && !dashboard) {
    return <TasksDashboardLoadingSkeleton />;
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
    <div
      className="@container flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto p-5"
      data-testid="tasks-dashboard-view"
    >
      <TasksDashboardCards dashboard={dashboard} />

      <TasksDashboardActiveRuns dashboard={dashboard} />

      <div className="grid grid-cols-1 gap-4 @5xl:grid-cols-[2fr_1fr]">
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
