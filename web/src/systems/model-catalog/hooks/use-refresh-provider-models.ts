import { useMutation, useQueryClient } from "@tanstack/react-query";

import { refreshProviderModels } from "../adapters/model-catalog-api";
import { modelCatalogKeys } from "../lib/query-keys";
import type { ProviderModelsRefreshInput, ProviderModelsRefreshResponse } from "../types";

export function useRefreshProviderModels() {
  const queryClient = useQueryClient();
  return useMutation<ProviderModelsRefreshResponse, Error, ProviderModelsRefreshInput>({
    mutationFn: input => refreshProviderModels(input),
    onSettled: (_result, _error, variables) => {
      const providerId = variables?.providerId.trim();
      if (!providerId) {
        return queryClient.invalidateQueries({ queryKey: modelCatalogKeys.all });
      }
      return queryClient.invalidateQueries({ queryKey: modelCatalogKeys.providerRoot(providerId) });
    },
  });
}
