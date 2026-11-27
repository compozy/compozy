import { queryOptions } from "@tanstack/react-query";

import { isRemoteComputerId, LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";

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

export interface ProviderListOptionsArgs {
  computerId?: ComputerId;
  enabled?: boolean;
}

export interface ProviderDetailOptionsArgs {
  providerId: string;
  computerId?: ComputerId;
  enabled?: boolean;
}

export function providerListOptions(args: ProviderListOptionsArgs = {}) {
  const computerId = args.computerId ?? LOCAL_COMPUTER_ID;
  const keys = providerKeys.forComputer(computerId);
  return queryOptions({
    queryKey: keys.lists(),
    queryFn: ({ signal }) =>
      isRemoteComputerId(computerId) ? listProviders(signal, computerId) : listProviders(signal),
    enabled: args.enabled ?? true,
    staleTime: STALE_TIME,
    refetchInterval: REFETCH_INTERVAL,
    retry: shouldRetry,
  });
}

export function providerDetailOptions(args: ProviderDetailOptionsArgs) {
  const providerId = args.providerId.trim();
  const computerId = args.computerId ?? LOCAL_COMPUTER_ID;
  const keys = providerKeys.forComputer(computerId);
  return queryOptions({
    queryKey: keys.detail(providerId),
    queryFn: ({ signal }) =>
      isRemoteComputerId(computerId)
        ? getProvider(providerId, signal, computerId)
        : getProvider(providerId, signal),
    enabled: providerId.length > 0 && (args.enabled ?? true),
    staleTime: STALE_TIME,
    refetchInterval: REFETCH_INTERVAL,
    retry: shouldRetry,
  });
}
