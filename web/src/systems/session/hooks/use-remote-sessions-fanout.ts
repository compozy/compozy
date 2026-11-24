import { QueryClient, QueryClientContext, useQueries } from "@tanstack/react-query";
import { useContext, useState } from "react";

import {
  COMPUTER_SESSION_PAGE_LIMIT,
  groupSessionsByProjectAndComputer,
  iconKindForPlatform,
  mergeSessionPages,
  normalizeComputerPlatform,
  selectNextComputerToLoadMore,
  useComputers,
  type ComputerPayload,
  type ComputerSessionPageInput,
  type MergedSessionRow,
  type NextComputerPageSelection,
  type ProjectComputerGroup,
  type ScopedComputerFailureItem,
} from "@/systems/computers";

import { fetchSessionCatalogPage } from "../adapters/session-catalog-api";
import { sessionCatalogFirstPageOptions } from "../lib/session-catalog-options";
import type { SessionRowComputerInfo } from "../lib/session-list-computer";
import type { SessionListFilters, SessionPayload, SessionsResponse } from "../types";

const NOOP_QUERY_CLIENT = new QueryClient({
  defaultOptions: { queries: { enabled: false, retry: false } },
});

interface RemoteExtraPageEntry {
  sessions: SessionPayload[];
  nextCursor: string | null;
  hasMore: boolean;
}

interface RemoteExtraPagesState {
  key: string;
  pagesByComputer: Record<string, RemoteExtraPageEntry>;
}

interface RemoteQuerySlice {
  data?: SessionsResponse;
  isLoading: boolean;
  isError: boolean;
  refetch: () => Promise<unknown>;
}

export interface RemoteSessionsFanoutResult {
  hasRemoteComputers: boolean;
  remoteSessions: MergedSessionRow<SessionPayload>[];
  computerBySessionId: ReadonlyMap<string, SessionRowComputerInfo>;
  loadingComputers: readonly { id: string; name: string }[];
  failedComputers: readonly ScopedComputerFailureItem[];
  hasMore: boolean;
  loadingMore: boolean;
  loadMore: () => Promise<void>;
  projectComputerGroups: ProjectComputerGroup<SessionPayload>[];
}

function projectRemoteFanout(
  remoteComputers: readonly ComputerPayload[],
  queries: readonly RemoteQuerySlice[],
  currentExtra: Record<string, RemoteExtraPageEntry>
): {
  remoteSessions: MergedSessionRow<SessionPayload>[];
  computerBySessionId: ReadonlyMap<string, SessionRowComputerInfo>;
  loadingComputers: { id: string; name: string }[];
  failedComputers: ScopedComputerFailureItem[];
  nextToLoad: NextComputerPageSelection | null;
} {
  const pageInputs: ComputerSessionPageInput<SessionPayload>[] = [];
  const loadingComputers: { id: string; name: string }[] = [];
  const failedComputers: ScopedComputerFailureItem[] = [];

  for (let index = 0; index < remoteComputers.length; index++) {
    const computer = remoteComputers[index];
    const query = queries[index];
    if (!computer || !query) continue;
    const isOffline = computer.status === "offline";
    if (!isOffline && query.isLoading) {
      loadingComputers.push({ id: computer.id, name: computer.name });
    }
    if (!isOffline && query.isError && !query.data) {
      failedComputers.push({
        id: computer.id,
        name: computer.name,
        retry: () => {
          void query.refetch();
        },
      });
    }
    const baseSessions = query.data?.sessions ?? [];
    const extra = currentExtra[computer.id];
    pageInputs.push({
      computer: { id: computer.id, name: computer.name, status: computer.status },
      sessions: extra ? [...baseSessions, ...extra.sessions] : baseSessions,
      nextCursor: extra ? extra.nextCursor : (query.data?.page.next_cursor ?? null),
      hasMore: extra ? extra.hasMore : Boolean(query.data?.page.has_more),
    });
  }

  const remoteSessions = mergeSessionPages(pageInputs);
  const computerBySessionId = new Map<string, SessionRowComputerInfo>();
  const computerMetaById = new Map(remoteComputers.map(computer => [computer.id, computer]));
  for (const row of remoteSessions) {
    const platform = normalizeComputerPlatform(computerMetaById.get(row.computerId)?.platform);
    computerBySessionId.set(row.session.id, {
      id: row.computerId,
      name: row.computerName,
      platform,
      iconKind: iconKindForPlatform(platform),
      offline: row.offline,
    });
  }

  const nextToLoad = selectNextComputerToLoadMore(
    pageInputs.filter(entry => entry.computer.status === "connected")
  );
  return { remoteSessions, computerBySessionId, loadingComputers, failedComputers, nextToLoad };
}

/**
 * Fans out session catalog reads across enabled remote computers (`limit = 50` per computer,
 * UT-087), retains cached rows with `offline: true` when a computer goes offline (UT-086),
 * merges rows by `updated_at` desc (UT-085), and advances only the oldest cursor on "load more".
 */
export function useRemoteSessionsFanout(options: {
  enabled?: boolean;
  filters?: SessionListFilters;
}): RemoteSessionsFanoutResult {
  const contextClient = useContext(QueryClientContext);
  const enabled = (options.enabled ?? true) && contextClient !== undefined;
  const filters: SessionListFilters = {
    ...options.filters,
    limit: options.filters?.limit ?? COMPUTER_SESSION_PAGE_LIMIT,
  };
  const filterKey = JSON.stringify(filters);
  const computers = useComputers({ enabled, passive: true });
  const remoteComputers = computers.enabledComputers;
  const [extraState, setExtraState] = useState<RemoteExtraPagesState>({
    key: filterKey,
    pagesByComputer: {},
  });
  const [loadingMore, setLoadingMore] = useState(false);

  const queries = useQueries(
    {
      queries: remoteComputers.map(computer => ({
        ...sessionCatalogFirstPageOptions(filters, computer.id),
        // Only fetch while connected; offline computers keep their cached TanStack Query data
        // so rows remain visible with `offline: true` (UT-086).
        enabled: enabled && computer.status === "connected",
      })),
    },
    contextClient ?? NOOP_QUERY_CLIENT
  );

  const currentExtra = extraState.key === filterKey ? extraState.pagesByComputer : {};
  const projection = projectRemoteFanout(remoteComputers, queries, currentExtra);

  const loadMore = async () => {
    const target = projection.nextToLoad;
    if (!target || !target.nextCursor || loadingMore) return;
    setLoadingMore(true);
    try {
      const page = await fetchSessionCatalogPage(
        { ...filters, cursor: target.nextCursor, limit: COMPUTER_SESSION_PAGE_LIMIT },
        undefined,
        target.computerId
      );
      setExtraState(prev => {
        const baseMap = prev.key === filterKey ? prev.pagesByComputer : {};
        const existing = baseMap[target.computerId];
        return {
          key: filterKey,
          pagesByComputer: {
            ...baseMap,
            [target.computerId]: {
              sessions: [...(existing?.sessions ?? []), ...page.sessions],
              nextCursor: page.page.next_cursor ?? null,
              hasMore: page.page.has_more,
            },
          },
        };
      });
    } finally {
      setLoadingMore(false);
    }
  };

  return {
    hasRemoteComputers: remoteComputers.length > 0,
    remoteSessions: projection.remoteSessions,
    computerBySessionId: projection.computerBySessionId,
    loadingComputers: projection.loadingComputers,
    failedComputers: projection.failedComputers,
    hasMore: projection.nextToLoad !== null,
    loadingMore,
    loadMore,
    projectComputerGroups: groupSessionsByProjectAndComputer(projection.remoteSessions),
  };
}
