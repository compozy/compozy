import type { useAgentsFleetPage } from "./use-agents-catalog";

export type AgentsFleetPage = ReturnType<typeof useAgentsFleetPage>;

export type AgentFleetListStatus = "loading" | "first-run-empty" | "filtered-empty" | "ready";

/** Load failed with nothing cached to show: the page renders the retry state. */
export function isAgentsErrorEmpty(page: AgentsFleetPage): boolean {
  return Boolean(page.agentsError) && page.agents.length === 0 && !page.isLoading;
}

/** Topbar count is shown only once a real, non-empty fleet total is known. */
export function agentsFleetHeadCount(page: AgentsFleetPage): number | undefined {
  const countUnknown =
    page.isLoading || isAgentsErrorEmpty(page) || page.isFirstRunEmpty || page.workspaceId === "";
  return countUnknown ? undefined : page.fleetTotal;
}

export function agentFleetListStatus(page: AgentsFleetPage): AgentFleetListStatus {
  if (page.isLoading) return "loading";
  if (page.isFirstRunEmpty) return "first-run-empty";
  if (page.isFilteredEmpty) return "filtered-empty";
  return "ready";
}
