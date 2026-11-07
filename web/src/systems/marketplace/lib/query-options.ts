import {
  listMarketplaceSources,
  previewMarketplaceSource,
} from "../adapters/marketplace-sources-api";
import type { AddMarketplaceSourceRequest } from "../types";
import { infiniteQueryOptions, queryOptions, skipToken } from "@tanstack/react-query";
import {
  browseMarketplace,
  getMarketplaceCatalogEntry,
  getMarketplaceSections,
} from "../adapters/marketplace-api";
import { MarketplaceApiError } from "../adapters/marketplace-api-error";
import { marketplaceKeys } from "./query-keys";
import { readMarketplaceSnapshot } from "./catalog-snapshot";
import type {
  MarketplaceCatalogOptions,
  MarketplaceCatalogEntryOptions,
  MarketplaceDestinationScope,
  MarketplaceEnableFailures,
  MarketplaceSectionsOptions,
} from "../types";

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
    retry: retryMarketplaceRead,
    enabled,
  });
}

function retryMarketplaceRead(failures: number, error: Error) {
  return !(error instanceof MarketplaceApiError && error.status < 500) && failures < 2;
}

/**
 * Browse home: one daemon read for Featured and every non-empty category, with one revision and
 * one stale flag. A search edit keeps the previous sections until the new answer arrives.
 */
export function marketplaceSectionsOptions(
  options: MarketplaceSectionsOptions = {},
  enabled = true
) {
  const population = JSON.stringify(marketplaceKeys.sections({ ...options, q: undefined }));
  return queryOptions({
    queryKey: marketplaceKeys.sections(options),
    queryFn: ({ signal }) => getMarketplaceSections(options, signal),
    meta: { population },
    placeholderData: (previousData, previousQuery) =>
      previousQuery?.meta?.population === population ? previousData : undefined,
    refetchInterval: query => (query.state.data?.refreshing ? 1_000 : false),
    staleTime: MARKETPLACE_STALE_TIME,
    retry: retryMarketplaceRead,
    enabled,
  });
}

/** One category, daemon-paged by cursor; the cursor stays in `pageParam`. */
export function marketplaceCategoryOptions(options: MarketplaceCatalogOptions, enabled = true) {
  const population = JSON.stringify(marketplaceKeys.category({ ...options, q: undefined }));
  return infiniteQueryOptions({
    queryKey: marketplaceKeys.category(options),
    queryFn: ({ pageParam, signal }) =>
      browseMarketplace({ ...options, limit: options.limit ?? 50, cursor: pageParam }, signal),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: lastPage => lastPage.next_cursor || undefined,
    meta: { population },
    placeholderData: (previousData, previousQuery) =>
      previousQuery?.meta?.population === population ? previousData : undefined,
    // Follow the daemon's refresh flight like Browse does, so a starting catalog fills in.
    refetchInterval: query =>
      query.state.data?.pages.some(page => page.refreshing) ? 1_000 : false,
    staleTime: MARKETPLACE_STALE_TIME,
    retry: retryMarketplaceRead,
    enabled: enabled && (options.category?.trim() ?? "") !== "",
  });
}

/**
 * Enable failures of one destination. The controller writes them with `setQueryData`; nothing
 * fetches them (`skipToken`), so a marketplace invalidation leaves them alone and they outlive
 * the page that recorded them until a Retry or Turn on succeeds.
 */
export function marketplaceEnableFailuresOptions(scope: MarketplaceDestinationScope) {
  return queryOptions<MarketplaceEnableFailures>({
    queryKey: marketplaceKeys.enableFailures(scope),
    queryFn: skipToken,
    staleTime: Infinity,
    gcTime: Infinity,
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
