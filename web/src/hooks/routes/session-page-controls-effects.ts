import type { EnqueueObject } from "@xstate/store";

import type { SessionPromptSendResult } from "@/systems/session";

import type {
  SessionPageControlsEventPayloadMap,
  SessionPageControlsState,
} from "./session-page-controls-store";

type SessionPageControlsEnqueue = EnqueueObject<
  SessionPageControlsState,
  never,
  SessionPageControlsEventPayloadMap
>;

export function enqueueBusyInput(
  execute: () => Promise<SessionPromptSendResult>,
  enqueue: SessionPageControlsEnqueue,
  requestId: number
) {
  enqueue.effect(async ({ trigger }) => {
    try {
      const result = await execute();
      trigger.busyInputSucceeded({ requestId, result });
    } catch (error) {
      trigger.busyInputFailed({ requestId, error: normalizeError(error, "Busy input failed") });
    }
  });
}

export function enqueueStop(
  execute: () => Promise<unknown>,
  enqueue: SessionPageControlsEnqueue,
  requestId: number,
  failureMessage: string | null
) {
  enqueue.effect(async ({ trigger }) => {
    try {
      const result = await execute();
      trigger.stopSucceeded({ requestId, result });
    } catch (error) {
      trigger.stopFailed({
        error: normalizeError(error, "Failed to stop session."),
        failureMessage,
        requestId,
      });
    }
  });
}

function normalizeError(error: unknown, fallback: string): Error {
  return error instanceof Error ? error : new Error(fallback);
}
