import type { QueryClient } from "@tanstack/react-query";

import {
  bridgeDetailOptions,
  bridgeListFilterForScope,
  bridgeProvidersOptions,
  bridgeRoutesOptions,
  bridgeSecretBindingsOptions,
  bridgeTargetsOptions,
  bridgesListOptions,
  type BridgeCatalogFilter,
  type BridgeScopeFilter,
} from "@/systems/bridges";

import { resolveActiveWorkspaceId, settleRouteQueries } from "./-route-preload";

export async function preloadBridgesRoute(
  queryClient: QueryClient,
  deps: {
    platform?: string;
    q?: string;
    scope: BridgeScopeFilter;
    status?: BridgeCatalogFilter["status"];
  }
): Promise<void> {
  const workspaceId = await resolveActiveWorkspaceId(queryClient);
  const filters: BridgeCatalogFilter = {
    ...bridgeListFilterForScope(deps.scope, workspaceId),
    limit: 50,
    platform: deps.platform,
    q: deps.q,
    sort: "name",
    status: deps.status,
  };
  const enabled = deps.scope !== "workspace" || Boolean(workspaceId);

  const queries: Promise<unknown>[] = [queryClient.ensureQueryData(bridgeProvidersOptions())];
  if (enabled) {
    queries.push(queryClient.ensureInfiniteQueryData(bridgesListOptions(filters)));
  }
  await settleRouteQueries(queries);
}

export async function preloadBridgeDetailRoute(
  queryClient: QueryClient,
  bridgeId: string
): Promise<void> {
  const workspaceId = await resolveActiveWorkspaceId(queryClient);
  const filters = bridgeListFilterForScope("all", workspaceId);

  await settleRouteQueries([
    queryClient.ensureInfiniteQueryData(bridgesListOptions(filters)),
    queryClient.ensureQueryData(bridgeProvidersOptions()),
    queryClient.ensureQueryData(bridgeDetailOptions(bridgeId)),
    queryClient.ensureQueryData(bridgeRoutesOptions(bridgeId)),
    queryClient.ensureQueryData(bridgeTargetsOptions(bridgeId, { limit: 50, q: "" })),
    queryClient.ensureQueryData(bridgeSecretBindingsOptions(bridgeId)),
  ]);
}
