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

export function marketplaceCatalogOptions(options: MarketplaceCatalogOptions = {}, enabled = true) {
  return queryOptions({
    queryKey: marketplaceKeys.catalog(options),
    queryFn: ({ signal }) => readMarketplaceSnapshot(options, signal),
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
