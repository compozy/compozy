import type { InfiniteData } from "@tanstack/react-query";

import type { AgentCatalogItem, AgentCatalogResponse, AgentCatalogStableFilter } from "../types";

function normalizeOptionalText(value?: string | null): string | undefined {
  if (typeof value !== "string") return undefined;
  const normalized = value.trim();
  return normalized === "" ? undefined : normalized;
}

export function normalizeAgentCatalogFilter(
  filters: AgentCatalogStableFilter = {}
): AgentCatalogStableFilter {
  return {
    name: normalizeOptionalText(filters.name),
    q: normalizeOptionalText(filters.q),
    category: normalizeOptionalText(filters.category),
    status: filters.status,
    limit: filters.limit,
  };
}

export function agentCatalogRequest(
  workspace: string,
  filters: AgentCatalogStableFilter,
  cursor?: string
) {
  return {
    workspace: workspace.trim(),
    ...normalizeAgentCatalogFilter(filters),
    ...(cursor ? { cursor } : {}),
  };
}

export function flattenAgentCatalogPages(
  data: InfiniteData<AgentCatalogResponse, unknown> | undefined
): AgentCatalogItem[] {
  const seen = new Set<string>();
  return (
    data?.pages.flatMap(page =>
      page.agents.filter(item => {
        if (seen.has(item.agent.name)) return false;
        seen.add(item.agent.name);
        return true;
      })
    ) ?? []
  );
}

export function agentCatalogPage(
  data: InfiniteData<AgentCatalogResponse, unknown> | undefined
): AgentCatalogResponse | undefined {
  return data?.pages[0];
}
