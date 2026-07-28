import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import {
  getBridge,
  listBridgeSecretBindings,
  listBridgeProviders,
  listBridgeRoutes,
  listBridgeTargets,
  listBridges,
} from "../adapters/bridges-api";
import { getSlackBridgeManifest } from "../adapters/bridge-setup-api";
import { bridgeKeys } from "./query-keys";
import { bridgeListRequest, normalizeBridgeCatalogFilter } from "./bridge-list-query";
import type { BridgeCatalogFilter, BridgeTargetsQuery } from "../types";

const DEFAULT_STALE_TIME = 15_000;
const DEFAULT_REFETCH_INTERVAL = 30_000;
const PROVIDERS_REFETCH_INTERVAL = 60_000;

export function bridgesListOptions(filters: BridgeCatalogFilter = {}) {
  const normalizedFilters = normalizeBridgeCatalogFilter(filters);
  return infiniteQueryOptions({
    queryKey: bridgeKeys.list(normalizedFilters),
    queryFn: ({ pageParam, signal }) =>
      listBridges(bridgeListRequest(normalizedFilters, pageParam), signal),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: lastPage => (lastPage.page.has_more ? lastPage.page.next_cursor : undefined),
    staleTime: DEFAULT_STALE_TIME,
  });
}

export function bridgeProvidersOptions() {
  return queryOptions({
    queryKey: bridgeKeys.providers(),
    queryFn: ({ signal }) => listBridgeProviders(signal),
    staleTime: DEFAULT_STALE_TIME,
    refetchInterval: PROVIDERS_REFETCH_INTERVAL,
  });
}

export function slackBridgeManifestOptions(instanceID: string, enabled = true) {
  const normalizedInstanceID = instanceID.trim();
  return queryOptions({
    queryKey: bridgeKeys.slackManifest(normalizedInstanceID),
    queryFn: ({ signal }) => getSlackBridgeManifest(normalizedInstanceID, signal),
    staleTime: DEFAULT_STALE_TIME,
    enabled: Boolean(normalizedInstanceID) && enabled,
  });
}

export function bridgeDetailOptions(id: string, enabled = true) {
  return queryOptions({
    queryKey: bridgeKeys.detail(id),
    queryFn: ({ signal }) => getBridge(id, signal),
    staleTime: DEFAULT_STALE_TIME,
    refetchInterval: DEFAULT_REFETCH_INTERVAL,
    enabled: Boolean(id) && enabled,
  });
}

export function bridgeRoutesOptions(id: string, enabled = true) {
  return queryOptions({
    queryKey: bridgeKeys.routes(id),
    queryFn: ({ signal }) => listBridgeRoutes(id, signal),
    staleTime: DEFAULT_STALE_TIME,
    refetchInterval: DEFAULT_REFETCH_INTERVAL,
    enabled: Boolean(id) && enabled,
  });
}

export function bridgeTargetsOptions(id: string, query: BridgeTargetsQuery = {}, enabled = true) {
  return queryOptions({
    queryKey: bridgeKeys.targets(id, query),
    queryFn: ({ signal }) => listBridgeTargets(id, query, signal),
    staleTime: DEFAULT_STALE_TIME,
    refetchInterval: DEFAULT_REFETCH_INTERVAL,
    enabled: Boolean(id) && enabled,
  });
}

export function bridgeSecretBindingsOptions(id: string, enabled = true) {
  return queryOptions({
    queryKey: bridgeKeys.secretBindings(id),
    queryFn: ({ signal }) => listBridgeSecretBindings(id, signal),
    staleTime: DEFAULT_STALE_TIME,
    refetchInterval: DEFAULT_REFETCH_INTERVAL,
    enabled: Boolean(id) && enabled,
  });
}
