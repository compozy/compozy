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

export function useBridges(filters: BridgeCatalogFilter = {}, options?: { enabled?: boolean }) {
  const query = useInfiniteQuery({
    ...bridgesListOptions(filters),
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

export function useBridgeProviders() {
  return useQuery(bridgeProvidersOptions());
}

export function useSlackBridgeManifest(instanceID: string, options?: { enabled?: boolean }) {
  return useQuery(slackBridgeManifestOptions(instanceID, options?.enabled));
}

export function useBridge(id: string, options?: { enabled?: boolean }) {
  return useQuery(bridgeDetailOptions(id, options?.enabled));
}

export function useBridgeRoutes(id: string, options?: { enabled?: boolean }) {
  return useQuery(bridgeRoutesOptions(id, options?.enabled));
}

export function useBridgeTargets(
  id: string,
  query: BridgeTargetsQuery = {},
  options?: { enabled?: boolean }
) {
  return useQuery(bridgeTargetsOptions(id, query, options?.enabled));
}

export function useBridgeSecretBindings(id: string, options?: { enabled?: boolean }) {
  return useQuery(bridgeSecretBindingsOptions(id, options?.enabled));
}
