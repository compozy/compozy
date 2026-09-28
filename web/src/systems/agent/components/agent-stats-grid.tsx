import { Metric, MetricGrid, Time } from "@compozy/ui";

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
  const dash = "—";
  const showMetrics = metricsAvailable && !unavailable;
  const unavailableLabel = showMetrics ? undefined : UNAVAILABLE_LABEL;

  return (
    <MetricGrid data-testid="agent-stats-grid" className={className}>
      <Metric
        label="Active"
        value={showMetrics ? active : dash}
        tone={showMetrics && active > 0 ? "success" : "default"}
        subtext={
          showMetrics && typeof sessionsTotal === "number"
            ? `of ${sessionsTotal} sessions`
            : undefined
        }
        aria-label={unavailableLabel}
        data-testid="agent-stat-active"
      />
      <Metric
        label="Time working"
        value={showMetrics && runtimeLabel !== null ? runtimeLabel : dash}
        aria-label={unavailableLabel}
        data-testid="agent-stat-runtime"
      />
      <Metric
        label="Failed"
        value={showMetrics && failed !== null ? failed : dash}
        tone={showMetrics && failed !== null && failed > 0 ? "danger" : "default"}
        aria-label={unavailableLabel}
        data-testid="agent-stat-failed"
      />
      <Metric
        label="Last activity"
        value={
          !showMetrics ? (
            dash
          ) : lastActivityAt ? (
            <Time iso={lastActivityAt} mode="relative" />
          ) : (
            "Never"
          )
        }
        aria-label={unavailableLabel}
        data-testid="agent-stat-last-activity"
      />
    </MetricGrid>
  );
}
