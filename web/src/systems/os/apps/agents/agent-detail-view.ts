import type { LaneTabsItem } from "@compozy/ui";

import type { UseAgentDetailResult } from "./use-agent-detail";
import type { AgentDetailTab } from "@/systems/agent";

/** Metrics are exact only once loaded and available; otherwise nothing is inferred. */
export function agentMetricsReady(page: UseAgentDetailResult): boolean {
  return !page.metricsLoading && !page.metricsUnavailable;
}

export function agentDetailTabItems(page: UseAgentDetailResult): LaneTabsItem<AgentDetailTab>[] {
  const metricsReady = agentMetricsReady(page);
  return [
    { value: "overview", label: "Overview", testId: "agent-tab-overview" },
    { value: "instructions", label: "Instructions", testId: "agent-tab-instructions" },
    { value: "configuration", label: "Configuration", testId: "agent-tab-configuration" },
    {
      value: "sessions",
      label: "Sessions",
      // Omit count when metrics are loading/unavailable — never show an inferred zero.
      ...(metricsReady ? { count: page.sessionsTotal } : {}),
      liveLabel: metricsReady && page.activeSessionsTotal > 0 ? "Live" : undefined,
      testId: "agent-tab-sessions",
    },
  ];
}

export function agentSessionsStatus(page: UseAgentDetailResult): "loading" | "error" | "ready" {
  if (page.sessionsLoading) return "loading";
  return page.sessionsError ? "error" : "ready";
}
