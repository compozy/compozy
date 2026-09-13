import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import {
  browseMarketplace,
  getMarketplaceCatalogEntry,
  browseMarketplaceKind,
  getMarketplaceEntry,
  searchMarketplace,
} from "../adapters/marketplace-api";
import { marketplaceKeys } from "./query-keys";
import type {
  MarketplaceCatalogOptions,
  MarketplaceCatalogEntryOptions,
  MarketplaceEntryOptions,
  MarketplaceKindOptions,
  MarketplaceSearchOptions,
} from "../types";

import { MarketplaceApiError } from "../adapters/marketplace-api-error";

const MARKETPLACE_STALE_TIME = 60_000;

export function marketplaceSearchOptions(options: MarketplaceSearchOptions = {}, enabled = true) {
  return queryOptions({
    queryKey: marketplaceKeys.search(options),
    queryFn: ({ signal }) => searchMarketplace(options, signal),
    staleTime: MARKETPLACE_STALE_TIME,
    enabled,
  });
}

export function marketplaceKindOptions(options: MarketplaceKindOptions, enabled = true) {
  return infiniteQueryOptions({
    queryKey: marketplaceKeys.kind(options),
    queryFn: ({ pageParam, signal }) =>
      browseMarketplaceKind({ ...options, cursor: pageParam }, signal),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: page => page.next_cursor || undefined,
    staleTime: MARKETPLACE_STALE_TIME,
    enabled,
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

export function marketplaceCatalogOptions(options: MarketplaceCatalogOptions = {}, enabled = true) {
  return infiniteQueryOptions({
    queryKey: marketplaceKeys.catalog(options),
    queryFn: ({ pageParam, signal }) =>
      browseMarketplace({ ...options, cursor: pageParam }, signal),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: page => page.next_cursor || undefined,
    staleTime: MARKETPLACE_STALE_TIME,
    retry: (failures, error) =>
      !(error instanceof MarketplaceApiError && error.status < 500) && failures < 2,
    enabled,
  });
}

export function marketplaceCatalogEntryOptions(options: MarketplaceCatalogEntryOptions) {
  return queryOptions({
    queryKey: marketplaceKeys.catalogEntry(options),
    queryFn: ({ signal }) => getMarketplaceCatalogEntry(options, signal),
    staleTime: MARKETPLACE_STALE_TIME,
    enabled: options.entryId.trim() !== "",
  });
}
