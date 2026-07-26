import type { ListingViewMode } from "@agh/ui";

import { normalizeListingSearchValue, parseListingView } from "@/lib/listing-search";

import type { AgentFleetStatus } from "./fleet-signals";

export interface AgentsFleetSearch {
  q?: string;
  category?: string;
  status?: AgentFleetStatus;
  view?: ListingViewMode;
}

export function parseAgentFleetStatusFilter(value: unknown): AgentFleetStatus | undefined {
  return value === "active" || value === "idle" ? value : undefined;
}

export function parseAgentFleetCategoryFilter(value: unknown): string | undefined {
  return normalizeListingSearchValue(value);
}

export function validateAgentsFleetSearch(search: Record<string, unknown>): AgentsFleetSearch {
  return {
    q: normalizeListingSearchValue(search.q),
    category: parseAgentFleetCategoryFilter(search.category),
    status: parseAgentFleetStatusFilter(search.status),
    view: parseListingView(search.view),
  };
}

export function hasActiveAgentFleetFilters(search: AgentsFleetSearch): boolean {
  return Boolean(search.q || search.category || search.status);
}
