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

/**
 * The Sessions tab's count follows the status slot: a placeholder holds its place while the metrics
 * have not answered, the exact total shows once they have, and it is absent when they never will,
 * so the tab never prints an inferred zero and never gains its count after the others have settled.
 */
function agentSessionsCount(page: UseAgentDetailResult): LaneTabsItem<AgentDetailTab>["count"] {
  switch (agentStatusSlot(page)) {
    case "pill":
      return page.sessionsTotal;
    case "placeholder":
      return "pending";
    case "none":
      return undefined;
  }
}

/**
 * The agent page's tabs. `sessionsCount` is the Sessions tab's count (`"pending"` holds its slot);
 * a frame shown before any read passes `"pending"`, exactly what the page shows while it loads.
 */
export function agentDetailTabs(
  sessionsCount: LaneTabsItem<AgentDetailTab>["count"],
  live = false
): LaneTabsItem<AgentDetailTab>[] {
  return [
    { value: "overview", label: "Overview", testId: "agent-tab-overview" },
    { value: "instructions", label: "Instructions", testId: "agent-tab-instructions" },
    { value: "configuration", label: "Configuration", testId: "agent-tab-configuration" },
    {
      value: "sessions",
      label: "Sessions",
      count: sessionsCount,
      liveLabel: live ? "Live" : undefined,
      testId: "agent-tab-sessions",
    },
  ];
}

export function agentDetailTabItems(page: UseAgentDetailResult): LaneTabsItem<AgentDetailTab>[] {
  return agentDetailTabs(
    agentSessionsCount(page),
    agentMetricsReady(page) && page.activeSessionsTotal > 0
  );
}

export function agentSessionsStatus(page: UseAgentDetailResult): "loading" | "error" | "ready" {
  if (page.sessionsLoading) return "loading";
  return page.sessionsError ? "error" : "ready";
}
