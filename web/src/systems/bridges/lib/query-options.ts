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
import type { ProfileScopeParams } from "@/systems/profiles";
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

export function slackBridgeManifestOptions(
  instanceID: string,
  scope: ProfileScopeParams,
  enabled = true
) {
  return queryOptions({
    queryKey: bridgeKeys.slackManifest(instanceID, scope),
    queryFn: ({ signal }) => getSlackBridgeManifest(instanceID, scope, signal),
    staleTime: DEFAULT_STALE_TIME,
    enabled: Boolean(instanceID) && enabled,
  });
}

export function bridgeDetailOptions(id: string, scope: ProfileScopeParams, enabled = true) {
  return queryOptions({
    queryKey: bridgeKeys.detail(id, scope),
    queryFn: ({ signal }) => getBridge(id, scope, signal),
    staleTime: DEFAULT_STALE_TIME,
    refetchInterval: DEFAULT_REFETCH_INTERVAL,
    enabled: Boolean(id) && enabled,
  });
}

export function bridgeRoutesOptions(id: string, scope: ProfileScopeParams, enabled = true) {
  return queryOptions({
    queryKey: bridgeKeys.routes(id, scope),
    queryFn: ({ signal }) => listBridgeRoutes(id, scope, signal),
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

export function bridgeSecretBindingsOptions(id: string, scope: ProfileScopeParams, enabled = true) {
  return queryOptions({
    queryKey: bridgeKeys.secretBindings(id, scope),
    queryFn: ({ signal }) => listBridgeSecretBindings(id, scope, signal),
    staleTime: DEFAULT_STALE_TIME,
    refetchInterval: DEFAULT_REFETCH_INTERVAL,
    enabled: Boolean(id) && enabled,
  });
}
