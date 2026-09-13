import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import {
  marketplaceCatalogOptions,
  marketplaceCatalogEntryOptions,
  marketplaceEntryOptions,
  marketplaceKindOptions,
  marketplaceSearchOptions,
} from "../lib/query-options";
import type {
  MarketplaceCatalogOptions,
  MarketplaceCatalogEntryOptions,
  MarketplaceEntryOptions,
  MarketplaceKindOptions,
  MarketplaceSearchOptions,
} from "../types";

export function useMarketplaceSearch(options: MarketplaceSearchOptions = {}, enabled = true) {
  return useQuery(marketplaceSearchOptions(options, enabled));
}

export function useMarketplaceKind(options: MarketplaceKindOptions, enabled = true) {
  return useInfiniteQuery(marketplaceKindOptions(options, enabled));
}

export function useMarketplaceEntry(options: MarketplaceEntryOptions) {
  return useQuery(marketplaceEntryOptions(options));
}

export function useMarketplaceCatalog(options: MarketplaceCatalogOptions = {}, enabled = true) {
  return useQuery(marketplaceCatalogOptions(options, enabled));
}

export function useMarketplaceCatalogEntry(
  options: MarketplaceCatalogEntryOptions,
  enabled = true
) {
  return useQuery(marketplaceCatalogEntryOptions(options, enabled));
}
