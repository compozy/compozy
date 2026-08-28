import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import {
  fetchTerminal,
  fetchTerminalInputRequestProjection,
  fetchTerminalJournal,
  fetchTerminalRecording,
  fetchTerminals,
  readTerminal,
} from "../adapters/terminal-api";
import type {
  TerminalJournalFilters,
  TerminalProfileScopeParams,
  TerminalScopeKey,
  TerminalScopeParams,
} from "../types";
import {
  TERMINAL_ALL_PROFILES_KEY,
  terminalJournalFiltersWithDefaults,
  terminalKeys,
} from "./query-keys";

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
  aggregate?: false
): TerminalProfileQueryScope;
export function terminalScope(
  workspaceId: string,
  profile: string,
  aggregate: true
): TerminalQueryScope;
export function terminalScope(
  workspaceId: string,
  profile: string,
  aggregate: boolean
): TerminalQueryScope;
export function terminalScope(
  workspaceId: string,
  profile: string,
  aggregate = false
): TerminalQueryScope {
  return aggregate
    ? {
        key: { workspaceId, profileKey: TERMINAL_ALL_PROFILES_KEY },
        params: { all_profiles: true },
      }
    : { key: { workspaceId, profileKey: profile }, params: { profile } };
}

export interface TerminalQueryScope {
  key: TerminalScopeKey;
  params: TerminalScopeParams;
}

export interface TerminalProfileQueryScope extends TerminalQueryScope {
  params: TerminalProfileScopeParams;
}

export function terminalCatalogQuery(scope: TerminalQueryScope) {
  return queryOptions({
    queryKey: terminalKeys.catalog(scope.key),
    queryFn: ({ signal }) => fetchTerminals(scope.key.workspaceId, scope.params, signal),
  });
}

export function terminalDetailQuery(scope: TerminalProfileQueryScope, terminalId: string) {
  return queryOptions({
    queryKey: terminalKeys.detail(scope.key, terminalId),
    queryFn: ({ signal }) => fetchTerminal(scope.key.workspaceId, terminalId, scope.params, signal),
  });
}

/** Captured output for a pipe terminal, which has no interactive screen. */
export function terminalPipeOutputQuery(scope: TerminalProfileQueryScope, terminalId: string) {
  return queryOptions({
    queryKey: terminalKeys.read(scope.key, terminalId, "tail"),
    queryFn: ({ signal }) =>
      readTerminal(
        scope.key.workspaceId,
        terminalId,
        { view: "tail", maxBytes: 1 << 20 },
        scope.params,
        signal
      ),
  });
}

/** Pending and resolved pins for one scope. Callers pick the half they render. */
export function terminalInputRequestsQuery(scope: TerminalQueryScope) {
  return queryOptions({
    queryKey: terminalKeys.inputRequests(scope.key),
    queryFn: ({ signal }) =>
      fetchTerminalInputRequestProjection(scope.key.workspaceId, scope.params, undefined, signal),
  });
}

export function terminalJournalQuery(scope: TerminalQueryScope, filters: TerminalJournalFilters) {
  const withLimit = terminalJournalFiltersWithDefaults(filters);
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

export function terminalRecordingQuery(scope: TerminalProfileQueryScope, recordingId: string) {
  return queryOptions({
    queryKey: terminalKeys.recording(scope.key, recordingId),
    queryFn: ({ signal }) =>
      fetchTerminalRecording(scope.key.workspaceId, recordingId, scope.params, signal),
  });
}
