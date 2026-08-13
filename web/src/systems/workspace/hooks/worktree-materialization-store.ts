import { createStoreLogic } from "@xstate/store";

import type { WorktreePayload } from "../types";

/**
 * The request half of materializing one worktree. It owns only what no server
 * read can answer: whether a request is in flight, which row it produced, and
 * why a create call failed. Phase and readiness are read from the worktree row.
 */
export type WorktreeMaterializationRequestStatus =
  | "idle"
  | "creating"
  | "active"
  | "canceled"
  | "failed";

export interface WorktreeMaterializationRequestState {
  status: WorktreeMaterializationRequestStatus;
  attempt: number;
  /** Creates canceled before their 202 response; cancel them as soon as their id arrives. */
  cancelAfterCreateAttempts: number[];
  /** The name the operator asked for, kept so Retry repeats the same request. */
  name: string;
  worktreeId?: string;
  error?: string;
}

type WorktreeMaterializationEvents = {
  startRequested: {
    name: string;
    execute: (name: string) => Promise<WorktreePayload>;
    cancel: (worktreeId: string) => Promise<void>;
  };
  createSucceeded: {
    attempt: number;
    worktree: WorktreePayload;
    cancel: (worktreeId: string) => Promise<void>;
  };
  createFailed: { attempt: number; error: string };
  cancelRequested: { execute: (worktreeId: string) => Promise<void> };
  retryRequested: {
    execute: (name: string) => Promise<WorktreePayload>;
    cancel: (worktreeId: string) => Promise<void>;
  };
  failureObserved: { worktreeId: string; error: string };
  reset: {};
};

const INITIAL: WorktreeMaterializationRequestState = {
  status: "idle",
  attempt: 0,
  cancelAfterCreateAttempts: [],
  name: "",
};

export const worktreeMaterializationLogic = createStoreLogic<
  WorktreeMaterializationRequestState,
  WorktreeMaterializationEvents
>({
  context: INITIAL,
  on: {
    startRequested: (context, event, enqueue) => {
      if (context.status === "creating" || context.status === "active") return;
      const attempt = context.attempt + 1;
      enqueue.effect(async ({ trigger }) => {
        try {
          trigger.createSucceeded({
            attempt,
            worktree: await event.execute(event.name),
            cancel: event.cancel,
          });
        } catch (cause) {
          trigger.createFailed({ attempt, error: materializationError(cause) });
        }
      });
      return {
        status: "creating",
        attempt,
        cancelAfterCreateAttempts: context.cancelAfterCreateAttempts,
        name: event.name,
      };
    },
    createSucceeded: (context, event, enqueue) => {
      if (context.cancelAfterCreateAttempts.includes(event.attempt)) {
        enqueue.effect(async () => {
          await event.cancel(event.worktree.id);
        });
        const cancelAfterCreateAttempts = context.cancelAfterCreateAttempts.filter(
          attempt => attempt !== event.attempt
        );
        return context.attempt === event.attempt
          ? { ...INITIAL, attempt: context.attempt, cancelAfterCreateAttempts }
          : { ...context, cancelAfterCreateAttempts };
      }
      if (context.status !== "creating" || context.attempt !== event.attempt) return;
      return { ...context, status: "active", worktreeId: event.worktree.id };
    },
    createFailed: (context, event) => {
      if (context.cancelAfterCreateAttempts.includes(event.attempt)) {
        const cancelAfterCreateAttempts = context.cancelAfterCreateAttempts.filter(
          attempt => attempt !== event.attempt
        );
        return context.attempt === event.attempt
          ? { ...INITIAL, attempt: context.attempt, cancelAfterCreateAttempts }
          : { ...context, cancelAfterCreateAttempts };
      }
      if (context.status !== "creating" || context.attempt !== event.attempt) return;
      return { ...context, status: "failed", error: event.error };
    },
    cancelRequested: (context, event, enqueue) => {
      if (context.status === "creating") {
        return {
          ...context,
          cancelAfterCreateAttempts: [
            ...context.cancelAfterCreateAttempts.filter(attempt => attempt !== context.attempt),
            context.attempt,
          ],
          status: "canceled",
        };
      }
      const worktreeId = context.worktreeId;
      if (context.status !== "active" || !worktreeId) return;
      // The daemon rolls the artifacts back and deletes the row; a create that
      // already finished answers `worktree_not_pending` and stays as it is.
      enqueue.effect(async () => {
        await event.execute(worktreeId);
      });
      return { ...context, status: "canceled" };
    },
    retryRequested: (context, event, enqueue) => {
      if (context.status !== "active" && context.status !== "failed") return;
      const attempt = context.attempt + 1;
      enqueue.effect(async ({ trigger }) => {
        try {
          trigger.createSucceeded({
            attempt,
            worktree: await event.execute(context.name),
            cancel: event.cancel,
          });
        } catch (cause) {
          trigger.createFailed({ attempt, error: materializationError(cause) });
        }
      });
      return {
        status: "creating",
        attempt,
        cancelAfterCreateAttempts: context.cancelAfterCreateAttempts,
        name: context.name,
      };
    },
    failureObserved: (context, event) => {
      if (context.status !== "active" || context.worktreeId !== event.worktreeId) return;
      return { ...context, status: "failed", error: event.error };
    },
    reset: context => ({
      ...INITIAL,
      attempt: context.attempt,
      cancelAfterCreateAttempts: context.cancelAfterCreateAttempts,
    }),
  },
});

function materializationError(error: unknown): string {
  return error instanceof Error && error.message.trim() !== ""
    ? error.message
    : "Failed to create the worktree.";
}
