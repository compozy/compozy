import { useQuery, useQueryClient } from "@tanstack/react-query";

import { initialModelCatalogReadinessOptions } from "../lib/query-options";

interface InitialModelCatalogRefreshInput {
  enabled: boolean;
  missingAllowedProvider: boolean;
}

/**
 * Complete the daemon's non-blocking catalog warmup once per browser cache.
 * TanStack Query owns the operation so concurrent selectors share one request.
 */
export function useInitialModelCatalogRefresh({
  enabled,
  missingAllowedProvider,
}: InitialModelCatalogRefreshInput) {
  const queryClient = useQueryClient();

  return useQuery(
    initialModelCatalogReadinessOptions(queryClient, enabled && missingAllowedProvider)
  );
}
