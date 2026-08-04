import type { EnqueueObject } from "@xstate/store";

import type {
  SessionPageControlsEventPayloadMap,
  SessionPageControlsState,
  SessionResumeFailure,
} from "./session-page-controls-store";

type SessionPageControlsEnqueue = EnqueueObject<
  SessionPageControlsState,
  never,
  SessionPageControlsEventPayloadMap
>;

export function enqueueBusyInput(
  execute: () => Promise<unknown>,
  enqueue: SessionPageControlsEnqueue,
  requestId: number
) {
  enqueue.effect(async ({ trigger }) => {
    try {
      await execute();
      trigger.busyInputSucceeded({ requestId });
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
      await execute();
      trigger.stopSucceeded({ requestId });
    } catch (error) {
      trigger.stopFailed({
        error: normalizeError(error, "Failed to stop session."),
        failureMessage,
        requestId,
      });
    }
  });
}

export function enqueueResume(
  resumeSession: () => Promise<unknown>,
  enqueue: SessionPageControlsEnqueue,
  requestId: number,
  sessionId: string
) {
  enqueue.effect(async ({ trigger }) => {
    try {
      await resumeSession();
      trigger.resumeSucceeded({ requestId });
    } catch (error) {
      trigger.resumeFailed({ failure: describeResumeFailure(sessionId, error), requestId });
    }
  });
}

function normalizeError(error: unknown, fallback: string): Error {
  return error instanceof Error ? error : new Error(fallback);
}

const PROVIDER_VALIDATION_PATTERN =
  /validate agent "([^"]+)" with provider "([^"]+)" for session "([^"]+)"/;

function describeResumeFailure(sessionId: string, error: unknown): SessionResumeFailure {
  const message =
    error instanceof Error && error.message.trim().length > 0
      ? error.message
      : "Failed to attach session.";
  const match = message.match(PROVIDER_VALIDATION_PATTERN);
  if (!match) return { message, providerUnavailable: null };

  const [, agentName, missingProvider, parsedSessionId] = match;
  const providerName = missingProvider?.trim() ?? "";
  if (providerName.length === 0) return { message, providerUnavailable: null };

  return {
    message,
    providerUnavailable: {
      sessionId: parsedSessionId?.trim().length ? parsedSessionId : sessionId,
      missingProvider: providerName,
      agentName: agentName?.trim().length ? agentName : undefined,
    },
  };
}
