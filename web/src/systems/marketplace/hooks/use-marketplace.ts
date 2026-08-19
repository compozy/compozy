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

export function useMarketplaceSearch(options: MarketplaceSearchOptions = {}, enabled = true) {
  return useQuery({ ...marketplaceSearchOptions(options), enabled });
}

export function useMarketplaceKind(options: MarketplaceKindOptions, enabled = true) {
  return useInfiniteQuery(marketplaceKindOptions(options, enabled));
}

export function useMarketplaceEntry(options: MarketplaceEntryOptions) {
  return useQuery(marketplaceEntryOptions(options));
}
