import { normalizeListingSearchValue } from "@/lib/listing-search";

/** What Browse lists: catalog plugins (the default, never written to the URL) or MCP servers. */
export type MarketplaceKind = "plugins" | "mcps";

/** Search state of Browse (`?kind=` + `?q=`) and Installed (`?q=` only). */
export interface MarketplaceSearch {
  kind?: "mcps";
  q?: string;
}

/** The kind Browse shows; anything but `mcps` is the Plugins default. */
export function marketplaceKind(search: MarketplaceSearch): MarketplaceKind {
  return search.kind === "mcps" ? "mcps" : "plugins";
}

/** Route search validator: keeps `kind` only when it is `mcps`, and a normalized `q`. */
export function validateMarketplaceSearch(search: Record<string, unknown>): MarketplaceSearch {
  const q = normalizeListingSearchValue(search.q);
  return search.kind === "mcps" ? { kind: "mcps", q } : { q };
}
