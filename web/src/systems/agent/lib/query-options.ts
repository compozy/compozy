import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import {
  fetchAgent,
  fetchAgentAggregate,
  fetchAgentCatalog,
  fetchAgents,
} from "../adapters/agent-api";
import {
  fetchAgentActivity,
  fetchAgentInstanceCheckIns,
  fetchAgentInstanceReports,
  fetchAgentInstanceOccurrence,
  fetchAgentInstances,
  fetchAgentInstanceAway,
} from "../adapters/agent-instance-api";
import {
  fetchAgentHeartbeat,
  fetchAgentHeartbeatHistory,
  fetchAgentHeartbeatStatus,
  type FetchAgentHeartbeatStatusParams,
} from "../adapters/agent-heartbeat-api";
import { fetchAgentMemory, fetchAgentMemoryHistory } from "../adapters/agent-memory-api";
import { fetchAgentSoul, fetchAgentSoulHistory } from "../adapters/agent-soul-api";
import { agentKeys } from "./query-keys";
import {
  agentCatalogRequest,
  keepCatalogAcrossSearch,
  normalizeAgentCatalogFilter,
} from "./agent-catalog-query";
import type { AgentCatalogStableFilter, AgentTeamItem } from "../types";

const HEARTBEAT_STATUS_FRESHNESS_MS = 5_000;

const CHECK_INS_SHOWN = 12;
/** Every 5 minutes for a whole day fits. */
const CHECK_INS_PER_DAY = 288;
const OCCURRENCE_POLL_MS = 1_500;
const GROUPED_CATALOG_PAGE = 100;

/** Every instance, across scopes; `agent_instance.updated` keeps it live between reads. */
export function agentInstancesOptions() {
  return queryOptions({
    queryKey: agentKeys.instances(),
    queryFn: ({ signal }) => fetchAgentInstances(signal),
    staleTime: 30_000,
    retry: false,
  });
}

/** One read per visit: the digest holds still until the operator leaves and it's marked seen. */
export function agentInstanceAwayOptions(id: string) {
  return queryOptions({
    queryKey: agentKeys.instanceAway(id),
    queryFn: ({ signal }) => fetchAgentInstanceAway(id, signal),
    staleTime: Infinity,
    gcTime: 0,
    retry: false,
  });
}

export function agentInstanceCheckInsOptions(id: string) {
  return queryOptions({
    queryKey: agentKeys.instanceCheckIns(id),
    queryFn: ({ signal }) => fetchAgentInstanceCheckIns(id, CHECK_INS_SHOWN, signal),
    staleTime: 30_000,
    retry: false,
  });
}

/** All of today's check-ins for Check-ins' strip; the hover card keeps its short read. */
export function agentInstanceDayCheckInsOptions(id: string) {
  return queryOptions({
    queryKey: agentKeys.instanceDayCheckIns(id),
    queryFn: ({ signal }) => fetchAgentInstanceCheckIns(id, CHECK_INS_PER_DAY, signal),
    staleTime: 30_000,
    retry: false,
  });
}

/** Polls a check started by Run it for real until the daemon says it finished. */
export function agentInstanceOccurrenceOptions(id: string, occurrence: string) {
  return queryOptions({
    queryKey: agentKeys.instanceOccurrence(id, occurrence),
    queryFn: ({ signal }) => fetchAgentInstanceOccurrence(id, occurrence, signal),
    retry: false,
    refetchInterval: query =>
      query.state.data?.state === "finished" || query.state.error ? false : OCCURRENCE_POLL_MS,
  });
}

const REPORTS_SHOWN = 10;

export function agentInstanceReportsOptions(id: string) {
  return queryOptions({
    queryKey: agentKeys.instanceReports(id),
    queryFn: ({ signal }) => fetchAgentInstanceReports(id, REPORTS_SHOWN, signal),
    staleTime: 30_000,
    retry: false,
  });
}

async function fetchAgentTeam(workspace: string, profile: string, signal: AbortSignal) {
  const agents: AgentTeamItem[] = [];
  let cursor: string | undefined;
  do {
    const page = await fetchAgentCatalog(
      {
        group: "agent",
        limit: GROUPED_CATALOG_PAGE,
        profile,
        ...(workspace ? { workspace } : {}),
        ...(cursor ? { cursor } : {}),
      },
      signal
    );
    const rows = new Map(
      (page.agents ?? []).map(row => [row.agent.definition_digest, row.sessions])
    );
    agents.push(
      ...(page.items ?? []).map(item => {
        const sessions = rows.get(item.definition_digest);
        return sessions ? { ...item, sessions } : item;
      })
    );
    cursor = page.page.has_more ? page.page.next_cursor : undefined;
  } while (cursor);
  return agents;
}

async function fetchStaysOnAgents(workspace: string, profile: string, signal: AbortSignal) {
  const agents = await fetchAgentTeam(workspace, profile, signal);
  return agents.filter(agent => agent.kind === "stays_on");
}

/** Every agent of a scope, one per definition, across pages; the page filters and counts them. */
export function agentTeamOptions(workspace: string, profile = "default", enabled = true) {
  return queryOptions({
    queryKey: agentKeys.team(workspace, profile),
    queryFn: ({ signal }) => fetchAgentTeam(workspace, profile, signal),
    staleTime: 5_000,
    enabled,
  });
}

/** Activity rolls up by day, so a read stays good for a while. */
export function agentActivityOptions(
  name: string,
  days: number,
  workspace?: string | null,
  profile = "default",
  enabled = true
) {
  return queryOptions({
    queryKey: agentKeys.activity(name, days, workspace, profile),
    queryFn: ({ signal }) => fetchAgentActivity(name, { days, workspace, profile }, signal),
    staleTime: 60_000,
    retry: false,
    enabled: enabled && !!name,
  });
}

/** The stays-on agents a scope resolves; `""` reads the User scope. */
export function staysOnAgentsOptions(workspace: string, profile = "default", enabled = true) {
  return queryOptions({
    queryKey: agentKeys.staysOnCatalog(workspace, profile),
    queryFn: ({ signal }) => fetchStaysOnAgents(workspace, profile, signal),
    staleTime: 5_000,
    enabled,
  });
}

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

export function agentMemoryOptions(name: string, workspace?: string | null, profile = "default") {
  return queryOptions({
    queryKey: agentKeys.memory(name, workspace, profile),
    queryFn: ({ signal }) => fetchAgentMemory(name, workspace, signal, profile),
    staleTime: 30_000,
    enabled: !!name,
  });
}

export function agentMemoryHistoryOptions(
  name: string,
  workspace?: string | null,
  profile = "default"
) {
  return queryOptions({
    queryKey: agentKeys.memoryHistory(name, workspace, profile),
    queryFn: ({ signal }) => fetchAgentMemoryHistory(name, workspace, signal, profile),
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
