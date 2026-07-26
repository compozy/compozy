import { useMutation, useQueryClient } from "@tanstack/react-query";

import { refreshAllModels } from "../adapters/model-catalog-api";
import { modelCatalogKeys } from "../lib/query-keys";
import type { AllModelsRefreshInput, AllModelsRefreshResponse } from "../types";

/**
 * Refresh every provider's catalog sources through the aggregate endpoint. Used
 * by surfaces that show the whole cross-provider set (All / Favorites), so the
 * visible refresh affordance is truthful: it refreshes the entire catalog rather
 * than silently touching a single provider. Invalidates the whole catalog cache
 * so the aggregate list re-reads the merged projection.
 */
export function useRefreshAllModels() {
  const queryClient = useQueryClient();
  return useMutation<AllModelsRefreshResponse, Error, AllModelsRefreshInput | void>({
    mutationFn: input => refreshAllModels(input ?? {}),
    onSettled: () => queryClient.invalidateQueries({ queryKey: modelCatalogKeys.all }),
  });
}
