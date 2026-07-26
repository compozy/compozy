import { useMutation, useQueryClient } from "@tanstack/react-query";

import { probeProviderAuth } from "../adapters/providers-api";
import { providerKeys } from "../lib/query-keys";
import type { ProviderAuthProbeResponse } from "../types";

export function useProbeProviderAuth() {
  const queryClient = useQueryClient();
  return useMutation<ProviderAuthProbeResponse, Error, string>({
    mutationFn: providerId => probeProviderAuth(providerId),
    onSettled: (_result, _error, providerId) => {
      const trimmed = providerId?.trim();
      if (!trimmed) {
        return queryClient.invalidateQueries({ queryKey: providerKeys.all });
      }
      return Promise.all([
        queryClient.invalidateQueries({ queryKey: providerKeys.detail(trimmed) }),
        queryClient.invalidateQueries({ queryKey: providerKeys.authProbe(trimmed) }),
        queryClient.invalidateQueries({ queryKey: providerKeys.lists() }),
      ]);
    },
  });
}
