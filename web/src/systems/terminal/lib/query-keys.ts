import { computerQueryKey, LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";
import type { TerminalJournalFilters, TerminalScopeKey } from "../types";

export function terminalKeysFor(computerId: ComputerId = LOCAL_COMPUTER_ID) {
  const root = computerQueryKey(computerId, "terminal");
  return {
    all: () => root,
    catalog: (scope: TerminalScopeKey) =>
      [...root, "catalog", scope.workspaceId, scope.profileKey] as const,
    /** The All projects list, per Project-filter selection and profile lens. */
    aggregate: (selection: string, profileKey: string) =>
      [...root, "aggregate", selection, profileKey] as const,
    detail: (scope: TerminalScopeKey, terminalId: string) =>
      [...root, "detail", scope.workspaceId, scope.profileKey, terminalId] as const,
    read: (scope: TerminalScopeKey, terminalId: string, view: string) =>
      [...root, "read", scope.workspaceId, scope.profileKey, terminalId, view] as const,
    inputRequests: (scope: TerminalScopeKey) =>
      [...root, "input-requests", scope.workspaceId, scope.profileKey] as const,
    journalScope: (scope: TerminalScopeKey) =>
      [...root, "journal", scope.workspaceId, scope.profileKey] as const,
    journal: (scope: TerminalScopeKey, filters: TerminalJournalFilters) =>
      [
        ...root,
        "journal",
        scope.workspaceId,
        scope.profileKey,
        journalFilterKey(terminalJournalFiltersWithDefaults(filters)),
      ] as const,
    recording: (scope: TerminalScopeKey, recordingId: string) =>
      [...root, "recording", scope.workspaceId, scope.profileKey, recordingId] as const,
    /** Live recording presence, distinct from a downloaded recording artifact. */
    recordings: (scope: TerminalScopeKey) =>
      [...root, "recordings", scope.workspaceId, scope.profileKey] as const,
  } as const;
}

/**
 * Cache identity for every terminal read.
 *
 * Every key carries `(computer, workspace, profile)`.
 */
export const terminalKeys = {
  ...terminalKeysFor(LOCAL_COMPUTER_ID),
  forComputer: terminalKeysFor,
} as const;

/** The server's page size. Paging is cursor-driven; there is no total. */
export const TERMINAL_JOURNAL_PAGE_SIZE = 50;

export function terminalJournalFiltersWithDefaults(
  filters: TerminalJournalFilters
): TerminalJournalFilters {
  return { ...filters, limit: filters.limit ?? TERMINAL_JOURNAL_PAGE_SIZE };
}

/**
 * A stable, order-independent identity for a filter set.
 *
 * Filters compose into the server query, so two filter sets that produce the
 * same query must share one cache entry — and two that differ must never.
 */
function journalFilterKey(filters: TerminalJournalFilters): string {
  return JSON.stringify({
    actor: filters.actor ?? null,
    failed: filters.failed ?? false,
    limit: filters.limit ?? null,
    since: filters.since ?? null,
    terminalId: filters.terminalId ?? null,
  });
}

/** The reserved cache segment for the read-only all-profiles view. */
export const TERMINAL_ALL_PROFILES_KEY = "@all";
