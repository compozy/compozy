import { createStore } from "@xstate/store";
import type { Query, QueryClient, QueryKey } from "@tanstack/react-query";

import { LOOP_AGGREGATE_KEY, loopsKeys } from "./query-keys";

type LoopLiveRefreshScope = "run" | "catalog" | "request" | "goal-turns";

interface LoopLiveRefreshContext {
  closed: boolean;
  pending: LoopLiveRefreshScope[];
  phase: "idle" | "scheduled";
}

type LoopLiveRefreshEvents = {
  disposed: Record<never, never>;
  refreshRequested: { scope: LoopLiveRefreshScope };
  refreshStarted: Record<never, never>;
};

interface LoopQueryRefresh {
  followUpRequired: boolean;
}

function refreshKeys(
  workspaceId: string,
  runId: string,
  scopes: LoopLiveRefreshScope[]
): QueryKey[] {
  const keys: QueryKey[] = [];
  const refreshRun = scopes.some(scope => scope !== "goal-turns");
  if (refreshRun) {
    keys.push(
      loopsKeys.runDetail(workspaceId, runId),
      loopsKeys.runsByWorkspace(workspaceId),
      loopsKeys.runsByWorkspace(LOOP_AGGREGATE_KEY),
      // The daemon authors the briefing, roster and story at read time.
      loopsKeys.runReads(workspaceId, runId)
    );
  } else if (scopes.includes("goal-turns")) {
    keys.push(loopsKeys.goalTurns(workspaceId, runId));
  }
  if (scopes.includes("catalog")) {
    keys.push(
      loopsKeys.catalogByWorkspace(workspaceId),
      loopsKeys.catalogByWorkspace(LOOP_AGGREGATE_KEY)
    );
  }
  if (scopes.includes("request")) {
    keys.push(
      loopsKeys.nodeInventoryByWorkspace(workspaceId),
      loopsKeys.requestsByWorkspace(workspaceId),
      loopsKeys.requestsByWorkspace(LOOP_AGGREGATE_KEY)
    );
  }
  return keys;
}

function reportRefreshError(error: unknown): void {
  console.error("Failed to refresh loop live data", error);
}

async function drainLoopQueryRefresh(
  queryClient: QueryClient,
  query: Query,
  refresh: LoopQueryRefresh,
  refreshes: Map<Query, LoopQueryRefresh>
): Promise<void> {
  try {
    const read = query.state.fetchStatus === "fetching" ? query.promise : undefined;
    if (read) await Promise.allSettled([read]);
    while (refresh.followUpRequired) {
      // This fresh read consumes every wake accepted before it starts, including
      // those received while an older initial read or disabled prefetch settled.
      refresh.followUpRequired = false;
      await queryClient
        .invalidateQueries({ predicate: candidate => candidate === query })
        .catch(reportRefreshError);
    }
  } finally {
    refreshes.delete(query);
  }
}

function refreshLoopQuery(
  queryClient: QueryClient,
  queryKey: QueryKey,
  refreshes: Map<Query, LoopQueryRefresh>
): void {
  const busy = new Set<Query>();
  const started = new Map<Query, LoopQueryRefresh>();
  for (const query of queryClient.getQueryCache().findAll({ queryKey })) {
    const current = refreshes.get(query);
    if (current) {
      current.followUpRequired = true;
      busy.add(query);
      continue;
    }
    const refresh = {
      // Active cached reads can restart now. Other pre-event reads must settle
      // before trailing invalidation so their response cannot erase this wake.
      followUpRequired:
        query.state.fetchStatus === "fetching" &&
        (query.state.data === undefined || !query.isActive() || query.isStatic()),
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
    void drainLoopQueryRefresh(queryClient, query, refresh, refreshes).catch(reportRefreshError);
  }
}

/**
 * Frames stay immediate and synchronous wakes share one microtask. Each existing
 * Query owns one read and one pending follow-up, so unrelated reads can progress.
 * Closing the subscription preserves every already accepted reconciliation.
 */
export function createLoopLiveRefreshStore(
  queryClient: QueryClient,
  workspaceId: string,
  runId: string
) {
  const refreshes = new Map<Query, LoopQueryRefresh>();
  return createStore<LoopLiveRefreshContext, LoopLiveRefreshEvents>({
    context: { closed: false, pending: [], phase: "idle" },
    on: {
      refreshRequested: (context, event, enqueue) => {
        if (context.closed) return;
        const pending = context.pending.includes(event.scope)
          ? context.pending
          : [...context.pending, event.scope];
        if (context.phase === "idle") {
          enqueue.effect(({ trigger }) => {
            queueMicrotask(() => trigger.refreshStarted());
          });
        }
        return {
          ...context,
          pending,
          phase: context.phase === "idle" ? "scheduled" : context.phase,
        };
      },
      refreshStarted: (context, _event, enqueue) => {
        if (context.phase !== "scheduled") return;
        const keys = refreshKeys(workspaceId, runId, context.pending);
        enqueue.effect(() => {
          for (const queryKey of keys) refreshLoopQuery(queryClient, queryKey, refreshes);
        });
        return { ...context, pending: [], phase: "idle" };
      },
      disposed: context => ({ ...context, closed: true }),
    },
  });
}
