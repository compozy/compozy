import { createStore } from "@xstate/store";
import type { QueryClient, QueryKey } from "@tanstack/react-query";

import { LOOP_AGGREGATE_KEY, loopsKeys } from "./query-keys";

type LoopLiveRefreshScope = "run" | "catalog" | "request" | "goal-turns";

interface LoopLiveRefreshContext {
  closed: boolean;
  pending: LoopLiveRefreshScope[];
  phase: "idle" | "scheduled" | "refreshing";
}

type LoopLiveRefreshEvents = {
  disposed: Record<never, never>;
  refreshRequested: { scope: LoopLiveRefreshScope };
  refreshSettled: Record<never, never>;
  refreshStarted: Record<never, never>;
};

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

async function refreshLoopQuery(queryClient: QueryClient, queryKey: QueryKey): Promise<void> {
  // Active cached reads can restart immediately. Other in-flight reads must
  // settle before trailing invalidation so they cannot erase an accepted wake.
  const trailingReads = queryClient
    .getQueryCache()
    .findAll({ queryKey, fetchStatus: "fetching" })
    .flatMap(query => {
      const read = query.promise;
      return read && (query.state.data === undefined || !query.isActive() || query.isStatic())
        ? [{ query, read }]
        : [];
    });
  await Promise.all([
    queryClient.invalidateQueries({ queryKey }),
    ...trailingReads.map(async ({ query, read }) => {
      await Promise.allSettled([read]);
      // Preserve the matched query's identity, including custom key hashing.
      // Default active-only refetch leaves disabled observers stale until enabled.
      await queryClient.invalidateQueries({ predicate: candidate => candidate === query });
    }),
  ]);
}

/**
 * Uses the task stream's microtask/in-flight reconciliation pattern. Frames stay
 * immediate; one burst starts one read round and arrivals during it merge into
 * one follow-up. Closing the subscription preserves that final pending read.
 */
export function createLoopLiveRefreshStore(
  queryClient: QueryClient,
  workspaceId: string,
  runId: string
) {
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
        enqueue.effect(async ({ trigger }) => {
          try {
            await Promise.all(keys.map(queryKey => refreshLoopQuery(queryClient, queryKey)));
          } catch (error) {
            console.error("Failed to refresh loop live data", error);
          } finally {
            trigger.refreshSettled();
          }
        });
        return { ...context, pending: [], phase: "refreshing" };
      },
      refreshSettled: (context, _event, enqueue) => {
        if (context.phase !== "refreshing") return;
        if (context.pending.length === 0) return { ...context, phase: "idle" };
        enqueue.effect(({ trigger }) => {
          queueMicrotask(() => trigger.refreshStarted());
        });
        return { ...context, phase: "scheduled" };
      },
      disposed: context => ({ ...context, closed: true }),
    },
  });
}
