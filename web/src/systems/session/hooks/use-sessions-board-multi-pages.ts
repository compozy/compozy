import { readCachedOfflineRowsForComputer } from "../lib/session-offline-catalog";
import { QueryClientContext, useQueries, type QueryClient } from "@tanstack/react-query";
import { useContext, useState } from "react";

import type { ComputerId } from "@/lib/computer-scope";
import type { ProfileScopeParams } from "@/systems/profiles";
import type { DataScope } from "@/systems/workspace";

import type { SessionBoardPayload } from "../lib/session-board-contract";
import {
  SESSION_BOARD_PAGE_SIZE,
  SESSION_BOARD_REFETCH_MS,
  sessionCatalogPageQueryOptions,
} from "../lib/session-catalog-options";
import { sessionCatalogScopedFilters } from "../lib/session-catalog-scope";
import { sessionListShelfFilters } from "../lib/session-list-shelf";
import {
  mergeTogetherFlatRows,
  selectNextTogetherComputer,
  type TogetherFlatPageEntry,
} from "../lib/session-list-together";
import { sessionRowKey } from "../lib/session-row-ref";
import type { SessionCatalogPageResponse, SessionListFilters } from "../types";
import { type ComputerSessionsSource } from "./use-computer-sessions-sources";
import type { SessionsBoardFailure, SessionsBoardShelf } from "./use-sessions-board";

export interface BoardSourceFilterSpec {
  filters: SessionListFilters;
  emptyRemoteMatch: boolean;
}

export interface MultiPageSlot {
  source: ComputerSessionsSource;
  sourceIndex: number;
  filters: SessionListFilters;
  emptyRemoteMatch: boolean;
  cursor: string | undefined;
}

export interface MultiQuerySlotResult {
  data?: SessionCatalogPageResponse;
  isLoading: boolean;
  isFetching: boolean;
  isError: boolean;
  refetch: () => Promise<unknown>;
}

export interface MultiSlotSummary {
  sessionsBySource: Map<ComputerId, SessionBoardPayload[]>;
  lastPageBySource: Map<ComputerId, SessionCatalogPageResponse>;
  firstPageTotal: number;
  hasFirstPageTotal: boolean;
  initialLoading: boolean;
  loadingMore: boolean;
  fetching: boolean;
  allInitialFailed: boolean;
  anyFirstFailed: boolean;
  anyMoreFailed: boolean;
}

export function boardFiltersForSource(
  source: ComputerSessionsSource,
  scope: DataScope | null,
  shared: SessionListFilters,
  localLens: ProfileScopeParams
): BoardSourceFilterSpec {
  if (source.isLocal) {
    return {
      filters: sessionCatalogScopedFilters(scope, shared, localLens),
      emptyRemoteMatch: false,
    };
  }
  if (scope?.kind === "project") {
    const matched = source.matchedWorkspaceIds ?? [];
    if (matched.length === 0) {
      return {
        filters: { ...shared, all_workspaces: true, all_profiles: true },
        emptyRemoteMatch: true,
      };
    }
    return {
      filters: {
        ...shared,
        all_workspaces: true,
        all_profiles: true,
        workspace_ids: [...matched],
      },
      emptyRemoteMatch: false,
    };
  }
  return {
    filters: sessionCatalogScopedFilters(scope?.kind === "user" ? scope : { kind: "all" }, shared, {
      all_profiles: true,
    }),
    emptyRemoteMatch: false,
  };
}

export function resolveMultiBoardFailure(
  summary: MultiSlotSummary,
  sessionCount: number
): SessionsBoardFailure | null {
  if (summary.allInitialFailed && sessionCount === 0) return "initial";
  if (summary.anyMoreFailed) return "more";
  if (summary.anyFirstFailed && sessionCount > 0) return "refresh";
  return null;
}

export function appendMultiCursor(
  prev: { resetKey: string; cursors: Record<string, string[]> },
  resetKey: string,
  bucketKey: string,
  cursor: string
): { resetKey: string; cursors: Record<string, string[]> } {
  const base = prev.resetKey === resetKey ? prev.cursors : {};
  const existing = base[bucketKey] ?? [];
  if (existing.includes(cursor)) return prev;
  return { resetKey, cursors: { ...base, [bucketKey]: [...existing, cursor] } };
}

export function summarizeMultiQuerySlots(
  slots: readonly MultiPageSlot[],
  queries: readonly MultiQuerySlotResult[]
): MultiSlotSummary {
  const sessionsBySource = new Map<ComputerId, SessionBoardPayload[]>();
  const lastPageBySource = new Map<ComputerId, SessionCatalogPageResponse>();
  const seenKeys = new Set<string>();
  let firstPageTotal = 0;
  let hasFirstPageTotal = false;
  let initialLoading = false;
  let loadingMore = false;
  let fetching = false;
  let anyFirstFailed = false;
  let anyMoreFailed = false;
  let onlineActive = 0;
  let onlineFirstFailed = 0;

  for (let index = 0; index < slots.length; index++) {
    const slot = slots[index]!;
    const query = queries[index];
    const isMore = slot.cursor !== undefined;
    const online = slot.source.read === "ok" && !slot.emptyRemoteMatch;
    if (!online) continue;
    if (!isMore) onlineActive += 1;
    if (query?.isFetching) {
      fetching = true;
      if (isMore) loadingMore = true;
    }
    if (query?.isLoading && !isMore && online) initialLoading = true;
    if (query?.isError) {
      if (isMore) anyMoreFailed = true;
      else {
        anyFirstFailed = true;
        onlineFirstFailed += 1;
      }
    }
    const page = query?.data;
    if (!page) continue;
    lastPageBySource.set(slot.source.key, page);
    if (!isMore && page.page.total != null) {
      hasFirstPageTotal = true;
      firstPageTotal += page.page.total;
    }
    const bucket = sessionsBySource.get(slot.source.key) ?? [];
    for (const session of page.sessions) {
      const key = sessionRowKey({ computerId: slot.source.key, sessionId: session.id });
      if (seenKeys.has(key)) continue;
      seenKeys.add(key);
      bucket.push(session as SessionBoardPayload);
    }
    sessionsBySource.set(slot.source.key, bucket);
  }

  return {
    sessionsBySource,
    lastPageBySource,
    firstPageTotal,
    hasFirstPageTotal,
    initialLoading,
    loadingMore,
    fetching,
    allInitialFailed: onlineActive > 0 && onlineFirstFailed === onlineActive,
    anyFirstFailed,
    anyMoreFailed,
  };
}

function buildShelfSlots(
  answered: readonly ComputerSessionsSource[],
  scope: DataScope | null,
  shared: SessionListFilters,
  shelfFilters: SessionListFilters,
  localLens: ProfileScopeParams,
  activeCursors: Readonly<Record<string, string[]>>
) {
  const slots: MultiPageSlot[] = [];
  for (let index = 0; index < answered.length; index++) {
    const source = answered[index]!;
    const spec = boardFiltersForSource(source, scope, shared, localLens);
    const filters: SessionListFilters = {
      ...spec.filters,
      ...shelfFilters,
      limit: SESSION_BOARD_PAGE_SIZE,
    };
    slots.push({
      source,
      sourceIndex: index,
      filters: { ...filters, skip_total: false },
      emptyRemoteMatch: spec.emptyRemoteMatch,
      cursor: undefined,
    });
    for (const cursor of activeCursors[source.key] ?? []) {
      slots.push({
        source,
        sourceIndex: index,
        filters: { ...filters, skip_total: true },
        emptyRemoteMatch: spec.emptyRemoteMatch,
        cursor,
      });
    }
  }

  return slots;
}

function collectShelfEntries(
  answered: readonly ComputerSessionsSource[],
  slots: readonly MultiPageSlot[],
  summary: MultiSlotSummary,
  queryClient: QueryClient | undefined,
  shelf: "archive" | "later"
) {
  const firstSlotsByComputer = new Map<ComputerId, MultiPageSlot>();
  for (const slot of slots) {
    if (!firstSlotsByComputer.has(slot.source.key)) firstSlotsByComputer.set(slot.source.key, slot);
  }

  const entries: TogetherFlatPageEntry[] = [];
  let total = summary.firstPageTotal;
  let hasTotal = summary.hasFirstPageTotal;

  for (const source of answered) {
    const slot = firstSlotsByComputer.get(source.key);
    if (!slot || slot.emptyRemoteMatch) continue;
    if (source.read === "offline") {
      const cached = readCachedOfflineRowsForComputer(queryClient, source.key, slot.filters);
      const first = queryClient?.getQueryData(
        sessionCatalogPageQueryOptions(slot.filters, undefined, source.key).queryKey
      );
      const cachedTotal = first?.page.total ?? (shelf === "later" ? source.laterCount : undefined);
      if (cachedTotal !== undefined) {
        hasTotal = true;
        total += cachedTotal;
      }
      entries.push({ source, sessions: cached, hasMore: false, nextCursor: null });
      continue;
    }
    const bucket = summary.sessionsBySource.get(source.key) ?? [];
    const last = summary.lastPageBySource.get(source.key);
    if (shelf === "later" && !last) {
      hasTotal = true;
      total += source.laterCount;
    }
    entries.push({
      source,
      sessions: bucket,
      hasMore: Boolean(last?.page.has_more && last.page.next_cursor),
      nextCursor: last?.page.next_cursor ?? null,
    });
  }

  return { entries, total, hasTotal };
}

/**
 * Multi-computer Archive or Later shelf with cursor pagination across answered computers
 * and 15s polling on remote daemons (`Opus 7`, `Opus 12`, `Astra 13`, `Astra 14`).
 */
export function useMultiComputerBoardShelf(
  sources: readonly ComputerSessionsSource[],
  scope: DataScope | null,
  shared: SessionListFilters,
  localLens: ProfileScopeParams,
  shelf: "archive" | "later",
  enabled: boolean
): SessionsBoardShelf | undefined {
  const queryClient = useContext(QueryClientContext);
  const shelfFilters = sessionListShelfFilters(shelf);
  const answered =
    enabled && scope !== null
      ? sources.filter(source => source.read === "ok" || source.read === "offline")
      : [];
  const resetKey = JSON.stringify([scope, shared, localLens, shelf]);
  const [cursorState, setCursorState] = useState<{
    resetKey: string;
    cursors: Record<string, string[]>;
  }>({ resetKey: "", cursors: {} });
  const activeCursors = cursorState.resetKey === resetKey ? cursorState.cursors : {};

  const slots = buildShelfSlots(answered, scope, shared, shelfFilters, localLens, activeCursors);
  const queries = useQueries({
    queries: slots.map(slot => ({
      ...sessionCatalogPageQueryOptions(slot.filters, slot.cursor, slot.source.key),
      ...(slot.source.isLocal ? { refetchInterval: SESSION_BOARD_REFETCH_MS } : {}),
      enabled: enabled && slot.source.read === "ok" && !slot.emptyRemoteMatch && scope !== null,
    })),
  });
  if (!enabled) return undefined;

  const summary = summarizeMultiQuerySlots(slots, queries);
  const { entries, total, hasTotal } = collectShelfEntries(
    answered,
    slots,
    summary,
    queryClient,
    shelf
  );
  const sort =
    shared.sort === "created" || shared.sort === "attention" ? shared.sort : "last_activity";
  const allSessions = mergeTogetherFlatRows(entries, sort).map(
    row => row.session as SessionBoardPayload
  );
  const failure = resolveMultiBoardFailure(summary, allSessions.length);
  const next = selectNextTogetherComputer(entries, sort);
  return {
    sessions: allSessions,
    loading: summary.initialLoading && allSessions.length === 0,
    failure,
    retrying: failure !== null && summary.fetching,
    fetching: summary.fetching,
    hasMore: next !== null && next.nextCursor !== null,
    loadingMore: summary.loadingMore,
    loadMore: () => {
      if (!next?.nextCursor || summary.loadingMore) return;
      setCursorState(prev => appendMultiCursor(prev, resetKey, next.computerId, next.nextCursor!));
    },
    retry: () => {
      const online = queries.filter((_, index) => {
        const slot = slots[index];
        return slot?.source.read === "ok" && !slot.emptyRemoteMatch;
      });
      const failed = online.filter(query => query.isError);
      const targets = failed.length > 0 ? failed : online;
      for (const query of targets) void query.refetch();
    },
    total: hasTotal ? total : undefined,
    count: hasTotal ? total : undefined,
  };
}
