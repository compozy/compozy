import type { MetricTone } from "@compozy/ui";

export const AGENT_STAT_DASH = "—";

export interface AgentStatsInput {
  active: number;
  runtimeLabel: string | null;
  failed: number | null;
  sessionsTotal?: number;
  showMetrics: boolean;
}

export interface AgentStatsView {
  active: number | string;
  activeTone: MetricTone;
  activeSubtext: string | undefined;
  runtime: string;
  failed: number | string;
  failedTone: MetricTone;
}

const DASHED_STATS: AgentStatsView = {
  active: AGENT_STAT_DASH,
  activeTone: "default",
  activeSubtext: undefined,
  runtime: AGENT_STAT_DASH,
  failed: AGENT_STAT_DASH,
  failedTone: "default",
};

/** Maps agent session aggregates to metric values; unavailable metrics render as dashes. */
export function agentStatsView(input: AgentStatsInput): AgentStatsView {
  if (!input.showMetrics) return DASHED_STATS;
  const { active, failed, runtimeLabel, sessionsTotal } = input;
  return {
    active,
    activeTone: active > 0 ? "success" : "default",
    activeSubtext: typeof sessionsTotal === "number" ? `of ${sessionsTotal} sessions` : undefined,
    runtime: runtimeLabel ?? AGENT_STAT_DASH,
    failed: failed ?? AGENT_STAT_DASH,
    failedTone: failed !== null && failed > 0 ? "danger" : "default",
  };
}
