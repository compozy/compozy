import { createStoreLogic, type EnqueueObject, type TriggerObject } from "@xstate/store";

import { notifyUser } from "@/lib/user-feedback";

export type HomeAttentionResolvedKind = "approved" | "rejected";

export type HomeAttentionOperation =
  | { requestId: number; status: "pending" }
  | { requestId: number; status: "resolved"; resolved: HomeAttentionResolvedKind };

export interface HomeAttentionActionsContext {
  nextRequestId: number;
  operations: Record<string, HomeAttentionOperation>;
}

interface HomeAttentionExecution {
  execute: (id: string) => Promise<unknown>;
  refreshOverview: () => Promise<void>;
}

type HomeAttentionIntent = "approve" | "reject" | "retry";

const intentOutcome: Record<
  HomeAttentionIntent,
  { failure: string; resolved: HomeAttentionResolvedKind | null }
> = {
  approve: { failure: "Failed to approve task", resolved: "approved" },
  reject: { failure: "Failed to reject task", resolved: "rejected" },
  retry: { failure: "Failed to retry run", resolved: null },
};

type HomeAttentionActionsEventPayloadMap = {
  approveRequested: HomeAttentionExecution & { id: string };
  rejectRequested: HomeAttentionExecution & { id: string };
  retryRequested: HomeAttentionExecution & { id: string };
  overviewRefreshFailed: { message: string };
  rowOperationCompleted: { id: string; refreshOverview: () => Promise<void>; requestId: number };
  rowOperationFailed: { error: string; id: string; requestId: number };
  rowResolved: {
    id: string;
    kind: HomeAttentionResolvedKind;
    refreshOverview: () => Promise<void>;
    requestId: number;
  };
};

type HomeAttentionActionsEnqueue = EnqueueObject<
  HomeAttentionActionsContext,
  never,
  HomeAttentionActionsEventPayloadMap
>;

type HomeAttentionActionsTrigger = TriggerObject<HomeAttentionActionsEventPayloadMap>;

export function createHomeAttentionActionsLogic() {
  return createStoreLogic<HomeAttentionActionsContext, HomeAttentionActionsEventPayloadMap>({
    context: { nextRequestId: 0, operations: {} },
    on: {
      approveRequested: (context, event, enqueue) =>
        startOperation(context, event, "approve", enqueue),
      rejectRequested: (context, event, enqueue) =>
        startOperation(context, event, "reject", enqueue),
      retryRequested: (context, event, enqueue) => startOperation(context, event, "retry", enqueue),
      overviewRefreshFailed: (context, event, enqueue) => {
        enqueue.effect(() => notifyUser({ message: event.message, tone: "error" }));
        return context;
      },
      rowOperationCompleted: (context, event, enqueue) => {
        if (!matchingOperation(context, event)) return;
        enqueueOverviewRefresh(enqueue, event.refreshOverview);
        return { ...context, operations: withoutOperation(context, event.id) };
      },
      rowOperationFailed: (context, event, enqueue) => {
        if (!matchingOperation(context, event)) return;
        enqueue.effect(() => notifyUser({ message: event.error, tone: "error" }));
        return { ...context, operations: withoutOperation(context, event.id) };
      },
      rowResolved: (context, event, enqueue) => {
        const operation = matchingOperation(context, event);
        if (!operation) return;
        enqueueOverviewRefresh(enqueue, event.refreshOverview);
        return {
          ...context,
          operations: {
            ...context.operations,
            [event.id]: {
              requestId: operation.requestId,
              status: "resolved",
              resolved: event.kind,
            },
          },
        };
      },
    },
  });
}

function startOperation(
  context: HomeAttentionActionsContext,
  event: HomeAttentionExecution & { id: string },
  kind: HomeAttentionIntent,
  enqueue: HomeAttentionActionsEnqueue
): HomeAttentionActionsContext | undefined {
  const { id } = event;
  if (!id || context.operations[id]?.status === "pending") return;
  const requestId = context.nextRequestId + 1;
  enqueue.effect(async ({ trigger }) => {
    await executeOperation(trigger, event, id, requestId, kind);
  });
  return {
    ...context,
    nextRequestId: requestId,
    operations: {
      ...context.operations,
      [id]: { requestId, status: "pending" },
    },
  };
}

async function executeOperation(
  trigger: HomeAttentionActionsTrigger,
  execution: HomeAttentionExecution,
  id: string,
  requestId: number,
  kind: HomeAttentionIntent
): Promise<void> {
  const outcome = intentOutcome[kind];
  try {
    await execution.execute(id);
    if (outcome.resolved !== null) {
      trigger.rowResolved({
        id,
        kind: outcome.resolved,
        refreshOverview: execution.refreshOverview,
        requestId,
      });
    } else {
      trigger.rowOperationCompleted({ id, refreshOverview: execution.refreshOverview, requestId });
    }
  } catch (error) {
    trigger.rowOperationFailed({
      error: errorMessage(error, outcome.failure),
      id,
      requestId,
    });
  }
}

function enqueueOverviewRefresh(
  enqueue: HomeAttentionActionsEnqueue,
  refreshOverview: () => Promise<void>
) {
  enqueue.effect(async ({ trigger }) => {
    try {
      await refreshOverview();
    } catch (error) {
      trigger.overviewRefreshFailed({
        message: errorMessage(error, "Failed to refresh dashboard"),
      });
    }
  });
}

function matchingOperation(
  context: HomeAttentionActionsContext,
  event: { id: string; requestId: number }
): HomeAttentionOperation | undefined {
  const operation = context.operations[event.id];
  return operation?.requestId === event.requestId ? operation : undefined;
}

function withoutOperation(
  context: HomeAttentionActionsContext,
  id: string
): Record<string, HomeAttentionOperation> {
  const { [id]: removed, ...operations } = context.operations;
  return removed ? operations : context.operations;
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim() ? error.message : fallback;
}
