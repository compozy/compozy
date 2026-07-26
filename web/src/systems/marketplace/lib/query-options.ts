import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import {
  browseMarketplaceKind,
  getMarketplaceEntry,
  searchMarketplace,
} from "../adapters/marketplace-api";
import { marketplaceKeys } from "./query-keys";
import type {
  MarketplaceEntryOptions,
  MarketplaceKindOptions,
  MarketplaceSearchOptions,
} from "../types";

const MARKETPLACE_STALE_TIME = 60_000;

export function marketplaceSearchOptions(options: MarketplaceSearchOptions = {}) {
  return queryOptions({
    queryKey: marketplaceKeys.search(options),
    queryFn: ({ signal }) => searchMarketplace(options, signal),
    staleTime: MARKETPLACE_STALE_TIME,
  });
}

export function marketplaceKindOptions(options: MarketplaceKindOptions) {
  return infiniteQueryOptions({
    queryKey: marketplaceKeys.kind(options),
    queryFn: ({ pageParam, signal }) =>
      browseMarketplaceKind({ ...options, cursor: pageParam }, signal),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: page => page.next_cursor || undefined,
    staleTime: MARKETPLACE_STALE_TIME,
  });
}

export function marketplaceEntryOptions(options: MarketplaceEntryOptions) {
  return queryOptions({
    queryKey: marketplaceKeys.detail(options),
    queryFn: ({ signal }) => getMarketplaceEntry(options, signal),
    staleTime: MARKETPLACE_STALE_TIME,
    enabled: options.entryId.trim() !== "",
  });
}
