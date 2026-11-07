import { useInfiniteQuery, useIsFetching, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  marketplaceCatalogOptions,
  marketplaceCatalogEntryOptions,
  marketplaceCategoryOptions,
  marketplaceCategoryRestartFilters,
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
 * Retry instead of holding outdated rows without one. Load more waits while a restart runs; it reads
 * the query cache at call time, so a sentinel firing before React renders the restart cannot cancel it.
 *
 * Only the fields the page reads are returned, so the query keeps tracking just those and a
 * background fetch-status change does not re-render the list.
 */
export function useMarketplaceCategory(options: MarketplaceCatalogOptions, enabled = true) {
  const {
    data,
    error,
    fetchNextPage,
    hasNextPage,
    isFetchNextPageError,
    isFetchingNextPage,
    isPending,
    isPlaceholderData,
    refetch,
  } = useInfiniteQuery(marketplaceCategoryOptions(options, enabled));
  const queryClient = useQueryClient();
  const restartFilters = marketplaceCategoryRestartFilters(options);
  const isRestarting = useIsFetching(restartFilters) > 0;
  const loadMore = async () => {
    if (!hasNextPage || isFetchingNextPage) return;
    if (queryClient.isFetching(restartFilters) > 0) return;
    await fetchNextPage();
  };
  return {
    data,
    error,
    hasNextPage,
    isFetchNextPageError,
    isFetchingNextPage,
    isPending,
    isPlaceholderData,
    isRestarting,
    loadMore,
    refetch,
  };
}

export function useMarketplaceCatalogEntry(
  options: MarketplaceCatalogEntryOptions,
  enabled = true
) {
  return useQuery(marketplaceCatalogEntryOptions(options, enabled));
}
