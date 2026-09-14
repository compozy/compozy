import { normalizeListingSearchValue } from "@/lib/listing-search";

/** Search state of the one-kind Browse and Installed pages: `?q=` only. */
export interface MarketplaceSearch {
  q?: string;
}

export function validateMarketplaceSearch(search: Record<string, unknown>): MarketplaceSearch {
  return { q: normalizeListingSearchValue(search.q) };
}
