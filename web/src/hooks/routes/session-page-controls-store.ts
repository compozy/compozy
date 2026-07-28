import { createStoreLogic } from "@xstate/store";

import type { QueuedPrompt } from "@/components/assistant-ui/session-composer-queued-prompts";
import { notifyUser } from "@/lib/user-feedback";
import {
  enqueueBusyInput,
  enqueueStop,
  enqueueQueuedSteer,
  enqueueQueueRemoval,
  enqueueResume,
} from "./session-page-controls-effects";

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
  queueScope: string;
}

interface PendingQueueEdit {
  kind: "remove" | "steer";
  originalIndex: number;
  prompt: QueuedPrompt;
}

export interface SessionPageControlsState {
  busyInput: BusyInputIdle | BusyInputPending;
  stop: { phase: "idle" | "pending"; requestId: number };
  nextRequestId: number;
  pendingQueueEdits: Record<number, PendingQueueEdit>;
  queueScope: string;
  queuedPrompts: QueuedPrompt[];
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
  busyInputSucceeded: { requestId: number; result: unknown };
  stopFailed: { error: Error; failureMessage: string | null; requestId: number };
  stopRequested: { execute: () => Promise<unknown>; failureMessage: string | null };
  stopSucceeded: { requestId: number };
  queuedPromptEditFailed: { error: Error; requestId: number; stage: "cancel" | "steer" };
  queuedPromptEditSucceeded: { requestId: number };
  queuedPromptRemovalRequested: {
    execute: () => Promise<void>;
    queueEntryId: string;
  };
  queuedPromptSteerRequested: {
    cancel: () => Promise<void>;
    prompt: QueuedPrompt;
    steer: () => Promise<void>;
  };
  queueScopeChanged: { queueScope: string };
  resumeFailed: { failure: SessionResumeFailure; requestId: number };
  resumeFailureDismissed: {};
  resumeRequested: { resumeSession: () => Promise<unknown>; sessionId: string };
  resumeSucceeded: { requestId: number };
};

export type SessionBusyInputSettlement =
  | { outcome: "succeeded"; requestId: number }
  | { outcome: "failed"; error: Error; requestId: number };

type SessionPageControlsEmittedPayloadMap = {
  busyInputAccepted: { requestId: number };
  busyInputSettled: SessionBusyInputSettlement;
  operationFailed: { message: string };
  operationSucceeded: { message: string };
};

export function createSessionPageControlsLogic(initialQueueScope = "settled") {
  return createStoreLogic<
    SessionPageControlsState,
    SessionPageControlsEventPayloadMap,
    SessionPageControlsEmittedPayloadMap
  >({
    context: {
      busyInput: { phase: "idle", requestId: 0 },
      stop: { phase: "idle", requestId: 0 },
      nextRequestId: 0,
      pendingQueueEdits: {},
      queueScope: initialQueueScope,
      queuedPrompts: [],
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
        if (context.busyInput.phase === "pending") return;
        const requestId = context.nextRequestId + 1;
        enqueue.emit.busyInputAccepted({ requestId });
        enqueueBusyInput(event.execute, enqueue, requestId);
        return {
          ...context,
          busyInput: {
            kind: event.kind,
            message: event.message,
            phase: "pending",
            queueScope: context.queueScope,
            requestId,
          },
          nextRequestId: requestId,
        };
      },
      busyInputSucceeded: (context, event, enqueue) => {
        if (!isCurrentBusyInput(context, event.requestId)) return;
        const busyInput = context.busyInput;
        let queuedPrompts = context.queuedPrompts;
        let successMessage: string | null = null;
        if (busyInput.kind === "queue") {
          const queuedPrompt = queuedPromptFromResult(event.result, busyInput.message);
          if (
            queuedPrompt &&
            busyInput.queueScope === context.queueScope &&
            !queuedPrompts.some(prompt => prompt.id === queuedPrompt.id)
          ) {
            queuedPrompts = [...queuedPrompts, queuedPrompt];
          }
          if (queuedPrompt) successMessage = "Prompt queued.";
        } else if (busyInput.kind === "interrupt") {
          queuedPrompts = [];
          successMessage = "Prompt interrupted.";
        } else {
          successMessage = "Steer staged.";
        }
        if (successMessage) {
          enqueue.effect(() => notifyUser({ message: successMessage, tone: "success" }));
        }
        enqueue.emit.busyInputSettled({ outcome: "succeeded", requestId: event.requestId });
        return {
          ...context,
          busyInput: { phase: "idle", requestId: event.requestId },
          queuedPrompts,
        };
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
        if (context.stop.phase === "pending" || context.resume.phase === "pending") {
          return;
        }
        const requestId = context.stop.requestId + 1;
        enqueueStop(event.execute, enqueue, requestId, event.failureMessage);
        return { ...context, stop: { phase: "pending", requestId } };
      },
      stopSucceeded: (context, event) => {
        if (context.stop.phase !== "pending" || context.stop.requestId !== event.requestId) return;
        return { ...context, stop: { ...context.stop, phase: "idle" } };
      },
      queuedPromptEditFailed: (context, event, enqueue) => {
        const pending = context.pendingQueueEdits[event.requestId];
        if (!pending) return;
        const pendingQueueEdits = withoutQueueEdit(context.pendingQueueEdits, event.requestId);
        const queuedPrompts = restoreQueuedPrompt(
          context.queuedPrompts,
          pending.prompt,
          pending.originalIndex
        );
        const fallback =
          pending.kind === "steer" && event.stage === "cancel"
            ? "Steer staged, but couldn't remove queued prompt."
            : pending.kind === "steer"
              ? "Couldn't steer queued prompt."
              : "Couldn't remove queued prompt.";
        enqueue.effect(() =>
          notifyUser({ message: describeActionError(event.error, fallback), tone: "error" })
        );
        return { ...context, pendingQueueEdits, queuedPrompts };
      },
      queuedPromptEditSucceeded: (context, event, enqueue) => {
        const pending = context.pendingQueueEdits[event.requestId];
        if (!pending) return;
        if (pending.kind === "steer") {
          enqueue.effect(() => notifyUser({ message: "Steer staged.", tone: "success" }));
        }
        return {
          ...context,
          pendingQueueEdits: withoutQueueEdit(context.pendingQueueEdits, event.requestId),
        };
      },
      queuedPromptRemovalRequested: (context, event, enqueue) => {
        const originalIndex = context.queuedPrompts.findIndex(
          prompt => prompt.id === event.queueEntryId
        );
        const prompt = context.queuedPrompts[originalIndex];
        if (!prompt) return;
        const requestId = context.nextRequestId + 1;
        enqueueQueueRemoval(event.execute, enqueue, requestId);
        return {
          ...context,
          nextRequestId: requestId,
          pendingQueueEdits: {
            ...context.pendingQueueEdits,
            [requestId]: { kind: "remove", originalIndex, prompt },
          },
          queuedPrompts: context.queuedPrompts.filter(item => item.id !== event.queueEntryId),
        };
      },
      queuedPromptSteerRequested: (context, event, enqueue) => {
        if (context.busyInput.phase === "pending" || hasPendingQueuedSteer(context)) {
          return;
        }
        const originalIndex = context.queuedPrompts.findIndex(
          prompt => prompt.id === event.prompt.id
        );
        if (originalIndex < 0) return;
        const prompt = context.queuedPrompts[originalIndex];
        if (!prompt) return;
        const requestId = context.nextRequestId + 1;
        enqueueQueuedSteer(event.steer, event.cancel, enqueue, requestId);
        return {
          ...context,
          nextRequestId: requestId,
          pendingQueueEdits: {
            ...context.pendingQueueEdits,
            [requestId]: { kind: "steer", originalIndex, prompt },
          },
          queuedPrompts: context.queuedPrompts.filter(prompt => prompt.id !== event.prompt.id),
        };
      },
      queueScopeChanged: (context, event) => {
        if (context.queueScope === event.queueScope) return;
        return {
          ...context,
          pendingQueueEdits: {},
          queueScope: event.queueScope,
          queuedPrompts: [],
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
        if (context.resume.phase === "pending" || context.stop.phase === "pending") {
          return;
        }
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

function queuedPromptFromResult(result: unknown, text: string): QueuedPrompt | null {
  if (typeof result !== "object" || result === null || "outcome" in result) return null;
  const candidate = result as { queued?: unknown; queue_entry_id?: unknown };
  return candidate.queued === true && typeof candidate.queue_entry_id === "string"
    ? { id: candidate.queue_entry_id, text }
    : null;
}

function withoutQueueEdit(
  edits: Record<number, PendingQueueEdit>,
  requestId: number
): Record<number, PendingQueueEdit> {
  const next = { ...edits };
  delete next[requestId];
  return next;
}

function hasPendingQueuedSteer(context: SessionPageControlsState): boolean {
  return Object.values(context.pendingQueueEdits).some(edit => edit.kind === "steer");
}

function restoreQueuedPrompt(
  prompts: QueuedPrompt[],
  prompt: QueuedPrompt,
  originalIndex: number
): QueuedPrompt[] {
  if (prompts.some(candidate => candidate.id === prompt.id)) return prompts;
  const next = [...prompts];
  next.splice(Math.min(Math.max(originalIndex, 0), next.length), 0, prompt);
  return next;
}

function describeActionError(error: Error, fallback: string): string {
  return error.message.trim().length > 0 ? error.message : fallback;
}
