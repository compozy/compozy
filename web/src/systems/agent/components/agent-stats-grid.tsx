import { Metric, MetricGrid, Time } from "@agh/ui";

export type AgentStatsGridVariant = "overview" | "sessions";

export interface AgentStatsGridProps {
  active: number;
  /** Formatted runtime duration, or null when there is no elapsed runtime yet. */
  runtimeLabel: string | null;
  failed: number | null;
  lastActivityAt?: string | null;
  sessionsTotal?: number;
  variant?: AgentStatsGridVariant;
  className?: string;
  /**
   * True when catalog returned the exact workspace-scoped session aggregate.
   * When false, Active / Runtime / Failed / Last activity (or Total) all render as dash —
   * never inferred zeros or "Never".
   */
  metricsAvailable?: boolean;
  /** True when the session/metrics query failed entirely. */
  unavailable?: boolean;
}

export function AgentStatsGrid({
  active,
  runtimeLabel,
  failed,
  lastActivityAt = null,
  sessionsTotal,
  variant = "overview",
  className,
  metricsAvailable = true,
  unavailable = false,
}: AgentStatsGridProps) {
  const dash = "—";
  const showMetrics = metricsAvailable && !unavailable;

  return (
    <MetricGrid data-testid="agent-stats-grid" className={className}>
      <Metric
        label="Active"
        value={showMetrics ? active : dash}
        tone={showMetrics && active > 0 ? "success" : "default"}
        subtext={
          variant === "overview" && showMetrics && typeof sessionsTotal === "number"
            ? `of ${sessionsTotal} sessions`
            : !showMetrics
              ? "session metrics unavailable"
              : undefined
        }
        aria-label={
          !showMetrics
            ? "Active is unavailable because the catalog did not return session aggregates for this agent."
            : undefined
        }
        data-testid="agent-stat-active"
      />
      <Metric
        label="Runtime"
        value={showMetrics && runtimeLabel !== null ? runtimeLabel : dash}
        subtext={
          showMetrics && runtimeLabel !== null
            ? "elapsed across sessions"
            : !showMetrics
              ? "session metrics unavailable"
              : undefined
        }
        aria-label={
          !showMetrics
            ? "Runtime is unavailable because the catalog did not return session aggregates for this agent."
            : undefined
        }
        data-testid="agent-stat-runtime"
      />
      <Metric
        label="Failed"
        value={showMetrics && failed !== null ? failed : dash}
        tone={showMetrics && failed !== null && failed > 0 ? "danger" : "default"}
        subtext={
          showMetrics && failed !== null
            ? "stopped with failure"
            : !showMetrics
              ? "session metrics unavailable"
              : undefined
        }
        aria-label={
          !showMetrics
            ? "Failed count is unavailable because the catalog did not return session aggregates for this agent."
            : undefined
        }
        data-testid="agent-stat-failed"
      />
      {variant === "sessions" ? (
        <Metric
          label="Total"
          value={showMetrics ? (sessionsTotal ?? 0) : dash}
          subtext={!showMetrics ? "session metrics unavailable" : undefined}
          aria-label={
            !showMetrics
              ? "Total is unavailable because the catalog did not return session aggregates for this agent."
              : undefined
          }
          data-testid="agent-stat-total"
        />
      ) : (
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
          subtext={showMetrics ? "from scoped sessions" : "session metrics unavailable"}
          aria-label={
            !showMetrics
              ? "Last activity is unavailable because the catalog did not return session aggregates for this agent."
              : undefined
          }
          data-testid="agent-stat-last-activity"
        />
      )}
    </MetricGrid>
  );
}
