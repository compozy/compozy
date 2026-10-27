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
    profile: normalizeOptionalText(filters.profile) ?? "default",
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
    workspace: normalizeOptionalText(workspace),
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

/** Aggregate identity: same-named agents in different projects are distinct rows. */
export function agentCatalogItemKey(item: AgentCatalogItem): string {
  return `${item.agent.workspace_id ?? ""}\u0000${item.agent.name}`;
}

export function flattenAgentAggregatePages(
  data: InfiniteData<AgentCatalogResponse, unknown> | undefined
): AgentCatalogItem[] {
  const seen = new Set<string>();
  return (
    data?.pages.flatMap(page =>
      page.agents.filter(item => {
        const key = agentCatalogItemKey(item);
        if (seen.has(key)) return false;
        seen.add(key);
        return true;
      })
    ) ?? []
  );
}

/** The key without its search text: the scope and filters a search edit does not change. */
function catalogPopulation(queryKey: readonly unknown[]): string {
  const { q: _search, ...population } = (queryKey.at(-1) ?? {}) as Record<string, unknown>;
  return JSON.stringify([...queryKey.slice(0, -1), population]);
}

/**
 * Placeholder rule for the catalog queries: while an edited search loads, the rows of the previous
 * search stay on screen instead of blanking into a skeleton. Only the search text may differ: a new
 * workspace, profile, project selection, category or status shows nothing stale.
 */
export function keepCatalogAcrossSearch<TData>(
  previousData: TData | undefined,
  previousKey: readonly unknown[] | undefined,
  nextKey: readonly unknown[]
): TData | undefined {
  if (!previousKey) return undefined;
  return catalogPopulation(previousKey) === catalogPopulation(nextKey) ? previousData : undefined;
}
