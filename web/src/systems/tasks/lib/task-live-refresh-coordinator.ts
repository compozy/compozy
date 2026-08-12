import { createStore, type EnqueueObject } from "@xstate/store";
import type { Query, QueryClient } from "@tanstack/react-query";

import { tasksKeys } from "./query-keys";

type TaskLiveRefreshPhase = "idle" | "scheduled" | "refreshing" | "disposed";

interface TaskLiveRefreshContext {
  catalogsDirty: boolean;
  followUpRequired: boolean;
  phase: TaskLiveRefreshPhase;
}

type TaskLiveRefreshEvents = {
  disposed: Record<never, never>;
  refreshRequested: Record<never, never>;
  refreshSettled: Record<never, never>;
  refreshStarted: Record<never, never>;
};

type TaskLiveRefreshEnqueue = EnqueueObject<TaskLiveRefreshContext, never, TaskLiveRefreshEvents>;

function isTaskCatalogQuery(query: Query): boolean {
  const key = query.queryKey;
  return (
    key[0] === tasksKeys.all[0] &&
    (key[1] === "list" || key[1] === "dashboard" || key[1] === "inbox")
  );
}

function isTaskLiveQuery(query: Query, taskId: string): boolean {
  const key = query.queryKey;
  if (key[0] !== tasksKeys.all[0]) return false;

  switch (key[1]) {
    case "detail":
    case "runs":
    case "timeline":
    case "tree":
    case "profile":
    case "bridge-notifications":
      return key[2] === taskId;
    case "inspect":
      return key[2] === "run" || (key[2] === "task" && key[3] === taskId);
    case "run-detail":
    case "agent-context":
      return true;
    case "reviews":
      return key[2] !== "task" || key[3] === taskId;
    default:
      return false;
  }
}

function enqueueRefreshStart(enqueue: TaskLiveRefreshEnqueue): void {
  enqueue.effect(({ trigger }) => {
    queueMicrotask(() => trigger.refreshStarted());
  });
}

/**
 * Event-driven coalescer for task stream wake-ups. One synchronous burst becomes
 * one refresh; events received during that refresh request exactly one follow-up.
 */
export function createTaskLiveRefreshStore(queryClient: QueryClient, taskId: string) {
  return createStore<TaskLiveRefreshContext, TaskLiveRefreshEvents>({
    context: {
      catalogsDirty: false,
      followUpRequired: false,
      phase: "idle",
    },
    on: {
      refreshRequested: (context, _event, enqueue) => {
        if (context.phase === "disposed") return;
        if (context.phase === "refreshing") {
          return { ...context, followUpRequired: true };
        }
        if (context.phase === "scheduled") return context;
        enqueueRefreshStart(enqueue);
        return { ...context, phase: "scheduled" };
      },
      refreshStarted: (context, _event, enqueue) => {
        if (context.phase !== "scheduled") return;
        const invalidateCatalogs = !context.catalogsDirty;
        enqueue.effect(async ({ trigger }) => {
          try {
            const operations: Promise<unknown>[] = [
              queryClient.invalidateQueries({
                predicate: query => isTaskLiveQuery(query, taskId),
              }),
            ];
            if (invalidateCatalogs) {
              operations.push(
                queryClient.invalidateQueries({
                  predicate: isTaskCatalogQuery,
                  refetchType: "none",
                })
              );
            }
            await Promise.all(operations);
          } catch (error) {
            console.error("Failed to refresh task live data", error);
          } finally {
            trigger.refreshSettled();
          }
        });
        return {
          ...context,
          catalogsDirty: true,
          phase: "refreshing",
        };
      },
      refreshSettled: (context, _event, enqueue) => {
        if (context.phase !== "refreshing") return;
        if (!context.followUpRequired) return { ...context, phase: "idle" };
        enqueueRefreshStart(enqueue);
        return { ...context, followUpRequired: false, phase: "scheduled" };
      },
      disposed: (context, _event, enqueue) => {
        if (context.phase === "disposed") return;
        if (context.catalogsDirty) {
          enqueue.effect(async () => {
            try {
              await queryClient.refetchQueries({ predicate: isTaskCatalogQuery, type: "active" });
            } catch (error) {
              console.error("Failed to reconcile task catalogs", error);
            }
          });
        }
        return {
          catalogsDirty: false,
          followUpRequired: false,
          phase: "disposed",
        };
      },
    },
  });
}

export type TaskLiveRefreshStore = ReturnType<typeof createTaskLiveRefreshStore>;
