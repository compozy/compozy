import { readCachedOfflineRowsForComputer } from "../lib/session-offline-catalog";
import { QueryClientContext, useQueries, type QueryClient } from "@tanstack/react-query";
import { useContext, useState } from "react";

import { isRemoteComputerId, type ComputerId } from "@/lib/computer-scope";
import type { ProfileScopeParams } from "@/systems/profiles";
import type { DataScope } from "@/systems/workspace";

import {
  SESSION_BOARD_COLUMNS,
  type SessionBoardColumnId,
  type SessionBoardPayload,
} from "../lib/session-board-contract";
import {
  SESSION_BOARD_PAGE_SIZE,
  SESSION_BOARD_REFETCH_MS,
  sessionCatalogPageQueryOptions,
} from "../lib/session-catalog-options";
import type { SessionRowComputerInfo } from "../lib/session-list-computer";
import type { SessionListSort } from "../lib/session-list-preferences";
import {
  mergeTogetherFlatRows,
  selectNextTogetherComputer,
  type TogetherFlatPageEntry,
} from "../lib/session-list-together";
import { sessionRowKey } from "../lib/session-row-ref";
import type { SessionListFilters } from "../types";
import { type ComputerSessionsSource } from "./use-computer-sessions-sources";
import type { SessionsBoardColumnView } from "./use-sessions-board";
import {
  appendMultiCursor,
  boardFiltersForSource,
  resolveMultiBoardFailure,
  summarizeMultiQuerySlots,
  type MultiPageSlot,
  type MultiQuerySlotResult,
} from "./use-sessions-board-multi-pages";

interface ColumnSlot extends MultiPageSlot {
  columnId: SessionBoardColumnId;
}

function recordColumnProvenance(
  entries: readonly TogetherFlatPageEntry[],
  computerBySessionId: Map<string, SessionRowComputerInfo>
): void {
  for (const entry of entries) {
    if (!isRemoteComputerId(entry.source.key)) continue;
    const info: SessionRowComputerInfo = {
      id: entry.source.key,
      name: entry.source.name,
      platform: entry.source.computer.platform,
      iconKind: entry.source.computer.iconKind,
      offline: entry.source.read === "offline",
    };
    for (const session of entry.sessions) {
      computerBySessionId.set(
        sessionRowKey({ computerId: entry.source.key, sessionId: session.id }),
        info
      );
    }
  }
}

function buildTogetherColumnView(input: {
  column: { id: SessionBoardColumnId; label: string };
  answered: readonly ComputerSessionsSource[];
  slots: readonly ColumnSlot[];
  queries: readonly MultiQuerySlotResult[];
  queryClient: QueryClient | undefined;
  sort: SessionListSort;
  onLoadMore: (columnId: SessionBoardColumnId, computerId: ComputerId, cursor: string) => void;
  computerBySessionId: Map<string, SessionRowComputerInfo>;
}): SessionsBoardColumnView {
  const { column, answered, slots, queries, queryClient, sort, onLoadMore, computerBySessionId } =
    input;
  const colSlots: ColumnSlot[] = [];
  const colQueries: MultiQuerySlotResult[] = [];
  for (let index = 0; index < slots.length; index++) {
    if (slots[index]!.columnId !== column.id) continue;
    if (slots[index]!.emptyRemoteMatch) continue;
    colSlots.push(slots[index]!);
    if (queries[index]) colQueries.push(queries[index]!);
  }
  const summary = summarizeMultiQuerySlots(colSlots, colQueries);

  let count = 0;
  let hasCount = false;
  const entries: TogetherFlatPageEntry[] = [];
  for (const source of answered) {
    if (!colSlots.some(slot => slot.source.key === source.key)) continue;
    const facetCount = source.facets?.facets.board?.[column.id];
    if (facetCount !== undefined) {
      hasCount = true;
      count += facetCount;
    }
    if (source.read === "offline") {
      const slot = colSlots.find(slot => slot.source.key === source.key);
      const cached =
        slot && !slot.emptyRemoteMatch
          ? readCachedOfflineRowsForComputer(queryClient, source.key, slot.filters)
          : [];
      entries.push({ source, sessions: cached, hasMore: false, nextCursor: null });
      continue;
    }
    const bucket = summary.sessionsBySource.get(source.key) ?? [];
    const last = summary.lastPageBySource.get(source.key);
    entries.push({
      source,
      sessions: bucket,
      hasMore: Boolean(last?.page.has_more && last.page.next_cursor),
      nextCursor: last?.page.next_cursor ?? null,
    });
  }

  recordColumnProvenance(entries, computerBySessionId);
  const sessions = mergeTogetherFlatRows(entries, sort).map(
    row => row.session as SessionBoardPayload
  );
  const next = selectNextTogetherComputer(entries, sort);
  const failure = resolveMultiBoardFailure(summary, sessions.length);

  return {
    id: column.id,
    label: column.label,
    sessions,
    loading: summary.initialLoading && sessions.length === 0,
    failure,
    retrying: failure !== null && summary.fetching,
    fetching: summary.fetching,
    hasMore: next !== null && next.nextCursor !== null,
    loadingMore: summary.loadingMore,
    loadMore: () => {
      if (!next?.nextCursor || summary.loadingMore) return;
      onLoadMore(column.id, next.computerId, next.nextCursor);
    },
    retry: () => {
      const online = colQueries.filter((_, index) => colSlots[index]?.source.read === "ok");
      const failed = online.filter(query => query.isError);
      const targets = failed.length > 0 ? failed : online;
      for (const query of targets) void query.refetch();
    },
    total: hasCount ? count : undefined,
    count: hasCount ? count : undefined,
  };
}

/**
 * Together + None Kanban columns (`Opus 4`, `Astra 9`): merges per-computer column pages
 * ordered by window sort, sums `facets.board`, and tracks qualified computer provenance.
 */
export function useTogetherBoardColumns(options: {
  sources: readonly ComputerSessionsSource[];
  scope: DataScope | null;
  shared: SessionListFilters;
  localLens: ProfileScopeParams;
  sort: SessionListSort;
  enabled: boolean;
}): {
  columns: SessionsBoardColumnView[] | null;
  computerBySessionId: Map<string, SessionRowComputerInfo>;
} {
  const { sources, scope, shared, localLens, sort, enabled } = options;
  const queryClient = useContext(QueryClientContext);
  const answered =
    enabled && scope !== null
      ? sources.filter(source => source.read === "ok" || source.read === "offline")
      : [];
  const resetKey = JSON.stringify([scope, shared, localLens]);
  const [cursorState, setCursorState] = useState<{
    resetKey: string;
    cursors: Record<string, string[]>;
  }>({ resetKey: "", cursors: {} });
  const activeCursors = cursorState.resetKey === resetKey ? cursorState.cursors : {};

  const slots: ColumnSlot[] = [];
  for (const column of SESSION_BOARD_COLUMNS) {
    for (let index = 0; index < answered.length; index++) {
      const source = answered[index]!;
      const spec = boardFiltersForSource(source, scope, shared, localLens);
      const filters: SessionListFilters = {
        ...spec.filters,
        board_column: [column.id],
        limit: SESSION_BOARD_PAGE_SIZE,
        skip_total: true,
      };
      slots.push({
        columnId: column.id,
        source,
        sourceIndex: index,
        filters,
        emptyRemoteMatch: spec.emptyRemoteMatch,
        cursor: undefined,
      });
      const bucketKey = `${column.id}:${source.key}`;
      for (const cursor of activeCursors[bucketKey] ?? []) {
        slots.push({
          columnId: column.id,
          source,
          sourceIndex: index,
          filters,
          emptyRemoteMatch: spec.emptyRemoteMatch,
          cursor,
        });
      }
    }
  }

  const queries = useQueries({
    queries: slots.map(slot => ({
      ...sessionCatalogPageQueryOptions(slot.filters, slot.cursor, slot.source.key),
      ...(slot.source.isLocal ? { refetchInterval: SESSION_BOARD_REFETCH_MS } : {}),
      enabled: enabled && slot.source.read === "ok" && !slot.emptyRemoteMatch && scope !== null,
    })),
  });

  const computerBySessionId = new Map<string, SessionRowComputerInfo>();
  if (!enabled) return { columns: null, computerBySessionId };

  const onLoadMore = (columnId: SessionBoardColumnId, computerId: ComputerId, cursor: string) => {
    setCursorState(prev => appendMultiCursor(prev, resetKey, `${columnId}:${computerId}`, cursor));
  };
  const columns = SESSION_BOARD_COLUMNS.map(column =>
    buildTogetherColumnView({
      column,
      answered,
      slots,
      queries,
      queryClient,
      sort,
      onLoadMore,
      computerBySessionId,
    })
  );
  return { columns, computerBySessionId };
}
