import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import { isMarketplaceCursorStale } from "../adapters/marketplace-api-error";
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
 * One category, paged by the daemon. A cursor that went stale while paging restarts the list from
 * the first page silently (US-002.EC-2): the rows on screen stay while every loaded page is read
 * again with fresh cursors, and the stale answer is never reported as an error.
 */
export function useMarketplaceCategory(options: MarketplaceCatalogOptions, enabled = true) {
  const query = useInfiniteQuery(marketplaceCategoryOptions(options, enabled));
  const loadMore = async () => {
    if (!query.hasNextPage || query.isFetchingNextPage) return;
    const result = await query.fetchNextPage();
    if (isMarketplaceCursorStale(result.error)) await query.refetch();
  };
  const restarting = isMarketplaceCursorStale(query.error);
  return { ...query, error: restarting ? null : query.error, loadMore };
}

export function useMarketplaceCatalogEntry(
  options: MarketplaceCatalogEntryOptions,
  enabled = true
) {
  return useQuery(marketplaceCatalogEntryOptions(options, enabled));
}
