import { queryOptions } from "@tanstack/react-query";

import { ProvidersApiError, getProvider, listProviders } from "../adapters/providers-api";
import { providerKeys } from "./query-keys";

const STALE_TIME = 30_000;
const REFETCH_INTERVAL = 60_000;
const RETRY_LIMIT = 2;

function shouldRetry(failureCount: number, error: Error): boolean {
  if (error instanceof ProvidersApiError && error.status >= 400 && error.status < 500) {
    return false;
  }
  return failureCount < RETRY_LIMIT;
}

export interface ProviderDetailOptionsArgs {
  providerId: string;
  enabled?: boolean;
}

export function providerListOptions() {
  return queryOptions({
    queryKey: providerKeys.lists(),
    queryFn: ({ signal }) => listProviders(signal),
    staleTime: STALE_TIME,
    refetchInterval: REFETCH_INTERVAL,
    retry: shouldRetry,
  });
}

export function providerDetailOptions(args: ProviderDetailOptionsArgs) {
  const providerId = args.providerId.trim();
  return queryOptions({
    queryKey: providerKeys.detail(providerId),
    queryFn: ({ signal }) => getProvider(providerId, signal),
    enabled: providerId.length > 0 && (args.enabled ?? true),
    staleTime: STALE_TIME,
    refetchInterval: REFETCH_INTERVAL,
    retry: shouldRetry,
  });
}
