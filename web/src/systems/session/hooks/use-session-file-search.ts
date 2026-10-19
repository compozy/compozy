import { useQuery } from "@tanstack/react-query";

import { useDebouncedValue } from "@/hooks/use-debounced-input";

import type { SessionFileEntry } from "../adapters/session-files-api";
import { sessionFileSearchOptions } from "../lib/session-file-search-options";

/** Picker debounce between keystrokes and the search request (Business Rule 13). */
export const SESSION_MENTION_DEBOUNCE_MS = 120;

const NO_ENTRIES: readonly SessionFileEntry[] = [];

/** One picker group's read state, flattened for the menu view model. */
export type SessionMentionSourceStatus = "loading" | "error" | "ready";

export interface SessionFileSearchView {
  entries: readonly SessionFileEntry[];
  truncated: boolean;
  status: SessionMentionSourceStatus;
}

/**
 * Debounced file/folder search for the `@` picker over the session's working
 * directory. A changed query aborts nothing it does not own: each query is its
 * own cache entry, so a late answer for an older query cannot replace the
 * current rows, and the previous rows stay until the new ones land.
 */
export function useSessionFileSearch({
  workspaceId,
  sessionId,
  query,
  enabled,
}: {
  workspaceId: string;
  sessionId: string;
  query: string;
  enabled: boolean;
}): SessionFileSearchView {
  const trimmed = query.trim();
  const debounced = useDebouncedValue(trimmed, SESSION_MENTION_DEBOUNCE_MS);
  const search = useQuery(sessionFileSearchOptions(workspaceId, sessionId, debounced, enabled));
  const entries = search.data?.entries ?? NO_ENTRIES;
  const status: SessionMentionSourceStatus = search.isError
    ? "error"
    : search.data === undefined
      ? "loading"
      : "ready";
  return {
    entries,
    truncated: search.data?.truncated ?? false,
    status,
  };
}
