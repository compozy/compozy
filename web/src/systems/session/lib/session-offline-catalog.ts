import type { QueryClient } from "@tanstack/react-query";

import type { ComputerId } from "@/lib/computer-scope";

import type { SessionCatalogPageResponse, SessionListFilters, SessionPayload } from "../types";
import { sessionKeys } from "./query-keys";
import { matchesOfflineSessionQuery } from "./session-list-computer-filter";
import { normalizeSessionListFilters } from "./session-list-query";
import { withSessionComputer } from "./session-row-ref";

function extractCachedSessions(data: unknown): readonly SessionPayload[] {
  if (!data || typeof data !== "object") return [];
  const candidate = data as {
    pages?: readonly SessionCatalogPageResponse[];
    sessions?: readonly SessionPayload[];
  };
  if (Array.isArray(candidate.pages)) {
    return candidate.pages.flatMap(page => page.sessions);
  }
  if (Array.isArray(candidate.sessions)) {
    return candidate.sessions;
  }
  return [];
}

/** True when any catalog page for `computerId` holds at least one session in memory (Rule 23). */
export function hasCachedRowsForComputer(
  queryClient: QueryClient,
  computerId: ComputerId
): boolean {
  const keys = sessionKeys.forComputer(computerId);
  const queries = queryClient.getQueriesData({ queryKey: keys.lists() });
  for (const [, data] of queries) {
    if (extractCachedSessions(data).length > 0) return true;
  }
  return false;
}

/**
 * Reads cached rows previously seen for an offline computer from the query cache
 * (deduplicated by session id) and filters them by `search` when non-empty (`R11`).
 */
export function readCachedOfflineRowsForComputer(
  queryClient: QueryClient | undefined,
  computerId: ComputerId,
  filters: SessionListFilters
): SessionPayload[] {
  if (!queryClient) return [];
  const keys = sessionKeys.forComputer(computerId);
  const queries = queryClient.getQueriesData({ queryKey: keys.lists() });
  const byId = new Map<string, SessionPayload>();
  const wanted = normalizeSessionListFilters(filters);
  const latest = [...queries].sort(
    ([left], [right]) =>
      (queryClient.getQueryState(right)?.dataUpdatedAt ?? 0) -
      (queryClient.getQueryState(left)?.dataUpdatedAt ?? 0)
  );
  for (const [key, data] of latest) {
    const cached = key[6];
    if ((key[5] !== "page" && key[5] !== "board") || !cached || typeof cached !== "object")
      continue;
    if (!compatibleOfflinePopulation(cached as SessionListFilters, wanted)) continue;
    for (const session of extractCachedSessions(data)) {
      if (!byId.has(session.id) && matchesOfflineSessionQuery(session, wanted.q ?? "")) {
        byId.set(session.id, withSessionComputer(session, computerId));
      }
    }
  }
  return [...byId.values()];
}

function compatibleOfflinePopulation(cached: SessionListFilters, wanted: SessionListFilters) {
  if (cached.q && cached.q !== wanted.q) return false;
  if (wanted.profile && cached.profile !== wanted.profile) return false;
  if (wanted.folder && cached.folder !== wanted.folder) return false;
  if (wanted.workspace_id && cached.workspace_id !== wanted.workspace_id) return false;
  if (wanted.workspace_ids?.length) {
    const matches = new Set(wanted.workspace_ids);
    const keys = cached.workspace_ids ?? (cached.workspace_id ? [cached.workspace_id] : []);
    if (keys.length === 0 || keys.some(key => !matches.has(key))) return false;
  }
  const population = ({
    q: _query,
    search_fields: _fields,
    sort: _sort,
    limit: _limit,
    include_health: _health,
    skip_total: _total,
    workspace_id: _workspace,
    workspace_ids: _workspaces,
    all_workspaces: _allWorkspaces,
    profile: _profile,
    all_profiles: _allProfiles,
    subagents: _subagents,
    folder: _folder,
    ...filters
  }: SessionListFilters) => filters;
  if (wanted.subagents === "exclude" && cached.subagents !== "exclude") return false;
  if (wanted.all_profiles && !cached.profile && !cached.all_profiles) return false;
  return JSON.stringify(population(cached)) === JSON.stringify(population(wanted));
}
