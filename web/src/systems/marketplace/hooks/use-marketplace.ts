import { useQuery } from "@tanstack/react-query";
import { marketplaceCatalogOptions, marketplaceCatalogEntryOptions } from "../lib/query-options";
import type { MarketplaceCatalogOptions, MarketplaceCatalogEntryOptions } from "../types";

export function useMarketplaceCatalog(options: MarketplaceCatalogOptions = {}, enabled = true) {
  return useQuery(marketplaceCatalogOptions(options, enabled));
}

export function useMarketplaceCatalogEntry(
  options: MarketplaceCatalogEntryOptions,
  enabled = true
) {
  return useQuery(marketplaceCatalogEntryOptions(options, enabled));
}
