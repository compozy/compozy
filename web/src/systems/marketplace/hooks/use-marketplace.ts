import { useEffect } from "react";
import { isMarketplaceCursorStale } from "../adapters/marketplace-api-error";
import { marketplaceKeys } from "../lib/query-keys";
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";

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
  const client = useQueryClient();
  const query = useInfiniteQuery(marketplaceCatalogOptions(options, enabled));
  const { error, data } = query;
  const restart = isMarketplaceCursorStale(error) && !!data?.pages.length;
  const { q, limit, workspaceId, profileName } = options;
  useEffect(() => {
    if (!enabled || !restart) return;
    // Reset the whole infinite-query envelope before fetching the new first page.
    // Returning page one as a continuation would mix two catalog revisions.
    void client.resetQueries({
      queryKey: marketplaceKeys.catalog({ q, limit, workspaceId, profileName }),
      exact: true,
    });
  }, [client, enabled, restart, q, limit, workspaceId, profileName]);
  return query;
}

export function useMarketplaceCatalogEntry(options: MarketplaceCatalogEntryOptions) {
  return useQuery(marketplaceCatalogEntryOptions(options));
}
