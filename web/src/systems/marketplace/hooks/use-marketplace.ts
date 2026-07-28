import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import {
  marketplaceEntryOptions,
  marketplaceKindOptions,
  marketplaceSearchOptions,
} from "../lib/query-options";
import type {
  MarketplaceEntryOptions,
  MarketplaceKindOptions,
  MarketplaceSearchOptions,
} from "../types";

export function useMarketplaceSearch(options: MarketplaceSearchOptions = {}) {
  return useQuery(marketplaceSearchOptions(options));
}

export function useMarketplaceKind(options: MarketplaceKindOptions) {
  return useInfiniteQuery(marketplaceKindOptions(options));
}

export function useMarketplaceEntry(options: MarketplaceEntryOptions) {
  return useQuery(marketplaceEntryOptions(options));
}
