import type { ListingViewMode } from "@compozy/ui";

import { normalizeListingSearchValue, parseListingView } from "@/lib/listing-search";
import type { AutomationScopeFilter, AutomationSource } from "../types";

export interface AutomationRouteSearch {
  create?: "loop";
  enabled?: boolean;
  event?: string;
  loop?: string;
  q?: string;
  scope?: AutomationScopeFilter;
  source?: AutomationSource;
  view?: ListingViewMode;
}

export function automationListLoopFilter(search: AutomationRouteSearch): string | undefined {
  return search.create === "loop" ? undefined : search.loop;
}

export function parseAutomationScope(value: unknown): AutomationScopeFilter | undefined {
  return value === "all" || value === "global" || value === "workspace" ? value : undefined;
}

export function parseAutomationSource(value: unknown): AutomationSource | undefined {
  return value === "config" || value === "package" || value === "dynamic" ? value : undefined;
}

export function parseAutomationEnabled(value: unknown): boolean | undefined {
  if (value === true || value === "true") return true;
  if (value === false || value === "false") return false;
  return undefined;
}

function validateAutomationSearch(search: Record<string, unknown>): AutomationRouteSearch {
  return {
    create: search.create === "loop" ? "loop" : undefined,
    enabled: parseAutomationEnabled(search.enabled),
    loop: normalizeListingSearchValue(search.loop),
    q: normalizeListingSearchValue(search.q),
    scope: parseAutomationScope(search.scope),
    source: parseAutomationSource(search.source),
    view: parseListingView(search.view),
  };
}

export function validateJobsSearch(search: Record<string, unknown>): AutomationRouteSearch {
  return validateAutomationSearch(search);
}

export function validateTriggersSearch(search: Record<string, unknown>): AutomationRouteSearch {
  return {
    ...validateAutomationSearch(search),
    event: normalizeListingSearchValue(search.event),
  };
}
