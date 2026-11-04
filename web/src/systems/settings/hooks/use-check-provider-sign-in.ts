import { useMutation, useQueryClient } from "@tanstack/react-query";

import { settingsKeys } from "../lib/query-keys";
import {
  probeProviderAuth,
  providerKeys,
  type ProviderAuthProbeResponse,
} from "@/systems/providers";

/**
 * Runs a provider's live sign-in check. The daemon keeps the verdict, so the
 * provider rows refetch and show it instead of "Sign-in unverified".
 */
export function useCheckProviderSignIn() {
  const queryClient = useQueryClient();
  return useMutation<ProviderAuthProbeResponse, Error, string>({
    mutationFn: providerId => probeProviderAuth(providerId),
    onSettled: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: settingsKeys.providersRoot() }),
        queryClient.invalidateQueries({ queryKey: providerKeys.all }),
      ]),
  });
}
