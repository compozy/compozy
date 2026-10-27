import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import {
  fetchAgent,
  fetchAgentAggregate,
  fetchAgentCatalog,
  fetchAgents,
} from "../adapters/agent-api";
import {
  fetchAgentHeartbeat,
  fetchAgentHeartbeatHistory,
  fetchAgentHeartbeatStatus,
  type FetchAgentHeartbeatStatusParams,
} from "../adapters/agent-heartbeat-api";
import { fetchAgentSoul, fetchAgentSoulHistory } from "../adapters/agent-soul-api";
import { agentKeys } from "./query-keys";
import {
  agentCatalogRequest,
  keepCatalogAcrossSearch,
  normalizeAgentCatalogFilter,
} from "./agent-catalog-query";
import type { AgentCatalogStableFilter } from "../types";

const HEARTBEAT_STATUS_FRESHNESS_MS = 5_000;

export function agentCatalogOptions(workspace: string, filters: AgentCatalogStableFilter = {}) {
  const normalizedFilters = normalizeAgentCatalogFilter(filters);
  const queryKey = agentKeys.catalog(workspace, normalizedFilters);
  return infiniteQueryOptions({
    queryKey,
    queryFn: ({ pageParam, signal }) =>
      fetchAgentCatalog(agentCatalogRequest(workspace, normalizedFilters, pageParam), signal),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: lastPage => (lastPage.page.has_more ? lastPage.page.next_cursor : undefined),
    placeholderData: (previousData, previousQuery) =>
      keepCatalogAcrossSearch(previousData, previousQuery?.queryKey, queryKey),
    staleTime: 5_000,
  });
}

/** Every project's agents plus User's for one profile, narrowed by the Project filter. */
export function agentAggregateOptions(
  workspaceIds: readonly string[],
  filters: AgentCatalogStableFilter = {}
) {
  const normalizedFilters = normalizeAgentCatalogFilter(filters);
  const ids = [...new Set(workspaceIds)].sort();
  const queryKey = agentKeys.aggregate(ids, normalizedFilters);
  return infiniteQueryOptions({
    queryKey,
    queryFn: ({ pageParam, signal }) =>
      fetchAgentAggregate(
        {
          ...normalizedFilters,
          ...(ids.length > 0 ? { workspace_ids: ids } : {}),
          ...(pageParam ? { cursor: pageParam } : {}),
        },
        signal
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: lastPage => (lastPage.page.has_more ? lastPage.page.next_cursor : undefined),
    placeholderData: (previousData, previousQuery) =>
      keepCatalogAcrossSearch(previousData, previousQuery?.queryKey, queryKey),
    staleTime: 5_000,
  });
}

export function agentsListOptions(workspace?: string | null, profile = "default") {
  return queryOptions({
    queryKey: agentKeys.list(workspace, profile),
    queryFn: ({ signal }) => fetchAgents(workspace, signal, profile),
    staleTime: 60_000,
  });
}

export function agentDetailOptions(name: string, workspace?: string | null, profile = "default") {
  return queryOptions({
    queryKey: agentKeys.detail(name, workspace, profile),
    queryFn: ({ signal }) => fetchAgent(name, workspace, signal, profile),
    staleTime: 60_000,
    enabled: !!name,
  });
}

export function agentSoulOptions(name: string, workspace?: string | null, profile = "default") {
  return queryOptions({
    queryKey: agentKeys.soul(name, workspace, profile),
    queryFn: ({ signal }) => fetchAgentSoul(name, workspace, signal, profile),
    staleTime: 30_000,
    enabled: !!name,
  });
}

export function agentSoulHistoryOptions(
  name: string,
  workspace?: string | null,
  profile = "default"
) {
  return queryOptions({
    queryKey: agentKeys.soulHistory(name, workspace, profile),
    queryFn: ({ signal }) => fetchAgentSoulHistory(name, workspace, signal, profile),
    staleTime: 30_000,
    enabled: !!name,
  });
}

export function agentHeartbeatOptions(
  name: string,
  workspace?: string | null,
  profile = "default"
) {
  return queryOptions({
    queryKey: agentKeys.heartbeat(name, workspace, profile),
    queryFn: ({ signal }) => fetchAgentHeartbeat(name, workspace, signal, profile),
    staleTime: 30_000,
    enabled: !!name,
  });
}

export function agentHeartbeatHistoryOptions(
  name: string,
  workspace?: string | null,
  profile = "default"
) {
  return queryOptions({
    queryKey: agentKeys.heartbeatHistory(name, workspace, profile),
    queryFn: ({ signal }) => fetchAgentHeartbeatHistory(name, workspace, signal, profile),
    staleTime: 30_000,
    enabled: !!name,
  });
}

export function agentHeartbeatStatusOptions(
  name: string,
  options: FetchAgentHeartbeatStatusParams = {}
) {
  const scopedOptions = { ...options, profile: options.profile ?? "default" };
  return queryOptions({
    queryKey: agentKeys.heartbeatStatus(name, scopedOptions),
    queryFn: ({ signal }) => fetchAgentHeartbeatStatus(name, scopedOptions, signal),
    staleTime: HEARTBEAT_STATUS_FRESHNESS_MS,
    refetchInterval: options.sessionId ? HEARTBEAT_STATUS_FRESHNESS_MS : false,
    enabled: !!name,
  });
}
