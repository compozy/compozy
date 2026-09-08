import { createStoreLogic } from "@xstate/store";

import { notifyUser } from "@/lib/user-feedback";
import {
  beginUnconfirmedRetry,
  findUnconfirmedSend,
  isSendAcknowledgmentLost,
  markSendUnconfirmed,
  resolveUnconfirmedSend,
  SessionApiError,
  type SessionPromptSendResult,
  type SessionSendAction,
  type SessionSendEnvelope,
  type SessionState,
  type UnconfirmedSend,
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

export type SessionBusyInputKind = SessionSendAction;

interface BusyInputIdle {
  phase: "idle";
  requestId: number;
}

interface BusyInputPending {
  phase: "pending";
  requestId: number;
  kind: SessionBusyInputKind;
  message: string;
  /** The identity and content on the wire, retained so a lost acknowledgment stays retryable. */
  send: SessionSendEnvelope | null;
  /** The unconfirmed row this request replays, when it is a Retry. */
  retryOf: string | null;
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

/** The daemon's word on a turn stop (`POST …/prompt/cancel`). */
export type SessionStopVerdict = "canceled" | "nothing-in-flight";

/**
 * The stop arrived after the turn had already finished (US-009.EC-2): nothing
 * was canceled, the turn completed on its own. Shown as a faint note for a few
 * seconds, then cleared — never a remount, never "canceled".
 */
export interface SessionStopCompletionNote {
  requestId: number;
  at: number;
}

/** How long the completion note stays before the store clears it. */
export const STOP_COMPLETION_NOTE_MS = 6_000;

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
  /** A turn stop the daemon answered `nothing-in-flight`; `null` once it expires or a new request starts. */
  stopCompletion: SessionStopCompletionNote | null;
  nextRequestId: number;
  /** The cap the daemon last named in a `queue_full` refusal; unknown until then. */
  queueCap: number | null;
  resume: {
    failure: SessionResumeFailure | null;
    phase: "idle" | "pending";
    requestId: number;
  };
  /**
   * Sends whose acknowledgment was lost, held in memory with their identity
   * until a replay observes an outcome or the operator discards them. Never a
   * durable outbox: a reload drops them and replay protection still holds.
   */
  unconfirmedSends: UnconfirmedSend[];
}

export type SessionPageControlsEventPayloadMap = {
  busyInputFailed: { error: Error; requestId: number };
  busyInputRequested: {
    execute: () => Promise<SessionPromptSendResult>;
    kind: SessionBusyInputKind;
    message: string;
    /** The envelope on the wire; absent for sends whose identity the caller does not retain. */
    send?: SessionSendEnvelope;
    /** Replays the identity of this unconfirmed row. */
    retryOf?: string;
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
  /** The stop request settled; `result` is the daemon's answer when the adapter returned one. */
  stopSucceeded: { requestId: number; result?: unknown };
  /** The completion note's time is up. */
  stopCompletionExpired: { requestId: number };
  resumeFailed: { failure: SessionResumeFailure; requestId: number };
  resumeFailureDismissed: Record<string, never>;
  resumeRequested: { resumeSession: () => Promise<unknown>; sessionId: string };
  resumeSucceeded: { requestId: number };
  /** A streaming prompt POST lost its acknowledgment mid-turn: retain its identity as unconfirmed. */
  sendUnconfirmedObserved: { send: SessionSendEnvelope };
  unconfirmedSendDiscarded: { id: string };
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
      stopCompletion: null,
      nextRequestId: 0,
      queueCap: null,
      resume: { failure: null, phase: "idle", requestId: 0 },
      unconfirmedSends: [],
    },
    on: {
      busyInputFailed: (context, event, enqueue) => {
        if (!isCurrentBusyInput(context, event.requestId)) return;
        enqueue.emit.busyInputSettled({
          error: event.error,
          outcome: "failed",
          requestId: event.requestId,
        });
        return {
          ...context,
          busyInput: { phase: "idle", requestId: event.requestId },
          queueCap: queueCapFromError(event.error) ?? context.queueCap,
          unconfirmedSends: unconfirmedAfterFailure(context, event.error),
        };
      },
      busyInputRequested: (context, event, enqueue) => {
        if (isBusyInputPending(context)) return;
        if (event.retryOf !== undefined) {
          const retried = findUnconfirmedSend(context.unconfirmedSends, event.retryOf);
          // One replay at a time per identity; a row that is gone has nothing to replay.
          if (retried === null || retried.phase !== "unconfirmed") return;
        }
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
            retryOf: event.retryOf ?? null,
            send: event.send ?? null,
          },
          nextRequestId: requestId,
          unconfirmedSends:
            event.retryOf === undefined
              ? context.unconfirmedSends
              : beginUnconfirmedRetry(context.unconfirmedSends, event.retryOf),
        };
      },
      busyInputSucceeded: (context, event, enqueue) => {
        if (!isCurrentBusyInput(context, event.requestId)) return;
        enqueue.emit.busyInputSettled({
          outcome: "succeeded",
          requestId: event.requestId,
          result: event.result,
        });
        // An observed outcome — replayed or fresh — resolves the retained identity.
        const retryOf = context.busyInput.retryOf;
        return {
          ...context,
          busyInput: { phase: "idle", requestId: event.requestId },
          unconfirmedSends:
            retryOf === null
              ? context.unconfirmedSends
              : resolveUnconfirmedSend(context.unconfirmedSends, retryOf),
        };
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
          // A fresh request supersedes any completion note still showing.
          stopCompletion: null,
        };
      },
      stopSucceeded: (context, event, enqueue) => {
        if (context.stop.phase !== "pending" || context.stop.requestId !== event.requestId) return;
        const verdict = stopVerdictFromResult(event.result);
        // The daemon found nothing in flight: the turn had already finished
        // when the stop arrived (US-009.EC-2). That answer is authoritative —
        // the stop settles at once and the reader gets the completion note
        // instead of a "Stopping…" that waits on a reread. The reread still
        // runs; it only confirms what the daemon said.
        if (context.stop.scope === "turn" && verdict === "nothing-in-flight") {
          const requestId = event.requestId;
          enqueue.effect(({ trigger }) => {
            armCompletionTimer(trigger, () => trigger.stopCompletionExpired({ requestId }));
          });
          return {
            ...context,
            stop: { phase: "idle", requestId },
            stopCompletion: { requestId, at: Date.now() },
          };
        }
        const stopping: StopStopping = { ...context.stop, phase: "stopping" };
        return {
          ...context,
          stop: hasStopLanded(stopping, context.lifecycle)
            ? { phase: "idle", requestId: context.stop.requestId }
            : stopping,
        };
      },
      stopCompletionExpired: (context, event) => {
        if (
          context.stopCompletion === null ||
          context.stopCompletion.requestId !== event.requestId
        ) {
          return;
        }
        return { ...context, stopCompletion: null };
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
      sendUnconfirmedObserved: (context, event) => ({
        ...context,
        unconfirmedSends: markSendUnconfirmed(context.unconfirmedSends, event.send),
      }),
      unconfirmedSendDiscarded: (context, event) => {
        const send = findUnconfirmedSend(context.unconfirmedSends, event.id);
        // A replay in flight settles on its own answer; only a waiting row can be dropped.
        if (send === null || send.phase !== "unconfirmed") return;
        return {
          ...context,
          unconfirmedSends: resolveUnconfirmedSend(context.unconfirmedSends, event.id),
        };
      },
    },
  });
}

export type SessionPageControlsStore = ReturnType<
  ReturnType<typeof createSessionPageControlsLogic>["createStore"]
>;

const completionTimers = new WeakMap<object, ReturnType<typeof setTimeout>>();

// One completion note at a time per store: a newer verdict replaces the timer
// of the previous one. The timer lives in a store effect, never in a component.
function armCompletionTimer(trigger: object, expire: () => void): void {
  const previous = completionTimers.get(trigger);
  if (previous !== undefined) clearTimeout(previous);
  completionTimers.set(
    trigger,
    setTimeout(() => {
      completionTimers.delete(trigger);
      expire();
    }, STOP_COMPLETION_NOTE_MS)
  );
}

/** The verdict a turn stop resolved with, when the adapter returned the daemon's body. */
export function stopVerdictFromResult(result: unknown): SessionStopVerdict | null {
  if (typeof result !== "object" || result === null || !("outcome" in result)) return null;
  const outcome = (result as { outcome?: unknown }).outcome;
  return outcome === "canceled" || outcome === "nothing-in-flight" ? outcome : null;
}

function isCurrentBusyInput(
  context: SessionPageControlsState,
  requestId: number
): context is SessionPageControlsState & { busyInput: BusyInputPending } {
  return context.busyInput.phase === "pending" && context.busyInput.requestId === requestId;
}

export function isBusyInputPending(context: SessionPageControlsState): boolean {
  return context.busyInput.phase === "pending";
}

/**
 * What a failed send leaves behind. A lost acknowledgment retains the identity
 * as an unconfirmed row (a failed replay returns to waiting); a daemon answer —
 * refusal or conflict — is an outcome and resolves any row it replayed.
 */
function unconfirmedAfterFailure(
  context: SessionPageControlsState & { busyInput: BusyInputPending },
  error: Error
): UnconfirmedSend[] {
  const { retryOf, send } = context.busyInput;
  if (send !== null && isSendAcknowledgmentLost(error)) {
    return markSendUnconfirmed(context.unconfirmedSends, send);
  }
  return retryOf === null
    ? context.unconfirmedSends
    : resolveUnconfirmedSend(context.unconfirmedSends, retryOf);
}

function queueCapFromError(error: Error): number | null {
  return error instanceof SessionApiError && error.code === "queue_full" ? error.queueCap : null;
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
