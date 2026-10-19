import { keepPreviousData, queryOptions } from "@tanstack/react-query";

import { searchSessionFiles } from "../adapters/session-files-api";
import { sessionKeys } from "./query-keys";

/** The picker asks for 50 entries (Business Rule 13); the route allows up to 200. */
export const SESSION_FILE_SEARCH_LIMIT = 50;

/** The daemon index is cached for 15 s per root; rereading sooner buys nothing. */
const SESSION_FILE_SEARCH_STALE_TIME_MS = 15_000;

/**
 * One cache entry per (workspace, session, query): a response for an older
 * query lands in its own entry and can never overwrite the current list, while
 * `keepPreviousData` keeps the last rows on screen until the next ones arrive
 * (US-001.EC-6/EC-7). The working directory is resolved by the daemon from the
 * session, so the session id is the scope — not the workspace root.
 */
export function sessionFileSearchOptions(
  workspaceId: string,
  sessionId: string,
  query: string,
  enabled: boolean,
  limit = SESSION_FILE_SEARCH_LIMIT
) {
  const q = query.trim();
  return queryOptions({
    queryKey: sessionKeys.fileSearch(workspaceId, sessionId, q, limit),
    queryFn: ({ signal }) =>
      searchSessionFiles(workspaceId, sessionId, q ? { q, limit } : { limit }, signal),
    enabled: enabled && workspaceId !== "" && sessionId !== "",
    placeholderData: keepPreviousData,
    staleTime: SESSION_FILE_SEARCH_STALE_TIME_MS,
    retry: false,
  });
}
