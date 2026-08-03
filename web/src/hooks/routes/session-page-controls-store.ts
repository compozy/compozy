import { createStoreLogic } from "@xstate/store";

import { notifyUser } from "@/lib/user-feedback";
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

export type SessionBusyInputKind = "interrupt" | "queue" | "steer";

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

export interface SessionPageControlsState {
  busyInput: BusyInputIdle | BusyInputPending;
  stop: { phase: "idle" | "pending"; requestId: number };
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
    execute: () => Promise<unknown>;
    kind: SessionBusyInputKind;
    message: string;
  };
  busyInputSucceeded: { requestId: number };
  stopFailed: { error: Error; failureMessage: string | null; requestId: number };
  stopRequested: { execute: () => Promise<unknown>; failureMessage: string | null };
  stopSucceeded: { requestId: number };
  resumeFailed: { failure: SessionResumeFailure; requestId: number };
  resumeFailureDismissed: Record<string, never>;
  resumeRequested: { resumeSession: () => Promise<unknown>; sessionId: string };
  resumeSucceeded: { requestId: number };
};

export type SessionBusyInputSettlement =
  | { outcome: "succeeded"; requestId: number }
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
        enqueue.emit.busyInputSettled({ outcome: "succeeded", requestId: event.requestId });
        return { ...context, busyInput: { phase: "idle", requestId: event.requestId } };
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
        return { ...context, stop: { ...context.stop, phase: "idle" } };
      },
      stopRequested: (context, event, enqueue) => {
        if (context.stop.phase === "pending" || context.resume.phase === "pending") return;
        const requestId = context.stop.requestId + 1;
        enqueueStop(event.execute, enqueue, requestId, event.failureMessage);
        return { ...context, stop: { phase: "pending", requestId } };
      },
      stopSucceeded: (context, event) => {
        if (context.stop.phase !== "pending" || context.stop.requestId !== event.requestId) return;
        return { ...context, stop: { ...context.stop, phase: "idle" } };
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
        if (context.resume.phase === "pending" || context.stop.phase === "pending") return;
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

function describeActionError(error: Error, fallback: string): string {
  return error.message.trim().length > 0 ? error.message : fallback;
}
