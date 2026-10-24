import {
  listMarketplaceSources,
  previewMarketplaceSource,
} from "../adapters/marketplace-sources-api";
import type { AddMarketplaceSourceRequest } from "../types";
import { queryOptions } from "@tanstack/react-query";
import { getMarketplaceCatalogEntry } from "../adapters/marketplace-api";
import { MarketplaceApiError } from "../adapters/marketplace-api-error";
import { marketplaceKeys } from "./query-keys";
import { readMarketplaceSnapshot } from "./catalog-snapshot";
import type { MarketplaceCatalogOptions, MarketplaceCatalogEntryOptions } from "../types";

const MARKETPLACE_STALE_TIME = 60_000;

/**
 * The scope a search edit leaves alone: while an edited search loads, the previous results stay
 * on screen, but never across a workspace, profile or page-size change. Kept in the query `meta`
 * so the rule does not depend on where `q` sits in the key.
 */
function catalogPopulation({ q: _search, ...population }: MarketplaceCatalogOptions): string {
  return JSON.stringify(marketplaceKeys.catalog({ ...population, q: undefined }));
}

export function marketplaceCatalogOptions(options: MarketplaceCatalogOptions = {}, enabled = true) {
  const population = catalogPopulation(options);
  return queryOptions({
    queryKey: marketplaceKeys.catalog(options),
    queryFn: ({ signal }) => readMarketplaceSnapshot(options, signal),
    meta: { population },
    placeholderData: (previousData, previousQuery) =>
      previousQuery?.meta?.population === population ? previousData : undefined,
    // Follow every daemon-owned flight, including healthy sources beside degraded ones.
    refetchInterval: query =>
      query.state.data?.pages.some(page => page.refreshing) ? 1_000 : false,
    staleTime: MARKETPLACE_STALE_TIME,
    retry: (failures, error) =>
      !(error instanceof MarketplaceApiError && error.status < 500) && failures < 2,
    enabled,
  });
}

export function marketplaceCatalogEntryOptions(
  options: MarketplaceCatalogEntryOptions,
  enabled = true
) {
  return queryOptions({
    queryKey: marketplaceKeys.catalogEntry(options),
    queryFn: ({ signal }) => getMarketplaceCatalogEntry(options, signal),
    staleTime: MARKETPLACE_STALE_TIME,
    enabled: enabled && options.entryId.trim() !== "",
  });
}

export function marketplaceSourcesOptions() {
  return queryOptions({
    queryKey: marketplaceKeys.sources(),
    queryFn: ({ signal }) => listMarketplaceSources(signal),
    staleTime: MARKETPLACE_STALE_TIME,
  });
}

export function marketplaceSourcePreviewOptions(body: AddMarketplaceSourceRequest, enabled = true) {
  return queryOptions({
    queryKey: marketplaceKeys.sourcePreview(body.ref, body.name),
    queryFn: ({ signal }) => previewMarketplaceSource(body, signal),
    enabled: enabled && body.ref.trim() !== "",
    retry: false,
    staleTime: 0,
  });
}
