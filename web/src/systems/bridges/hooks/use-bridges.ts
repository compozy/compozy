import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import {
  bridgeDetailOptions,
  bridgeSecretBindingsOptions,
  bridgeProvidersOptions,
  bridgeRoutesOptions,
  slackBridgeManifestOptions,
  bridgeTargetsOptions,
  bridgesListOptions,
} from "../lib/query-options";
import {
  bridgeHealthFromPages,
  bridgeListPage,
  flattenBridgePages,
} from "../lib/bridge-list-query";
import type { BridgeCatalogFilter, BridgeTargetsQuery } from "../types";
import { useProfileReadScope } from "@/systems/profiles";

/**
 * Bridge instances belong to the profile that created them, so every read here
 * states its lens. The scope is applied at this seam rather than by each caller,
 * so no bridge surface can fall back to an unscoped read the daemon would
 * silently resolve to `default`.
 */
export function useBridges(filters: BridgeCatalogFilter = {}, options?: { enabled?: boolean }) {
  const { params } = useProfileReadScope();
  const query = useInfiniteQuery({
    ...bridgesListOptions({ ...filters, ...params }),
    enabled: options?.enabled ?? true,
  });
  const page = bridgeListPage(query.data);

  return {
    ...query,
    bridges: flattenBridgePages(query.data),
    bridgeHealth: bridgeHealthFromPages(query.data),
    facets: page?.facets,
    total: page?.page.total ?? 0,
  };
}

export function useBridgeProviders(options?: { enabled?: boolean }) {
  return useQuery({
    ...bridgeProvidersOptions(),
    enabled: options?.enabled ?? true,
  });
}

export function useSlackBridgeManifest(instanceID: string, options?: { enabled?: boolean }) {
  const { params } = useProfileReadScope();
  return useQuery(slackBridgeManifestOptions(instanceID, params, options?.enabled));
}

export function useBridge(id: string, options?: { enabled?: boolean }) {
  const { params } = useProfileReadScope();
  return useQuery(bridgeDetailOptions(id, params, options?.enabled));
}

export function useBridgeRoutes(id: string, options?: { enabled?: boolean }) {
  const { params } = useProfileReadScope();
  return useQuery(bridgeRoutesOptions(id, params, options?.enabled));
}

export function useBridgeTargets(
  id: string,
  query: BridgeTargetsQuery = {},
  options?: { enabled?: boolean }
) {
  const { params } = useProfileReadScope();
  return useQuery(bridgeTargetsOptions(id, { ...query, ...params }, options?.enabled));
}

export function useBridgeSecretBindings(id: string, options?: { enabled?: boolean }) {
  const { params } = useProfileReadScope();
  return useQuery(bridgeSecretBindingsOptions(id, params, options?.enabled));
}
