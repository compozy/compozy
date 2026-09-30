import { AlertTriangle, Check } from "lucide-react";

import { MetadataList, MetadataListRow, Panel, Pill, type PillTone } from "@compozy/ui";

import { formatDurationMs } from "../lib/task-formatters";
import type { TaskDashboardView } from "../types";

export interface TasksDashboardQueueHealthProps {
  dashboard: TaskDashboardView;
}

/**
 * Queue health from the dashboard read model's current snapshot.
 *
 * The read model carries no queue-depth history, so the panel reports what it
 * does know — depth now, the oldest wait, the backlog threshold — instead of
 * drawing a 24h chart it would have to invent.
 */
export function TasksDashboardQueueHealth({ dashboard }: TasksDashboardQueueHealthProps) {
  const { queue, health, totals } = dashboard;
  const stuckRuns = health.stuck_runs;
  const orphanRuns = health.active_orphan_runs;
  const healthTone: PillTone =
    health.status === "ok" ? "success" : health.status === "warning" ? "warning" : "danger";
  const warningMessage = queue.backlog_warning
    ? `Queue older than ${formatDurationMs(queue.backlog_threshold_ms)}; oldest ${formatDurationMs(queue.oldest_queue_age_ms)}`
    : stuckRuns > 0
      ? `${stuckRuns} stuck runs detected. Investigate claimed or starting work.`
      : `${orphanRuns} active orphan runs detected.`;
  const hasWarning = queue.backlog_warning || stuckRuns > 0 || orphanRuns > 0;

  return (
    <Panel
      data-testid="tasks-dashboard-queue-health"
      right={
        <Pill data-testid="tasks-dashboard-health-status" form="plain" tone={healthTone}>
          <Pill.Dot />
          {health.status}
        </Pill>
      }
      title="Queue health"
    >
      <p className="text-form-label text-muted">
        {totals.runs_total} runs tracked · {totals.completed_runs} completed
      </p>

      <MetadataList className="mt-4" data-testid="tasks-dashboard-queue-snapshot">
        <MetadataListRow label="Waiting now">
          <span className="text-fg tabular-nums" data-testid="tasks-dashboard-queue-depth">
            {queue.total}
          </span>
        </MetadataListRow>
        <MetadataListRow label="Oldest wait">
          <span className="text-fg tabular-nums" data-testid="tasks-dashboard-queue-oldest-wait">
            {queue.total > 0 ? formatDurationMs(queue.oldest_queue_age_ms) : "—"}
          </span>
        </MetadataListRow>
        <MetadataListRow label="Alert after">
          <span className="tabular-nums">{formatDurationMs(queue.backlog_threshold_ms)}</span>
        </MetadataListRow>
      </MetadataList>

      {hasWarning ? (
        <div
          className="mt-4 flex items-start gap-2 rounded-lg bg-warning-tint px-3 py-2 text-form-label text-fg"
          data-testid="tasks-dashboard-warning"
        >
          <AlertTriangle aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-warning" />
          <span className="min-w-0">{warningMessage}</span>
        </div>
      ) : (
        <div
          className="mt-4 flex items-center gap-2 text-form-label text-success"
          data-testid="tasks-dashboard-ok"
        >
          <Check aria-hidden="true" className="size-3.5 shrink-0" />
          <span>Queue is healthy.</span>
        </div>
      )}
    </Panel>
  );
}
