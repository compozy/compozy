import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";
import { fetchSessionCatalogPage, fetchSessionFacets } from "../adapters/session-catalog-api";
import { fetchSessionFolders, type SessionLayoutScope } from "../adapters/session-folders-api";
import { fetchWorkspaceLayout } from "../adapters/workspace-layout-api";
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
    board_column: _column,
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

/** Cards per board column page (`_spec.md` §Assumptions: page size 50). */
export const SESSION_BOARD_PAGE_SIZE = 50;

/** Visible board surfaces re-read on this interval; TanStack pauses it while the document is hidden. */
export const SESSION_BOARD_REFETCH_MS = 60_000;

/**
 * One board column (or the Archive bar): the catalog filtered by the daemon's
 * `board_column` before pagination. "Load more" accumulates pages, so unlike the
 * list there is no `maxPages`. The key sits under the workspace list family, so
 * the catalog stream's wake re-reads it like every other catalog read.
 */
export function sessionBoardPageOptions(filters: SessionListFilters) {
  const normalized = normalizeSessionListFilters({ ...filters, limit: SESSION_BOARD_PAGE_SIZE });
  return infiniteQueryOptions({
    queryKey: [
      ...sessionKeys.workspaceLists(normalized.workspace_id ?? ""),
      "board",
      normalized,
    ] as const,
    queryFn: ({ pageParam, signal }) =>
      fetchSessionCatalogPage({ ...normalized, cursor: pageParam }, signal),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: page =>
      page.page.has_more && page.page.next_cursor ? page.page.next_cursor : undefined,
    staleTime: 5_000,
    refetchInterval: SESSION_BOARD_REFETCH_MS,
    refetchOnWindowFocus: false,
    retry: false,
  });
}

/** One profile's folders in one workspace, in the operator's manual order. */
export function sessionFoldersOptions(scope: SessionLayoutScope) {
  return queryOptions({
    queryKey: sessionKeys.folders(scope.workspaceId, scope.profile ?? ""),
    queryFn: ({ signal }) => fetchSessionFolders(scope, signal),
    staleTime: 5_000,
    refetchOnWindowFocus: false,
    retry: false,
  });
}

/** One profile's project folders and placements; empty `profile` resolves the daemon default. */
export function workspaceLayoutOptions(profile: string) {
  return queryOptions({
    queryKey: sessionKeys.workspaceLayout(profile),
    queryFn: ({ signal }) => fetchWorkspaceLayout(profile.trim() || undefined, signal),
    staleTime: 5_000,
    refetchOnWindowFocus: false,
    retry: false,
  });
}
