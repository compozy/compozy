import type { QueryClient } from "@tanstack/react-query";

import { extensionKeys } from "@/systems/extensions/lib/query-keys";
import { marketplaceKeys } from "@/systems/marketplace/lib/query-keys";
import type {
  MarketplaceEnableFailures,
  MarketplaceEnableTarget,
} from "@/systems/marketplace/types";
import { sessionKeys } from "@/systems/session/lib/query-keys";

export async function reconcileInstalledExtensionCaches(queryClient: QueryClient) {
  const queryKeys = [extensionKeys.all, marketplaceKeys.all, sessionKeys.commandsRoot];
  // Invalidating an initial fetch alone can reuse its pre-mutation response.
  await Promise.all(queryKeys.map(queryKey => queryClient.cancelQueries({ queryKey })));
  return Promise.all(queryKeys.map(queryKey => queryClient.invalidateQueries({ queryKey })));
}

/**
 * An instance that turned on from any surface (Installed, the detail window, a catalog Retry) no
 * longer carries the enable failure an Add recorded for it: the catalog row returns to its live
 * state instead of a stale "Couldn't turn on" once the instance is later turned off.
 */
export function clearMarketplaceEnableFailures(
  queryClient: QueryClient,
  target: MarketplaceEnableTarget
) {
  const matches = (failure: MarketplaceEnableTarget) =>
    failure.name === target.name &&
    failure.profileName === target.profileName &&
    (failure.workspaceId ?? null) === (target.workspaceId ?? null);
  queryClient.setQueriesData<MarketplaceEnableFailures>(
    { queryKey: [...marketplaceKeys.all, "enable-failures"] },
    current => {
      if (!current || !Object.values(current).some(matches)) return current;
      return Object.fromEntries(Object.entries(current).filter(([, failure]) => !matches(failure)));
    }
  );
}
