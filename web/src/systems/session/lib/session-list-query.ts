import type { InfiniteData } from "@tanstack/react-query";

import type { SessionListFilters, SessionPayload, SessionsQuery, SessionsResponse } from "../types";

function normalizedText(value: string | undefined): string | undefined {
  const normalized = value?.trim();
  return normalized ? normalized : undefined;
}

export function normalizeSessionListFilters(filters: SessionListFilters = {}): SessionListFilters {
  const normalized: SessionListFilters = {};
  const workspace = normalizedText(filters.workspace);
  const agent = normalizedText(filters.agent);
  const search = normalizedText(filters.q);

  if (workspace) normalized.workspace = workspace;
  if (filters.include_health !== undefined) {
    normalized.include_health = filters.include_health;
  }
  if (filters.state !== undefined) normalized.state = filters.state;
  if (filters.type !== undefined) normalized.type = filters.type;
  if (agent) normalized.agent = agent;
  if (search) normalized.q = search;
  if (filters.resumable !== undefined) normalized.resumable = filters.resumable;
  if (filters.sort !== undefined) normalized.sort = filters.sort;
  if (filters.limit !== undefined) normalized.limit = filters.limit;

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
