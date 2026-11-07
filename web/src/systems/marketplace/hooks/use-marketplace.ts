import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import {
  marketplaceCatalogOptions,
  marketplaceCatalogEntryOptions,
  marketplaceCategoryOptions,
  marketplaceSectionsOptions,
} from "../lib/query-options";
import type {
  MarketplaceCatalogOptions,
  MarketplaceCatalogEntryOptions,
  MarketplaceSectionsOptions,
} from "../types";

export function useMarketplaceCatalog(options: MarketplaceCatalogOptions = {}, enabled = true) {
  return useQuery(marketplaceCatalogOptions(options, enabled));
}

export function useMarketplaceSections(options: MarketplaceSectionsOptions = {}, enabled = true) {
  return useQuery(marketplaceSectionsOptions(options, enabled));
}

/**
 * One category, paged by the daemon. A cursor that went stale — while paging or during any
 * background refetch — restarts the list from the first page silently (US-002.EC-2): the rows on
 * screen stay while every loaded page is read again with fresh cursors, and the stale answer is
 * not reported as an error. A restart that itself ends stale exposes that error, so the page offers
 * Retry instead of holding outdated rows without one.
 */
export function useMarketplaceCategory(options: MarketplaceCatalogOptions, enabled = true) {
  const query = useInfiniteQuery(marketplaceCategoryOptions(options, enabled));
  const loadMore = async () => {
    if (!query.hasNextPage || query.isFetchingNextPage) return;
    await query.fetchNextPage();
  };
  return { ...query, loadMore };
}

export function useMarketplaceCatalogEntry(
  options: MarketplaceCatalogEntryOptions,
  enabled = true
) {
  return useQuery(marketplaceCatalogEntryOptions(options, enabled));
}
