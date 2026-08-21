import { normalizeListingSearchValue } from "@/lib/listing-search";

export type MarketplaceKindScope = "market" | "installed";

export interface MarketplaceKindSearch {
  tab?: "market";
  q?: string;
}

export function validateMarketplaceKindSearch(
  search: Record<string, unknown>
): MarketplaceKindSearch {
  return {
    tab: search.tab === "market" ? "market" : undefined,
    q: normalizeListingSearchValue(search.q),
  };
}

export function marketplaceKindScopeFromSearch(
  search: MarketplaceKindSearch
): MarketplaceKindScope {
  return search.tab === "market" ? "market" : "installed";
}

export function marketplaceKindPath(
  routeKind: "extensions" | "mcps" | "skills"
): "/marketplace/extensions" | "/marketplace/mcps" | "/marketplace/skills" {
  return `/marketplace/${routeKind}`;
}
