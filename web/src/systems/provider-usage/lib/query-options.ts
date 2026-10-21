import { queryOptions } from "@tanstack/react-query";

import { getProviderUsage, ProviderUsageApiError } from "../adapters/provider-usage-api";

import { providerUsageKeys } from "./query-keys";

const PROVIDER_USAGE_RETRY_LIMIT = 2;

/** Retries transient reads; an unreachable daemon surfaces at once as the banner. */
export function shouldRetryProviderUsageQuery(failureCount: number, error: unknown): boolean {
  if (error instanceof ProviderUsageApiError && (error.unreachable || error.status < 500)) {
    return false;
  }
  return failureCount < PROVIDER_USAGE_RETRY_LIMIT;
}

/**
 * The profile's cached usage envelope. The stream owns freshness: every
 * `ready` and `provider_usage_changed` frame invalidates this key.
 */
export function providerUsageOptions(profile: string) {
  return queryOptions({
    queryKey: providerUsageKeys.envelope(profile),
    queryFn: ({ signal }) => getProviderUsage(profile, signal),
    staleTime: 0,
    retry: shouldRetryProviderUsageQuery,
  });
}
