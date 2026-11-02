import { createStore } from "@xstate/store";
import type { QueryClient, QueryKey } from "@tanstack/react-query";

import { createQueryLiveRefreshCoordinator } from "@/lib/query-live-refresh-coordinator";
import { LOOP_AGGREGATE_KEY, loopsKeys } from "./query-keys";
import { LIVE_REFETCH_INTERVAL } from "./query-options";

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
  const coordinator = createQueryLiveRefreshCoordinator(queryClient, LIVE_REFETCH_INTERVAL);
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
          for (const queryKey of keys) coordinator.refresh(queryKey);
        });
        return { ...context, pending: [], phase: "idle" };
      },
      disposed: context => ({ ...context, closed: true }),
    },
  });
}
