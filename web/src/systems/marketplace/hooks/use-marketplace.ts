import { useInfiniteQuery, useIsFetching, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRef } from "react";

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
 * Retry instead of holding outdated rows without one. Load more does nothing while a restart runs,
 * waits for any other fetch of the category to settle before asking for the next page, and admits
 * one request at a time. It reads the query cache at call time, so neither the button nor a sentinel
 * firing before React re-renders can cancel a running fetch or append to pages it is replacing.
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
  const loadingMore = useRef(false);
  const loadMore = async () => {
    if (!enabled || !hasNextPage || isFetchingNextPage || loadingMore.current) return;
    loadingMore.current = true;
    try {
      const query = queryClient
        .getQueryCache()
        .find({ queryKey: restartFilters.queryKey, exact: true });
      if (query?.state.fetchStatus === "fetching") {
        // A restart shows on the control; a running next-page fetch is this same request.
        if (queryClient.isFetching(restartFilters) > 0 || query.state.fetchMeta?.fetchMore) return;
        await query.promise?.catch(() => undefined);
        // A refetch that met a stale cursor hands over to a restart; Load more waits for that too.
        if (query.state.fetchStatus === "fetching") return;
      }
      await fetchNextPage();
    } finally {
      loadingMore.current = false;
    }
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
