import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import {
  fetchTerminalInputRequestProjection,
  fetchTerminalJournal,
  fetchTerminalRecording,
  readTerminal,
} from "../adapters/terminal-api";
import type { TerminalJournalFilters } from "../types";
import type { TerminalProfileQueryScope, TerminalQueryScope } from "./catalog-query";
import { terminalJournalFiltersWithDefaults, terminalKeys } from "./query-keys";

export {
  terminalCatalogQuery,
  terminalScope,
  type TerminalProfileQueryScope,
  type TerminalQueryScope,
} from "./catalog-query";

/**
 * Query definitions for every terminal read beyond the catalog list.
 *
 * Loaders, hooks, mutations and the live stream reuse these factories, so
 * scope lives in exactly one place and no surface can quietly read unscoped.
 */

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
