import { normalizeListingSearchValue } from "@/lib/listing-search";

/** Search state of the one-kind Browse and Installed pages: `?q=` only. */
export interface MarketplaceSearch {
  q?: string;
}

export function validateMarketplaceSearch(search: Record<string, unknown>): MarketplaceSearch {
  return { q: normalizeListingSearchValue(search.q) };
}

export const RETIRED_MARKETPLACE_KIND_SEGMENTS = ["skills", "mcps", "extensions"] as const;
export const RETIRED_MARKETPLACE_DETAIL_KINDS = ["skill", "mcp", "extension"] as const;

/** Retired kind list segments (`/marketplace/skills`) redirect to Browse for one release. */
export function isRetiredMarketplaceKindSegment(value: unknown): boolean {
  return (
    typeof value === "string" &&
    (RETIRED_MARKETPLACE_KIND_SEGMENTS as readonly string[]).includes(value)
  );
}

/** Retired kind detail segments (`/marketplace/mcp/github`) redirect to the entry for one release. */
export function isRetiredMarketplaceDetailKind(value: unknown): boolean {
  return (
    typeof value === "string" &&
    (RETIRED_MARKETPLACE_DETAIL_KINDS as readonly string[]).includes(value)
  );
}
