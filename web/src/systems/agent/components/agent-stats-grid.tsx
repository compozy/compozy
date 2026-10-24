import { Metric, MetricGrid, SkeletonText, Time } from "@compozy/ui";

import { AGENT_STAT_DASH, agentStatsView } from "../lib/agent-stats-view";

export interface AgentStatsGridProps {
  active: number;
  /** Formatted runtime duration, or null when there is no elapsed runtime yet. */
  runtimeLabel: string | null;
  failed: number | null;
  lastActivityAt?: string | null;
  sessionsTotal?: number;
  className?: string;
  /**
   * True when catalog returned the exact workspace-scoped session aggregate.
   * When false, every metric renders as a dash — never inferred zeros or "Never".
   */
  metricsAvailable?: boolean;
  /** True when the session/metrics query failed entirely. */
  unavailable?: boolean;
}

const UNAVAILABLE_LABEL = "Not available right now.";

const STAT_LABEL = {
  active: "Active",
  runtime: "Time working",
  failed: "Failed",
  lastActivity: "Last activity",
} as const;

function LastActivityValue({
  showMetrics,
  lastActivityAt,
}: {
  showMetrics: boolean;
  lastActivityAt: string | null;
}) {
  if (!showMetrics) return AGENT_STAT_DASH;
  if (!lastActivityAt) return "Never";
  return <Time iso={lastActivityAt} mode="relative" />;
}

export function AgentStatsGrid({
  active,
  runtimeLabel,
  failed,
  lastActivityAt = null,
  sessionsTotal,
  className,
  metricsAvailable = true,
  unavailable = false,
}: AgentStatsGridProps) {
  const showMetrics = metricsAvailable && !unavailable;
  const unavailableLabel = showMetrics ? undefined : UNAVAILABLE_LABEL;
  const stats = agentStatsView({ active, runtimeLabel, failed, sessionsTotal, showMetrics });

  return (
    <MetricGrid data-testid="agent-stats-grid" className={className}>
      <Metric
        label={STAT_LABEL.active}
        value={stats.active}
        tone={stats.activeTone}
        subtext={stats.activeSubtext}
        aria-label={unavailableLabel}
        data-testid="agent-stat-active"
      />
      <Metric
        label={STAT_LABEL.runtime}
        value={stats.runtime}
        aria-label={unavailableLabel}
        data-testid="agent-stat-runtime"
      />
      <Metric
        label={STAT_LABEL.failed}
        value={stats.failed}
        tone={stats.failedTone}
        aria-label={unavailableLabel}
        data-testid="agent-stat-failed"
      />
      <Metric
        label={STAT_LABEL.lastActivity}
        value={<LastActivityValue showMetrics={showMetrics} lastActivityAt={lastActivityAt} />}
        aria-label={unavailableLabel}
        data-testid="agent-stat-last-activity"
      />
    </MetricGrid>
  );
}

/**
 * The tiles while the session aggregates load: the real labels, grid and tile padding, with a
 * placeholder where each value goes. The Active tile carries its "of N sessions" line, as it does
 * once the aggregate is exact, so the row keeps its height. Wrap in a `SkeletonRegion`.
 */
export function AgentStatsGridSkeleton({ className }: Pick<AgentStatsGridProps, "className">) {
  return (
    <MetricGrid data-testid="agent-stats-grid-skeleton" className={className}>
      <Metric
        label={STAT_LABEL.active}
        value={<SkeletonText className="w-8" />}
        subtext={<SkeletonText className="w-24" />}
      />
      <Metric label={STAT_LABEL.runtime} value={<SkeletonText className="w-16" />} />
      <Metric label={STAT_LABEL.failed} value={<SkeletonText className="w-8" />} />
      <Metric label={STAT_LABEL.lastActivity} value={<SkeletonText className="w-20" />} />
    </MetricGrid>
  );
}
