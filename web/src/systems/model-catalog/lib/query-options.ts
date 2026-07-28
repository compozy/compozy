import { queryOptions } from "@tanstack/react-query";

import {
  ModelCatalogApiError,
  getProviderModelStatus,
  listAllModels,
  listProviderModels,
} from "../adapters/model-catalog-api";
import type { AllModelsQuery, ProviderModelsQuery } from "../types";
import { modelCatalogKeys } from "./query-keys";

const STALE_TIME = 30_000;
const REFETCH_INTERVAL = 60_000;
const RETRY_LIMIT = 2;

function shouldRetry(failureCount: number, error: Error): boolean {
  if (error instanceof ModelCatalogApiError && error.status === 403) {
    return false;
  }
  return failureCount < RETRY_LIMIT;
}

export interface ProviderModelsListOptionsArgs extends ProviderModelsQuery {
  enabled?: boolean;
}

export function providerModelsListOptions(args: ProviderModelsListOptionsArgs) {
  const providerId = args.providerId.trim();
  return queryOptions({
    queryKey: modelCatalogKeys.providerModels(
      providerId,
      args.sourceId,
      args.includeStale,
      args.view
    ),
    queryFn: ({ signal }) =>
      listProviderModels(
        { providerId, sourceId: args.sourceId, includeStale: args.includeStale, view: args.view },
        signal
      ),
    enabled: providerId.length > 0 && (args.enabled ?? true),
    staleTime: STALE_TIME,
    refetchInterval: REFETCH_INTERVAL,
    retry: shouldRetry,
  });
}

export interface AllModelsListOptionsArgs extends Omit<AllModelsQuery, "refresh"> {
  enabled?: boolean;
}

export function allModelsListOptions(args: AllModelsListOptionsArgs = {}) {
  const providerId = args.providerId?.trim();
  return queryOptions({
    queryKey: modelCatalogKeys.allModels(args.view, providerId, args.sourceId, args.includeStale),
    queryFn: ({ signal }) =>
      listAllModels(
        {
          providerId,
          sourceId: args.sourceId,
          includeStale: args.includeStale,
          view: args.view,
        },
        signal
      ),
    enabled: args.enabled ?? true,
    staleTime: STALE_TIME,
    refetchInterval: REFETCH_INTERVAL,
    retry: shouldRetry,
  });
}

export interface ProviderModelStatusOptionsArgs {
  providerId: string;
  enabled?: boolean;
}

export function providerModelStatusOptions(args: ProviderModelStatusOptionsArgs) {
  const providerId = args.providerId.trim();
  return queryOptions({
    queryKey: modelCatalogKeys.providerStatus(providerId),
    queryFn: ({ signal }) => getProviderModelStatus(providerId, signal),
    enabled: providerId.length > 0 && (args.enabled ?? true),
    staleTime: STALE_TIME,
    refetchInterval: REFETCH_INTERVAL,
    retry: shouldRetry,
  });
}
