import type { AgentTeamStatus } from "@/systems/agent";

import type { useAgentsFleetPage } from "./use-agents-catalog";

export type AgentsFleetPage = ReturnType<typeof useAgentsFleetPage>;

/** Load failed with nothing cached to show: the page renders the retry state. */
export function isAgentsErrorEmpty(page: AgentsFleetPage): boolean {
  return Boolean(page.agentsError) && page.total === 0 && !page.isLoading;
}

/** Topbar count is shown only once a real, non-empty team total is known. */
export function agentsFleetHeadCount(page: AgentsFleetPage): number | undefined {
  const countUnknown = page.isLoading || isAgentsErrorEmpty(page) || page.isFirstRunEmpty;
  return countUnknown ? undefined : page.total;
}

export function agentFleetListStatus(page: AgentsFleetPage): AgentTeamStatus {
  if (page.isLoading) return "loading";
  if (page.isFirstRunEmpty) return "first-run-empty";
  if (page.isFilteredEmpty) return "filtered-empty";
  return "ready";
}
