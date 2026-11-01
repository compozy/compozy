import type { ListingViewMode } from "@compozy/ui";

import { normalizeListingSearchValue, parseListingView } from "@/lib/listing-search";

import type { AgentTeamFocus } from "./agent-team";

export interface AgentsFleetSearch {
  q?: string;
  focus?: Exclude<AgentTeamFocus, "all">;
  project?: string;
  model?: string;
  owner?: string;
  view?: ListingViewMode;
}

function parseFocus(value: unknown): AgentsFleetSearch["focus"] {
  return value === "needs-you" || value === "working" || value === "paused" ? value : undefined;
}

export function validateAgentsFleetSearch(search: Record<string, unknown>): AgentsFleetSearch {
  return {
    q: normalizeListingSearchValue(search.q),
    focus: parseFocus(search.focus),
    project: normalizeListingSearchValue(search.project),
    model: normalizeListingSearchValue(search.model),
    owner: normalizeListingSearchValue(search.owner),
    view: parseListingView(search.view),
  };
}

export function hasActiveAgentFleetFilters(search: AgentsFleetSearch): boolean {
  return Boolean(search.q || search.focus || search.project || search.model || search.owner);
}
