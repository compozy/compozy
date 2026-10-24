import type { LaneTabsItem } from "@compozy/ui";

import type { UseAgentDetailResult } from "./use-agent-detail";
import type { AgentDetailTab } from "@/systems/agent";

/** Metrics are exact only once loaded and available; otherwise nothing is inferred. */
export function agentMetricsReady(page: UseAgentDetailResult): boolean {
  return !page.metricsLoading && !page.metricsUnavailable;
}

/**
 * What the window head shows in the status slot. Metrics stay disabled until the agent's scope
 * resolves, so an agent that has not arrived yet still holds the placeholder; dropping the slot
 * would cost the trail its right alignment and make it jump when the pill lands.
 */
export function agentStatusSlot(page: UseAgentDetailResult): "pill" | "placeholder" | "none" {
  if (agentMetricsReady(page)) return "pill";
  if (page.metricsLoading || (!page.agent && !page.metricsError)) return "placeholder";
  return "none";
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
