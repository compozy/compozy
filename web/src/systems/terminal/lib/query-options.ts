import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import {
  fetchTerminal,
  fetchTerminalInputRequests,
  fetchTerminalJournal,
  fetchTerminalRecording,
  fetchTerminals,
} from "../adapters/terminal-api";
import type { TerminalJournalFilters, TerminalScopeKey, TerminalScopeParams } from "../types";
import { TERMINAL_ALL_PROFILES_KEY, terminalKeys } from "./query-keys";

/**
 * Query definitions for every terminal read.
 *
 * Loaders, hooks, mutations and the live stream all reuse these factories, so
 * scope lives in exactly one place and no surface can quietly read unscoped.
 */

/** Turns a read scope into both halves the cache and the query need. */
export function terminalScope(
  workspaceId: string,
  profile: string,
  aggregate = false
): { key: TerminalScopeKey; params: TerminalScopeParams } {
  return aggregate
    ? {
        key: { workspaceId, profileKey: TERMINAL_ALL_PROFILES_KEY },
        params: { all_profiles: true },
      }
    : { key: { workspaceId, profileKey: profile }, params: { profile } };
}

export function terminalCatalogQuery(scope: {
  key: TerminalScopeKey;
  params: TerminalScopeParams;
}) {
  return queryOptions({
    queryKey: terminalKeys.catalog(scope.key),
    queryFn: ({ signal }) => fetchTerminals(scope.key.workspaceId, scope.params, signal),
  });
}

export function terminalDetailQuery(
  scope: { key: TerminalScopeKey; params: TerminalScopeParams },
  terminalId: string
) {
  return queryOptions({
    queryKey: terminalKeys.detail(scope.key, terminalId),
    queryFn: ({ signal }) => fetchTerminal(scope.key.workspaceId, terminalId, scope.params, signal),
  });
}

export function terminalInputRequestsQuery(scope: {
  key: TerminalScopeKey;
  params: TerminalScopeParams;
}) {
  return queryOptions({
    queryKey: terminalKeys.inputRequests(scope.key),
    queryFn: ({ signal }) =>
      fetchTerminalInputRequests(scope.key.workspaceId, scope.params, undefined, signal),
  });
}

/** The server's page size. Paging is cursor-driven; there is no total. */
export const TERMINAL_JOURNAL_PAGE_SIZE = 50;

export function terminalJournalQuery(
  scope: { key: TerminalScopeKey; params: TerminalScopeParams },
  filters: TerminalJournalFilters
) {
  const withLimit = { ...filters, limit: filters.limit ?? TERMINAL_JOURNAL_PAGE_SIZE };
  return infiniteQueryOptions({
    queryKey: terminalKeys.journal(scope.key, withLimit),
    queryFn: ({ pageParam, signal }) =>
      fetchTerminalJournal(scope.key.workspaceId, withLimit, scope.params, pageParam, signal),
    initialPageParam: null as string | null,
    // Continuation lives in `pageParam` alone; the panel never re-derives a
    // cursor from the rows it happens to hold.
    getNextPageParam: page => page.next,
  });
}

export function terminalRecordingQuery(
  scope: { key: TerminalScopeKey; params: TerminalScopeParams },
  recordingId: string
) {
  return queryOptions({
    queryKey: terminalKeys.recording(scope.key, recordingId),
    queryFn: ({ signal }) =>
      fetchTerminalRecording(scope.key.workspaceId, recordingId, scope.params, signal),
  });
}
