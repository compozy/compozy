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
  // An initial read can predate the event and still be reused by invalidation.
  // Let it settle first so its response cannot clear this refresh obligation.
  const reads = queryClient
    .getQueryCache()
    .findAll({ queryKey, fetchStatus: "fetching" })
    .flatMap(query => (query.promise ? [query.promise] : []));
  if (reads.length > 0) await Promise.allSettled(reads);
  // Keep the default active-only refetch: disabled observers retain invalidation
  // until enabled, including when an older prefetch was already in flight.
  await queryClient.invalidateQueries({ queryKey });
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
