import { createStoreLogic } from "@xstate/store";

import { notifyUser } from "@/lib/user-feedback";

const PENDING_ACTION_KINDS = [
  "approve",
  "archive",
  "dismiss",
  "markRead",
  "reject",
  "retry",
] as const;
export type PendingActionKind = (typeof PENDING_ACTION_KINDS)[number];

type PendingByID = Record<string, number>;
type PendingActions = Record<PendingActionKind, PendingByID>;

interface TasksPageActionsContext {
  nextToken: number;
  pending: PendingActions;
}

interface ActionRequestedEvent {
  execute: () => Promise<unknown>;
  failureMessage: string;
  id: string;
  kind: PendingActionKind;
  successMessage?: string;
}

type TasksPageActionsEvents = {
  actionFailed: ActionRequestedEvent & { error: unknown; token: number };
  actionRequested: ActionRequestedEvent;
  actionSucceeded: ActionRequestedEvent & { token: number };
};

function emptyPending(): PendingActions {
  return {
    approve: {},
    archive: {},
    dismiss: {},
    markRead: {},
    reject: {},
    retry: {},
  };
}

function settleAction(
  context: TasksPageActionsContext,
  event: { id: string; kind: PendingActionKind; token: number }
): TasksPageActionsContext | undefined {
  if (context.pending[event.kind][event.id] !== event.token) return;
  const { [event.id]: _token, ...remaining } = context.pending[event.kind];
  return { ...context, pending: { ...context.pending, [event.kind]: remaining } };
}

export const tasksPageActionsLogic = createStoreLogic<
  TasksPageActionsContext,
  TasksPageActionsEvents
>({
  context: (): TasksPageActionsContext => ({ nextToken: 0, pending: emptyPending() }),
  on: {
    actionFailed: (context, event, enqueue) => {
      const next = settleAction(context, event);
      if (!next) return;
      enqueue.effect(() =>
        notifyUser({
          message: event.error instanceof Error ? event.error.message : event.failureMessage,
          tone: "error",
        })
      );
      return next;
    },
    actionRequested: (context, event, enqueue) => {
      if (context.pending[event.kind][event.id] !== undefined) return;
      const token = context.nextToken + 1;
      enqueue.effect(async ({ trigger }) => {
        try {
          await event.execute();
          trigger.actionSucceeded({ ...event, token });
        } catch (error) {
          trigger.actionFailed({ ...event, error, token });
        }
      });
      return {
        nextToken: token,
        pending: {
          ...context.pending,
          [event.kind]: { ...context.pending[event.kind], [event.id]: token },
        },
      };
    },
    actionSucceeded: (context, event, enqueue) => {
      const next = settleAction(context, event);
      if (!next) return;
      if (event.successMessage) {
        enqueue.effect(() => notifyUser({ message: event.successMessage ?? "", tone: "success" }));
      }
      return next;
    },
  },
});
