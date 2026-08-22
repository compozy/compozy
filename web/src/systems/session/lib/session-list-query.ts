import type { InfiniteData } from "@tanstack/react-query";

import type { SessionListFilters, SessionPayload, SessionsQuery, SessionsResponse } from "../types";

function normalizedText(value: string | undefined): string | undefined {
  const normalized = value?.trim();
  return normalized ? normalized : undefined;
}

export function normalizeSessionListFilters(filters: SessionListFilters = {}): SessionListFilters {
  const workspaceId = normalizedText(filters.workspace_id);
  const normalized: SessionListFilters = workspaceId
    ? { workspace_id: workspaceId }
    : { all_workspaces: true };
  const agent = normalizedText(filters.agent);
  const search = normalizedText(filters.q);
  // Server-side scoping: a worktree selection filters on the bound worktree id
  // rather than trimming a loaded page client-side.
  const worktree = normalizedText(filters.worktree);

  if (worktree) normalized.worktree = worktree;
  if (filters.include_health !== undefined) {
    normalized.include_health = filters.include_health;
  }
  if (filters.state !== undefined) normalized.state = filters.state;
  if (filters.type !== undefined) normalized.type = filters.type;
  if (agent) normalized.agent = agent;
  if (search) normalized.q = search;
  if (filters.resumable !== undefined) normalized.resumable = filters.resumable;
  // Attention scoping is server-side too: the daemon owns the needs-you class
  // and the badge vocabulary. Both are part of the query key, so dropping them
  // here would also collapse distinct queries onto one cache entry.
  if (filters.attention !== undefined) normalized.attention = filters.attention;
  const badge = normalizedText(filters.badge);
  if (badge) normalized.badge = badge;
  if (filters.archive !== undefined) normalized.archive = filters.archive;
  if (filters.sort !== undefined) normalized.sort = filters.sort;
  if (filters.limit !== undefined) normalized.limit = filters.limit;
  // Profile scope is part of the request, so it is part of the key: two profiles
  // reading the same workspace are two catalogs, never one shared cache entry.
  if (filters.all_profiles === true) normalized.all_profiles = true;
  else {
    const profile = normalizedText(filters.profile);
    if (profile) normalized.profile = profile;
  }

  return normalized;
}

export function sessionListRequest(filters: SessionListFilters, cursor?: string): SessionsQuery {
  return cursor ? { ...filters, cursor } : filters;
}

export function flattenSessionPages(
  data: InfiniteData<SessionsResponse, unknown> | undefined
): SessionPayload[] | undefined {
  return data?.pages.flatMap(page => page.sessions);
}

export function sessionListTotal(
  data: InfiniteData<SessionsResponse, unknown> | undefined
): number {
  return data?.pages[0]?.page.total ?? 0;
}
