import { Metric } from "@agh/ui";

import { formatDurationMs, formatPercent } from "../lib/task-formatters";
import type { TaskDashboardView } from "../types";

export interface TasksDashboardCardsProps {
  dashboard: TaskDashboardView;
}

/**
 * Dashboard KPI strip — four flat `<Metric>` neutrals with eyebrow labels. The
 * value stays `--fg-strong`, never tone recolored. The freshness pill lives in
 * the window topbar, not here.
 */
export function TasksDashboardCards({ dashboard }: TasksDashboardCardsProps) {
  const { active_runs, totals, cards, queue } = dashboard;

  const activeRuns = active_runs.running;
  const activeDetail =
    [
      active_runs.queued > 0 ? `${active_runs.queued} queued` : null,
      active_runs.claimed > 0 ? `${active_runs.claimed} claimed` : null,
    ]
      .filter(Boolean)
      .join(" · ") || "idle";

  const successRate = computeSuccessRate(totals);
  const avgDurationMs = cards.latency.claim_latency_ms.average_ms;
  const avgDurationSamples = cards.latency.claim_latency_ms.samples;
  const avgDurationDetail = avgDurationSamples > 0 ? `n=${avgDurationSamples}` : "no data";

  const queueDepth = queue.total;
  const queueDetail = queue.backlog_warning
    ? `oldest ${formatDurationMs(queue.oldest_queue_age_ms)}`
    : queueDepth > 0
      ? "queued"
      : "drained";

  return (
    <div
      className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4"
      data-testid="tasks-dashboard-cards"
    >
      <Metric
        labelCase="eyebrow"
        data-testid="tasks-dashboard-card-active-runs"
        subtext={activeDetail}
        label="Active runs"
        value={activeRuns}
      />
      <Metric
        labelCase="eyebrow"
        data-testid="tasks-dashboard-card-success-rate"
        subtext="last 24h"
        label="Success rate"
        value={successRate === null ? "--" : formatPercent(successRate)}
      />
      <Metric
        labelCase="eyebrow"
        data-testid="tasks-dashboard-card-average-duration"
        subtext={avgDurationDetail}
        label="Average duration"
        value={formatDurationMs(avgDurationMs)}
      />
      <Metric
        labelCase="eyebrow"
        data-testid="tasks-dashboard-card-queue-depth"
        subtext={queueDetail}
        label="Queue depth"
        value={queueDepth}
      />
    </div>
  );
}

function computeSuccessRate(totals: TaskDashboardView["totals"]): number | null {
  const completed = totals.completed_runs ?? 0;
  const failed = totals.failed_runs ?? 0;
  const canceled = totals.canceled_runs ?? 0;
  const observed = completed + failed + canceled;
  if (observed === 0) {
    return null;
  }
  return (completed / observed) * 100;
}
