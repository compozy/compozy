import type { QueryClient } from "@tanstack/react-query";

import { resolveActiveWorkspaceId, settleRouteQueries } from "./-route-preload";
import {
  type BridgeCatalogFilter,
  bridgeDetailOptions,
  bridgeListFilterForScope,
  bridgeProvidersOptions,
  bridgeRoutesOptions,
  type BridgeScopeFilter,
  bridgeSecretBindingsOptions,
  bridgesListOptions,
  bridgeTargetsOptions,
} from "@/systems/bridges";
import { readProfileLens, readProfileScopeParams } from "@/systems/profiles";

export async function preloadBridgesRoute(
  queryClient: QueryClient,
  deps: {
    platform?: string;
    q?: string;
    scope: BridgeScopeFilter;
    status?: BridgeCatalogFilter["status"];
  }
): Promise<void> {
  const profileScope = readProfileScopeParams(queryClient, readProfileLens());
  const workspaceId = await resolveActiveWorkspaceId(queryClient);
  const filters: BridgeCatalogFilter = {
    ...bridgeListFilterForScope(deps.scope, workspaceId),
    limit: 50,
    platform: deps.platform,
    q: deps.q,
    sort: "name",
    status: deps.status,
    ...profileScope,
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
  const profileScope = readProfileScopeParams(queryClient, readProfileLens());
  const workspaceId = await resolveActiveWorkspaceId(queryClient);
  const filters = { ...bridgeListFilterForScope("all", workspaceId), ...profileScope };

  await settleRouteQueries([
    queryClient.ensureInfiniteQueryData(bridgesListOptions(filters)),
    queryClient.ensureQueryData(bridgeProvidersOptions()),
    queryClient.ensureQueryData(bridgeDetailOptions(bridgeId, profileScope)),
    queryClient.ensureQueryData(bridgeRoutesOptions(bridgeId, profileScope)),
    queryClient.ensureQueryData(
      bridgeTargetsOptions(bridgeId, { limit: 50, q: "", ...profileScope })
    ),
    queryClient.ensureQueryData(bridgeSecretBindingsOptions(bridgeId, profileScope)),
  ]);
}
