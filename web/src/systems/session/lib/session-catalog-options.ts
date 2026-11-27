import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";
import { isRemoteComputerId, LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";
import { fetchSessionCatalogPage, fetchSessionFacets } from "../adapters/session-catalog-api";
import { fetchSessionFolders, type SessionLayoutScope } from "../adapters/session-folders-api";
import { fetchWorkspaceLayout } from "../adapters/workspace-layout-api";
import type { SessionCatalogPageResponse, SessionListFilters, SessionPayload } from "../types";
import { sessionKeys } from "./query-keys";
import { normalizeSessionListFilters } from "./session-list-query";
import { withSessionsComputer } from "./session-row-ref";

type OwnedCatalogPage = Omit<SessionCatalogPageResponse, "sessions"> & {
  sessions: SessionPayload[];
};

export interface SessionCatalogCursor {
  cursor?: string;
  previous: (string | undefined)[];
}

/** Live refresh interval for remote facets and visible remote pages (Q-C). */
export const REMOTE_SESSION_REFETCH_MS = 15_000;

export function sessionCatalogOptions(
  filters: SessionListFilters = {},
  computerId: ComputerId = LOCAL_COMPUTER_ID
) {
  const normalized = normalizeSessionListFilters({
    ...filters,
    skip_total: filters.skip_total ?? true,
  });
  const remote = isRemoteComputerId(computerId);
  const keys = sessionKeys.forComputer(computerId);
  return infiniteQueryOptions({
    queryKey: keys.catalogPage(normalized),
    queryFn: async ({ pageParam, signal }): Promise<OwnedCatalogPage> => {
      const page = remote
        ? await fetchSessionCatalogPage(
            { ...normalized, cursor: pageParam.cursor },
            signal,
            computerId
          )
        : await fetchSessionCatalogPage({ ...normalized, cursor: pageParam.cursor }, signal);
      return remote ? { ...page, sessions: withSessionsComputer(page.sessions, computerId) } : page;
    },
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
    ...(remote ? { refetchInterval: REMOTE_SESSION_REFETCH_MS } : {}),
    refetchOnWindowFocus: false,
    retry: false,
  });
}

/** Single-page session catalog query used by multi-computer fan-out (`useQueries`). */
export function sessionCatalogFirstPageOptions(
  filters: SessionListFilters = {},
  computerId: ComputerId = LOCAL_COMPUTER_ID
) {
  return sessionCatalogPageQueryOptions(filters, undefined, computerId);
}

/**
 * One cursor-addressed session catalog page query for multi-computer pagination (`useQueries`).
 * Keyed under `keys.catalogPage(normalized)` so workspace invalidations refresh every loaded page,
 * and polls remote daemons every 15s (`Q-C`).
 */
export function sessionCatalogPageQueryOptions(
  filters: SessionListFilters = {},
  cursor?: string,
  computerId: ComputerId = LOCAL_COMPUTER_ID
) {
  const normalized = normalizeSessionListFilters({
    ...filters,
    skip_total: filters.skip_total ?? true,
  });
  const remote = isRemoteComputerId(computerId);
  const keys = sessionKeys.forComputer(computerId);
  const pageTag = cursor ? `cursor:${cursor}` : "first-page";
  return queryOptions({
    queryKey: [...keys.catalogPage(normalized), pageTag] as const,
    queryFn: async ({ signal }): Promise<OwnedCatalogPage> => {
      const request = cursor ? { ...normalized, cursor } : normalized;
      const page = remote
        ? await fetchSessionCatalogPage(request, signal, computerId)
        : await fetchSessionCatalogPage(request, signal);
      return remote ? { ...page, sessions: withSessionsComputer(page.sessions, computerId) } : page;
    },
    staleTime: 5_000,
    ...(remote ? { refetchInterval: REMOTE_SESSION_REFETCH_MS } : {}),
    refetchOnWindowFocus: false,
    retry: false,
  });
}

export function sessionLaunchPreviewOptions(filters: SessionListFilters) {
  const normalized = normalizeSessionListFilters({ ...filters, skip_total: true });
  return queryOptions({
    queryKey: sessionKeys.launchPreview(normalized),
    queryFn: ({ signal }) => fetchSessionCatalogPage(normalized, signal),
    staleTime: 5_000,
    refetchOnWindowFocus: false,
    retry: false,
  });
}

export function sessionFacetsOptions(
  filters: SessionListFilters = {},
  computerId: ComputerId = LOCAL_COMPUTER_ID
) {
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
    folder: _folder,
    ...scope
  } = normalizeSessionListFilters(filters);
  const remote = isRemoteComputerId(computerId);
  const keys = sessionKeys.forComputer(computerId);
  return queryOptions({
    queryKey: [...keys.workspaceLists(scope.workspace_id ?? ""), "facets", scope],
    queryFn: ({ signal }) =>
      remote ? fetchSessionFacets(scope, signal, computerId) : fetchSessionFacets(scope, signal),
    staleTime: 5_000,
    ...(remote ? { refetchInterval: REMOTE_SESSION_REFETCH_MS } : {}),
    refetchOnWindowFocus: false,
    retry: false,
  });
}

/** Cards per board column page (`_spec.md` §Assumptions: page size 50). */
export const SESSION_BOARD_PAGE_SIZE = 50;
export const SESSION_BOARD_STALE_MS = 5_000;

/** Visible board surfaces re-read on this interval; TanStack pauses it while the document is hidden. */
export const SESSION_BOARD_REFETCH_MS = 60_000;

/**
 * One board column (or the Archive bar): the catalog filtered by the daemon's
 * `board_column` before pagination. "Load more" accumulates pages, so unlike the
 * list there is no `maxPages`. The key sits under the workspace list family, so
 * the catalog stream's wake re-reads it like every other catalog read.
 */
export function sessionBoardPageOptions(
  filters: SessionListFilters,
  computerId: ComputerId = LOCAL_COMPUTER_ID
) {
  const normalized = normalizeSessionListFilters({ ...filters, limit: SESSION_BOARD_PAGE_SIZE });
  const remote = isRemoteComputerId(computerId);
  const keys = sessionKeys.forComputer(computerId);
  return infiniteQueryOptions({
    queryKey: [...keys.workspaceLists(normalized.workspace_id ?? ""), "board", normalized] as const,
    queryFn: async ({ pageParam, signal }): Promise<OwnedCatalogPage> => {
      const page = remote
        ? await fetchSessionCatalogPage({ ...normalized, cursor: pageParam }, signal, computerId)
        : await fetchSessionCatalogPage({ ...normalized, cursor: pageParam }, signal);
      return remote ? { ...page, sessions: withSessionsComputer(page.sessions, computerId) } : page;
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: page =>
      page.page.has_more && page.page.next_cursor ? page.page.next_cursor : undefined,
    staleTime: SESSION_BOARD_STALE_MS,
    refetchInterval: remote ? REMOTE_SESSION_REFETCH_MS : SESSION_BOARD_REFETCH_MS,
    refetchOnWindowFocus: false,
    retry: false,
  });
}

/** A board column or Archive bar read (`sessionBoardPageOptions`), in any scope or computer. */
export function isSessionBoardQueryKey(queryKey: readonly unknown[]): boolean {
  return (
    queryKey.length >= 6 &&
    queryKey[0] === "computer" &&
    typeof queryKey[1] === "string" &&
    queryKey[2] === "sessions" &&
    queryKey[3] === "list" &&
    queryKey[5] === "board"
  );
}

/** One profile's folders in one workspace, in the operator's manual order. */
export function sessionFoldersOptions(
  scope: SessionLayoutScope,
  computerId: ComputerId = LOCAL_COMPUTER_ID
) {
  const remote = isRemoteComputerId(computerId);
  const keys = sessionKeys.forComputer(computerId);
  return queryOptions({
    queryKey: keys.folders(scope.workspaceId, scope.profile ?? ""),
    queryFn: ({ signal }) =>
      remote ? fetchSessionFolders(scope, signal, computerId) : fetchSessionFolders(scope, signal),
    staleTime: 5_000,
    refetchOnWindowFocus: false,
    retry: false,
  });
}

/** One profile's project folders and placements; empty `profile` resolves the daemon default. */
export function workspaceLayoutOptions(
  profile: string,
  computerId: ComputerId = LOCAL_COMPUTER_ID
) {
  const remote = isRemoteComputerId(computerId);
  const keys = sessionKeys.forComputer(computerId);
  return queryOptions({
    queryKey: keys.workspaceLayout(profile),
    queryFn: ({ signal }) =>
      remote
        ? fetchWorkspaceLayout(profile.trim() || undefined, signal, computerId)
        : fetchWorkspaceLayout(profile.trim() || undefined, signal),
    staleTime: 5_000,
    refetchOnWindowFocus: false,
    retry: false,
  });
}
