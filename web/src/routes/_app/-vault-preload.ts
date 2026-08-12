import type { QueryClient } from "@tanstack/react-query";

import { settleRouteQueries } from "./-route-preload";
import { type VaultListFilter, vaultSecretsListOptions } from "@/systems/vault";

export function preloadVaultRoute(
  queryClient: QueryClient,
  filter: VaultListFilter = {}
): Promise<void> {
  return settleRouteQueries([queryClient.ensureQueryData(vaultSecretsListOptions(filter))]);
}
