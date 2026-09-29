import { Metric } from "@compozy/ui";

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
      active_runs.queued > 0 ? `${active_runs.queued} waiting` : null,
      active_runs.claimed > 0 ? `${active_runs.claimed} starting` : null,
    ]
      .filter(Boolean)
      .join(" · ") || "nothing else waiting";

  const successRate = computeSuccessRate(totals);
  // Claim latency: how long queued work waits before an agent picks it up. The
  // dashboard read model carries no run-duration average, so the card names
  // what it measures instead of calling it a duration.
  const pickupMs = cards.latency.claim_latency_ms.average_ms;
  const pickupSamples = cards.latency.claim_latency_ms.samples;
  const pickupDetail =
    pickupSamples > 0
      ? `avg over ${pickupSamples} ${pickupSamples === 1 ? "run" : "runs"}`
      : "no runs yet";

  const queueDepth = queue.total;
  const queueDetail = queue.backlog_warning
    ? `oldest waiting ${formatDurationMs(queue.oldest_queue_age_ms)}`
    : queueDepth > 0
      ? "waiting to start"
      : "nothing waiting";

  return (
    <div
      className="grid grid-cols-1 gap-4 @xl:grid-cols-2 @4xl:grid-cols-4"
      data-testid="tasks-dashboard-cards"
    >
      <Metric
        labelCase="eyebrow"
        data-testid="tasks-dashboard-card-active-runs"
        subtext={activeDetail}
        label="Running now"
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
        subtext={pickupDetail}
        label="Time to pick up"
        value={pickupSamples > 0 ? formatDurationMs(pickupMs) : "--"}
      />
      <Metric
        labelCase="eyebrow"
        data-testid="tasks-dashboard-card-queue-depth"
        subtext={queueDetail}
        label="Waiting"
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
