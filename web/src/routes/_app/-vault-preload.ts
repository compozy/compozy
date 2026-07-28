import type { QueryClient } from "@tanstack/react-query";

import { vaultSecretsListOptions, type VaultListFilter } from "@/systems/vault";

import { settleRouteQueries } from "./-route-preload";

export function preloadVaultRoute(
  queryClient: QueryClient,
  filter: VaultListFilter = {}
): Promise<void> {
  return settleRouteQueries([queryClient.ensureQueryData(vaultSecretsListOptions(filter))]);
}
