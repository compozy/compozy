import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";
import { fetchSessionCatalogPage, fetchSessionFacets } from "../adapters/session-catalog-api";
import type { SessionListFilters } from "../types";
import { sessionKeys } from "./query-keys";
import { normalizeSessionListFilters } from "./session-list-query";

export interface SessionCatalogCursor {
  cursor?: string;
  previous: (string | undefined)[];
}

export function sessionCatalogOptions(filters: SessionListFilters = {}) {
  const normalized = normalizeSessionListFilters({ ...filters, skip_total: true });
  return infiniteQueryOptions({
    queryKey: [
      ...sessionKeys.workspaceLists(normalized.workspace_id ?? ""),
      "page",
      normalized,
    ] as const,
    queryFn: ({ pageParam, signal }) =>
      fetchSessionCatalogPage({ ...normalized, cursor: pageParam.cursor }, signal),
    initialPageParam: { previous: [] } as SessionCatalogCursor,
    getNextPageParam: (page, _pages, current) =>
      page.page.has_more && page.page.next_cursor
        ? { cursor: page.page.next_cursor, previous: [...current.previous, current.cursor] }
        : undefined,
    getPreviousPageParam: (_page, _pages, current) =>
      current.previous.length > 0
        ? { cursor: current.previous.at(-1), previous: current.previous.slice(0, -1) }
        : undefined,
    maxPages: 1,
    staleTime: 5_000,
    refetchOnWindowFocus: false,
    retry: false,
  });
}

export function sessionFacetsOptions(filters: SessionListFilters = {}) {
  const {
    q: _search,
    badge: _badge,
    attention: _attention,
    limit: _limit,
    skip_total: _skip,
    sort: _sort,
    include_health: _health,
    search_fields: _fields,
    ...scope
  } = normalizeSessionListFilters(filters);
  return queryOptions({
    queryKey: [...sessionKeys.workspaceLists(scope.workspace_id ?? ""), "facets", scope],
    queryFn: ({ signal }) => fetchSessionFacets(scope, signal),
    staleTime: 5_000,
    refetchOnWindowFocus: false,
    retry: false,
  });
}
