import { normalizeListingSearchValue } from "@/lib/listing-search";

export type MarketplaceKindScope = "market" | "installed";

export interface MarketplaceKindSearch {
  config_scope?: "global" | "workspace";
  tab?: "installed";
  q?: string;
}

export function validateMarketplaceKindSearch(
  search: Record<string, unknown>
): MarketplaceKindSearch {
  return {
    config_scope:
      search.config_scope === "global" || search.config_scope === "workspace"
        ? search.config_scope
        : undefined,
    tab: search.tab === "installed" ? "installed" : undefined,
    q: normalizeListingSearchValue(search.q),
  };
}

export function marketplaceKindScopeFromSearch(
  search: MarketplaceKindSearch
): MarketplaceKindScope {
  return search.tab === "installed" ? "installed" : "market";
}

export function marketplaceKindPath(routeKind: string): `/marketplace/${string}` {
  return `/marketplace/${routeKind}`;
}
