import { Metric, MetricGrid, Time } from "@compozy/ui";

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
        label="Active"
        value={stats.active}
        tone={stats.activeTone}
        subtext={stats.activeSubtext}
        aria-label={unavailableLabel}
        data-testid="agent-stat-active"
      />
      <Metric
        label="Time working"
        value={stats.runtime}
        aria-label={unavailableLabel}
        data-testid="agent-stat-runtime"
      />
      <Metric
        label="Failed"
        value={stats.failed}
        tone={stats.failedTone}
        aria-label={unavailableLabel}
        data-testid="agent-stat-failed"
      />
      <Metric
        label="Last activity"
        value={<LastActivityValue showMetrics={showMetrics} lastActivityAt={lastActivityAt} />}
        aria-label={unavailableLabel}
        data-testid="agent-stat-last-activity"
      />
    </MetricGrid>
  );
}
