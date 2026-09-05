import { createStoreLogic } from "@xstate/store";

import { notifyUser } from "@/lib/user-feedback";
import type {
  SessionBusyInputAction,
  SessionPromptSendResult,
  SessionState,
} from "@/systems/session";
import { enqueueBusyInput, enqueueResume, enqueueStop } from "./session-page-controls-effects";

export interface ResumeProviderUnavailableDetail {
  agentName?: string;
  missingProvider: string;
  sessionId: string;
}

export interface SessionResumeFailure {
  message: string;
  providerUnavailable: ResumeProviderUnavailableDetail | null;
}

export type SessionBusyInputKind = SessionBusyInputAction;

interface BusyInputIdle {
  phase: "idle";
  requestId: number;
}

interface BusyInputPending {
  phase: "pending";
  requestId: number;
  kind: SessionBusyInputKind;
  message: string;
}

/** Which lifecycle a stop targets: the live turn (`prompt-cancel`) or the whole session (`stop`). */
export type SessionStopScope = "turn" | "session";

interface StopIdle {
  phase: "idle";
  requestId: number;
}

/**
 * The stop request is on the wire; nothing has been accepted yet. A retry of a
 * stop the daemon could not verify (`stop_verification_failed`) waits here for
 * the daemon's settled answer, so the daemon already reading `stopping` and any
 * unrelated metadata change on the session cannot release it early.
 */
interface StopPending {
  phase: "pending";
  requestId: number;
  scope: SessionStopScope;
  /** The turn the operator asked to stop, or "" when the poll had not reported one. */
  turnId: string;
  /** This request retries an unverified stop and resolves with its settled outcome. */
  retry: boolean;
}

/**
 * The daemon accepted the request. Acceptance is not termination: the control
 * keeps reading "Stopping…" until the session's own lifecycle says the stop
 * landed (US-009.AC-3).
 */
interface StopStopping {
  phase: "stopping";
  requestId: number;
  scope: SessionStopScope;
  turnId: string;
  retry: boolean;
}

export type SessionStopPhase = StopIdle | StopPending | StopStopping;

/** The daemon's last word on the session, as the page currently reads it. */
export interface SessionLifecycleEvidence {
  running: boolean;
  state: SessionState;
  turnId: string;
}

export interface SessionPageControlsState {
  busyInput: BusyInputIdle | BusyInputPending;
  lifecycle: SessionLifecycleEvidence;
  stop: SessionStopPhase;
  nextRequestId: number;
  resume: {
    failure: SessionResumeFailure | null;
    phase: "idle" | "pending";
    requestId: number;
  };
}

export type SessionPageControlsEventPayloadMap = {
  busyInputFailed: { error: Error; requestId: number };
  busyInputRequested: {
    execute: () => Promise<SessionPromptSendResult>;
    kind: SessionBusyInputKind;
    message: string;
  };
  busyInputSucceeded: { requestId: number; result: SessionPromptSendResult };
  lifecycleObserved: SessionLifecycleEvidence;
  stopFailed: { error: Error; failureMessage: string | null; requestId: number };
  stopRequested: {
    /** Resolves on acceptance, or — for a retry — on the daemon's settled answer. */
    execute: () => Promise<unknown>;
    failureMessage: string | null;
    /** Retry of an unverified stop: the same session stop, waited on until it settles. */
    retry?: boolean;
    scope: SessionStopScope;
    turnId: string;
  };
  stopSucceeded: { requestId: number };
  resumeFailed: { failure: SessionResumeFailure; requestId: number };
  resumeFailureDismissed: Record<string, never>;
  resumeRequested: { resumeSession: () => Promise<unknown>; sessionId: string };
  resumeSucceeded: { requestId: number };
};

export type SessionBusyInputSettlement =
  | { outcome: "succeeded"; requestId: number; result: SessionPromptSendResult }
  | { outcome: "failed"; error: Error; requestId: number };

type SessionPageControlsEmittedPayloadMap = {
  busyInputAccepted: { requestId: number };
  busyInputSettled: SessionBusyInputSettlement;
};

export function createSessionPageControlsLogic() {
  return createStoreLogic<
    SessionPageControlsState,
    SessionPageControlsEventPayloadMap,
    SessionPageControlsEmittedPayloadMap
  >({
    context: {
      busyInput: { phase: "idle", requestId: 0 },
      lifecycle: { running: false, state: "active", turnId: "" },
      stop: { phase: "idle", requestId: 0 },
      nextRequestId: 0,
      resume: { failure: null, phase: "idle", requestId: 0 },
    },
    on: {
      busyInputFailed: (context, event, enqueue) => {
        if (!isCurrentBusyInput(context, event.requestId)) return;
        enqueue.emit.busyInputSettled({
          error: event.error,
          outcome: "failed",
          requestId: event.requestId,
        });
        return { ...context, busyInput: { phase: "idle", requestId: event.requestId } };
      },
      busyInputRequested: (context, event, enqueue) => {
        if (isBusyInputPending(context)) return;
        const requestId = context.nextRequestId + 1;
        enqueue.emit.busyInputAccepted({ requestId });
        enqueueBusyInput(event.execute, enqueue, requestId);
        return {
          ...context,
          busyInput: {
            kind: event.kind,
            message: event.message,
            phase: "pending",
            requestId,
          },
          nextRequestId: requestId,
        };
      },
      busyInputSucceeded: (context, event, enqueue) => {
        if (!isCurrentBusyInput(context, event.requestId)) return;
        enqueue.emit.busyInputSettled({
          outcome: "succeeded",
          requestId: event.requestId,
          result: event.result,
        });
        return { ...context, busyInput: { phase: "idle", requestId: event.requestId } };
      },
      lifecycleObserved: (context, event) => {
        const lifecycle = { running: event.running, state: event.state, turnId: event.turnId };
        // Only an accepted stop settles on lifecycle evidence; a pending request
        // (including a waited retry) settles on its own answer, never on a reread.
        const stop =
          context.stop.phase === "stopping" && hasStopLanded(context.stop, lifecycle)
            ? { phase: "idle" as const, requestId: context.stop.requestId }
            : context.stop;
        return { ...context, lifecycle, stop };
      },
      stopFailed: (context, event, enqueue) => {
        if (context.stop.phase !== "pending" || context.stop.requestId !== event.requestId) return;
        enqueue.effect(() =>
          notifyUser({
            message:
              event.failureMessage ?? describeActionError(event.error, "Failed to stop session."),
            tone: "error",
          })
        );
        // The request never landed: the control returns to Stop so the operator can retry.
        return { ...context, stop: { phase: "idle", requestId: context.stop.requestId } };
      },
      stopRequested: (context, event, enqueue) => {
        // One stop per lifecycle: a second activation while one is pending or
        // still landing is dropped (US-009.EC-1). A stop the daemon could not
        // verify has already landed as `stopping`, so the phase is idle and the
        // explicit retry passes this same guard as a fresh request.
        if (context.stop.phase !== "idle" || context.resume.phase === "pending") return;
        const requestId = context.stop.requestId + 1;
        enqueueStop(event.execute, enqueue, requestId, event.failureMessage);
        return {
          ...context,
          stop: {
            phase: "pending",
            requestId,
            retry: event.retry === true,
            scope: event.scope,
            turnId: event.turnId,
          },
        };
      },
      stopSucceeded: (context, event) => {
        if (context.stop.phase !== "pending" || context.stop.requestId !== event.requestId) return;
        const stopping: StopStopping = { ...context.stop, phase: "stopping" };
        return {
          ...context,
          stop: hasStopLanded(stopping, context.lifecycle)
            ? { phase: "idle", requestId: context.stop.requestId }
            : stopping,
        };
      },
      resumeFailed: (context, event, enqueue) => {
        if (context.resume.phase !== "pending" || context.resume.requestId !== event.requestId) {
          return;
        }
        if (event.failure.providerUnavailable === null) {
          enqueue.effect(() => notifyUser({ message: event.failure.message, tone: "error" }));
        }
        return { ...context, resume: { ...context.resume, failure: event.failure, phase: "idle" } };
      },
      resumeFailureDismissed: context => {
        if (context.resume.failure === null) return;
        return { ...context, resume: { ...context.resume, failure: null } };
      },
      resumeRequested: (context, event, enqueue) => {
        if (context.resume.phase === "pending" || context.stop.phase !== "idle") return;
        const requestId = context.resume.requestId + 1;
        enqueueResume(event.resumeSession, enqueue, requestId, event.sessionId);
        return {
          ...context,
          resume: { failure: null, phase: "pending", requestId },
        };
      },
      resumeSucceeded: (context, event) => {
        if (context.resume.phase !== "pending" || context.resume.requestId !== event.requestId) {
          return;
        }
        return { ...context, resume: { ...context.resume, failure: null, phase: "idle" } };
      },
    },
  });
}

export type SessionPageControlsStore = ReturnType<
  ReturnType<typeof createSessionPageControlsLogic>["createStore"]
>;

function isCurrentBusyInput(
  context: SessionPageControlsState,
  requestId: number
): context is SessionPageControlsState & { busyInput: BusyInputPending } {
  return context.busyInput.phase === "pending" && context.busyInput.requestId === requestId;
}

export function isBusyInputPending(context: SessionPageControlsState): boolean {
  return context.busyInput.phase === "pending";
}

/** A stop request is on the wire or accepted and not yet confirmed by the daemon. */
export function isStopRequestActive(context: SessionPageControlsState): boolean {
  return context.stop.phase !== "idle";
}

/**
 * A retry of an unverified stop is waiting on the daemon's settled answer. It
 * releases only when that request settles — verified, unverified, or failed —
 * so the notice's action holds and no second retry goes out meanwhile.
 */
export function isStopRetryPending(context: SessionPageControlsState): boolean {
  return context.stop.phase !== "idle" && context.stop.retry;
}

/**
 * Whether the daemon's lifecycle already reflects an accepted stop. A turn
 * stop has landed once the session reports no running turn, or a *known*
 * different turn than the one the operator stopped (the rebound replacement).
 * A running session whose turn identity is missing is not evidence of either.
 * A session stop has landed once the daemon itself reads `stopping` or
 * `stopped`: from there the session state is the truth the page renders, and a
 * later retry stays possible. A waited retry reaches this only with the
 * daemon's settled answer in hand, so it lands at once and the read model
 * (attention still present, or a verified `stopped`) says what happened.
 */
export function hasStopLanded(
  stop: StopPending | StopStopping,
  lifecycle: SessionLifecycleEvidence
): boolean {
  if (stop.scope === "session") {
    return lifecycle.state === "stopping" || lifecycle.state === "stopped";
  }
  if (!lifecycle.running) return true;
  return stop.turnId.length > 0 && lifecycle.turnId.length > 0 && lifecycle.turnId !== stop.turnId;
}

function describeActionError(error: Error, fallback: string): string {
  return error.message.trim().length > 0 ? error.message : fallback;
}
