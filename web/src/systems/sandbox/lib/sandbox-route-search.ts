import type { ListingViewMode } from "@compozy/ui";

import { normalizeListingSearchValue, parseListingView } from "@/lib/listing-search";
import { parseSandboxBackendFilter, parseSandboxPersistenceFilter } from "./sandbox-list-filters";

export interface SandboxRouteSearch {
  q?: string;
  backend?: string;
  persistence?: string;
  view?: ListingViewMode;
}

export function validateSandboxSearch(search: Record<string, unknown>): SandboxRouteSearch {
  return {
    q: normalizeListingSearchValue(search.q),
    backend: parseSandboxBackendFilter(search.backend),
    persistence: parseSandboxPersistenceFilter(search.persistence),
    view: parseListingView(search.view),
  };
}
