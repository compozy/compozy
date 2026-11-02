import { timeoutManager, type Query, type QueryClient, type QueryKey } from "@tanstack/react-query";

interface QueryRefresh {
  followUpRequired: boolean;
  pendingTimeout: ReturnType<typeof timeoutManager.setTimeout> | undefined;
}

function reportRefreshError(error: unknown): void {
  console.error("Failed to refresh live query data", error);
}

function canRestartRead(query: Query): boolean {
  return (
    query.state.fetchStatus === "fetching" &&
    query.state.data !== undefined &&
    query.isActive() &&
    !query.isStatic()
  );
}

function clearPendingTimeout(refresh: QueryRefresh): void {
  if (refresh.pendingTimeout === undefined) return;
  timeoutManager.clearTimeout(refresh.pendingTimeout);
  refresh.pendingTimeout = undefined;
}

function schedulePendingRecovery(
  query: Query,
  refresh: QueryRefresh,
  maxPendingWaitMs: number
): void {
  const ownedRead = query.promise;
  if (refresh.pendingTimeout !== undefined || !ownedRead || !canRestartRead(query)) return;
  refresh.pendingTimeout = timeoutManager.setTimeout(() => {
    refresh.pendingTimeout = undefined;
    if (refresh.followUpRequired && query.promise === ownedRead && canRestartRead(query)) {
      // Cancellation releases the existing drain, which owns the one fresh read.
      void query.cancel({ revert: true }).catch(reportRefreshError);
    }
  }, maxPendingWaitMs);
}

async function drainQueryRefresh(
  queryClient: QueryClient,
  query: Query,
  refresh: QueryRefresh,
  refreshes: Map<Query, QueryRefresh>
): Promise<void> {
  try {
    const read = query.state.fetchStatus === "fetching" ? query.promise : undefined;
    if (read) await Promise.allSettled([read]);
    clearPendingTimeout(refresh);
    while (refresh.followUpRequired) {
      // Consume every wake accepted before this fresh read, including those that
      // arrived while an initial read or disabled prefetch was still settling.
      refresh.followUpRequired = false;
      await queryClient
        .invalidateQueries({ predicate: candidate => candidate === query })
        .catch(reportRefreshError);
      clearPendingTimeout(refresh);
    }
  } finally {
    clearPendingTimeout(refresh);
    refreshes.delete(query);
  }
}

/**
 * Each existing Query owns one read and one pending wake. The caller batches
 * scopes and closes admission; accepted work still drains afterward. A pending
 * cached active read may restart after the caller's existing freshness budget.
 */
export function createQueryLiveRefreshCoordinator(
  queryClient: QueryClient,
  maxPendingWaitMs: number
) {
  const refreshes = new Map<Query, QueryRefresh>();
  return {
    refresh(queryKey: QueryKey): void {
      const busy = new Set<Query>();
      const started = new Map<Query, QueryRefresh>();
      for (const query of queryClient.getQueryCache().findAll({ queryKey })) {
        const current = refreshes.get(query);
        if (current) {
          current.followUpRequired = true;
          schedulePendingRecovery(query, current, maxPendingWaitMs);
          busy.add(query);
          continue;
        }
        const refresh: QueryRefresh = {
          // Active cached reads restart now. Other pre-event reads must settle
          // before trailing invalidation so their response cannot erase this wake.
          followUpRequired: query.state.fetchStatus === "fetching" && !canRestartRead(query),
          pendingTimeout: undefined,
        };
        refreshes.set(query, refresh);
        started.set(query, refresh);
      }
      void queryClient
        .invalidateQueries({
          queryKey,
          ...(busy.size > 0 ? { predicate: (query: Query) => !busy.has(query) } : {}),
        })
        .catch(reportRefreshError);
      for (const [query, refresh] of started) {
        void drainQueryRefresh(queryClient, query, refresh, refreshes).catch(reportRefreshError);
      }
    },
  };
}
