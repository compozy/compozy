import type { TerminalJournalFilters, TerminalScopeKey } from "../types";

/**
 * Cache identity for every terminal read.
 *
 * Every key carries `(workspace, profile)`. That is not decoration: terminals
 * are segmented by the profile that opened them, so a key missing either half
 * would let one profile's answer be served to another after a switch.
 */
export const terminalKeys = {
  all: () => ["terminal"] as const,
  catalog: (scope: TerminalScopeKey) =>
    ["terminal", "catalog", scope.workspaceId, scope.profileKey] as const,
  detail: (scope: TerminalScopeKey, terminalId: string) =>
    ["terminal", "detail", scope.workspaceId, scope.profileKey, terminalId] as const,
  read: (scope: TerminalScopeKey, terminalId: string, view: string) =>
    ["terminal", "read", scope.workspaceId, scope.profileKey, terminalId, view] as const,
  inputRequests: (scope: TerminalScopeKey) =>
    ["terminal", "input-requests", scope.workspaceId, scope.profileKey] as const,
  journalScope: (scope: TerminalScopeKey) =>
    ["terminal", "journal", scope.workspaceId, scope.profileKey] as const,
  journal: (scope: TerminalScopeKey, filters: TerminalJournalFilters) =>
    [
      "terminal",
      "journal",
      scope.workspaceId,
      scope.profileKey,
      journalFilterKey(terminalJournalFiltersWithDefaults(filters)),
    ] as const,
  recording: (scope: TerminalScopeKey, recordingId: string) =>
    ["terminal", "recording", scope.workspaceId, scope.profileKey, recordingId] as const,
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
