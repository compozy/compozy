import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import { fetchAgent, fetchAgentCatalog, fetchAgents } from "../adapters/agent-api";
import {
  fetchAgentHeartbeat,
  fetchAgentHeartbeatHistory,
  fetchAgentHeartbeatStatus,
  type FetchAgentHeartbeatStatusParams,
} from "../adapters/agent-heartbeat-api";
import { fetchAgentSoul, fetchAgentSoulHistory } from "../adapters/agent-soul-api";
import { agentKeys } from "./query-keys";
import { agentCatalogRequest, normalizeAgentCatalogFilter } from "./agent-catalog-query";
import type { AgentCatalogStableFilter } from "../types";

export function agentCatalogOptions(workspace: string, filters: AgentCatalogStableFilter = {}) {
  const normalizedFilters = normalizeAgentCatalogFilter(filters);
  return infiniteQueryOptions({
    queryKey: agentKeys.catalog(workspace, normalizedFilters),
    queryFn: ({ pageParam, signal }) =>
      fetchAgentCatalog(agentCatalogRequest(workspace, normalizedFilters, pageParam), signal),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: lastPage => (lastPage.page.has_more ? lastPage.page.next_cursor : undefined),
    staleTime: 5_000,
  });
}

export function agentsListOptions(workspace?: string | null) {
  return queryOptions({
    queryKey: agentKeys.list(workspace),
    queryFn: ({ signal }) => fetchAgents(workspace, signal),
    staleTime: 60_000,
  });
}

export function agentDetailOptions(name: string, workspace?: string | null) {
  return queryOptions({
    queryKey: agentKeys.detail(name, workspace),
    queryFn: ({ signal }) => fetchAgent(name, workspace, signal),
    staleTime: 60_000,
    enabled: !!name,
  });
}

export function agentSoulOptions(name: string, workspace?: string | null) {
  return queryOptions({
    queryKey: agentKeys.soul(name, workspace),
    queryFn: ({ signal }) => fetchAgentSoul(name, workspace, signal),
    staleTime: 30_000,
    enabled: !!name,
  });
}

export function agentSoulHistoryOptions(name: string, workspace?: string | null) {
  return queryOptions({
    queryKey: agentKeys.soulHistory(name, workspace),
    queryFn: ({ signal }) => fetchAgentSoulHistory(name, workspace, signal),
    staleTime: 30_000,
    enabled: !!name,
  });
}

export function agentHeartbeatOptions(name: string, workspace?: string | null) {
  return queryOptions({
    queryKey: agentKeys.heartbeat(name, workspace),
    queryFn: ({ signal }) => fetchAgentHeartbeat(name, workspace, signal),
    staleTime: 30_000,
    enabled: !!name,
  });
}

export function agentHeartbeatHistoryOptions(name: string, workspace?: string | null) {
  return queryOptions({
    queryKey: agentKeys.heartbeatHistory(name, workspace),
    queryFn: ({ signal }) => fetchAgentHeartbeatHistory(name, workspace, signal),
    staleTime: 30_000,
    enabled: !!name,
  });
}

export function agentHeartbeatStatusOptions(
  name: string,
  options: FetchAgentHeartbeatStatusParams = {}
) {
  return queryOptions({
    queryKey: agentKeys.heartbeatStatus(name, options),
    queryFn: ({ signal }) => fetchAgentHeartbeatStatus(name, options, signal),
    staleTime: 5_000,
    enabled: !!name,
  });
}
