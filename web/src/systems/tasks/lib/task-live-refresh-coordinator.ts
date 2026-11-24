import { createStore, type EnqueueObject } from "@xstate/store";
import type { Query, QueryClient } from "@tanstack/react-query";

import { tasksKeys } from "./query-keys";
import type { TaskRunDetailView, TaskRunInspectView } from "../types";

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

function isTasksKey(key: readonly unknown[]): boolean {
  return (
    key.length >= tasksKeys.all.length &&
    tasksKeys.all.every((segment, index) => key[index] === segment)
  );
}

function isTaskCatalogQuery(query: Query): boolean {
  const key = query.queryKey;
  const base = tasksKeys.all.length;
  return (
    isTasksKey(key) && (key[base] === "list" || key[base] === "dashboard" || key[base] === "inbox")
  );
}

function isTaskLiveQuery(query: Query, taskId: string): boolean {
  const key = query.queryKey;
  if (!isTasksKey(key)) return false;
  const base = tasksKeys.all.length;

  switch (key[base]) {
    case "detail":
    case "runs":
    case "timeline":
    case "tree":
    case "profile":
      return key[base + 1] === taskId;
    case "inspect": {
      if (key[base + 1] === "task") return key[base + 2] === taskId;
      if (key[base + 1] !== "run") return false;
      const inspect = query.state.data as TaskRunInspectView | undefined;
      return !inspect?.task?.id || inspect.task.id === taskId;
    }
    case "run-detail": {
      const detail = query.state.data as TaskRunDetailView | undefined;
      // Run ownership is immutable; a missing snapshot still needs a canonical reread.
      return !detail?.run?.task_id || detail.run.task_id === taskId;
    }
    case "agent-context":
      return true;
    case "reviews":
      return key[base + 1] !== "task" || key[base + 2] === taskId;
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
