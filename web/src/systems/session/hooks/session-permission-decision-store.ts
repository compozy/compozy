import { createStoreLogic, type EnqueueObject } from "@xstate/store";

import { notifyUser } from "@/lib/user-feedback";

import { SessionApiError, type PermissionDecision } from "../adapters/session-api";
import type { PermissionRequest } from "../types";

type PermissionDecisionPhase = "idle" | "submitting" | "resolved" | "unavailable";

interface SessionPermissionDecisionState {
  phase: PermissionDecisionPhase;
  requestId: number;
  permissionKey: string;
  decision: PermissionDecision | null;
}

type SessionPermissionDecisionEventPayloadMap = {
  decisionSubmitted: {
    decision: PermissionDecision;
    permission: PermissionRequest;
    permissionKey: string;
    sessionId: string;
    workspaceId: string;
    execute: (input: PermissionDecisionInput) => Promise<void>;
  };
  decisionResolved: { requestId: number };
  decisionFailed: { error: Error; requestId: number };
};

type SessionPermissionDecisionEmittedPayloadMap = {
  resolutionAccepted: { decision: PermissionDecision };
  permissionUnavailable: {};
};

interface PermissionDecisionInput {
  decision: PermissionDecision;
  permission: PermissionRequest;
  sessionId: string;
  workspaceId: string;
}

type SessionPermissionDecisionEnqueue = EnqueueObject<
  SessionPermissionDecisionState,
  never,
  SessionPermissionDecisionEventPayloadMap
>;

export function createSessionPermissionDecisionLogic() {
  return createStoreLogic<
    SessionPermissionDecisionState,
    SessionPermissionDecisionEventPayloadMap,
    SessionPermissionDecisionEmittedPayloadMap
  >({
    context: { decision: null, phase: "idle", permissionKey: "", requestId: 0 },
    on: {
      decisionSubmitted: (context, event, enqueue) => {
        if (context.permissionKey === event.permissionKey && context.phase !== "idle") return;
        const requestId = context.requestId + 1;
        enqueueDecision(enqueue, event, requestId);
        return {
          decision: event.decision,
          phase: "submitting",
          permissionKey: event.permissionKey,
          requestId,
        };
      },
      decisionResolved: (context, event, enqueue) => {
        if (context.phase !== "submitting" || context.requestId !== event.requestId) return;
        if (context.decision) enqueue.emit.resolutionAccepted({ decision: context.decision });
        return { ...context, phase: "resolved" };
      },
      decisionFailed: (context, event, enqueue) => {
        if (context.phase !== "submitting" || context.requestId !== event.requestId) return;
        const terminal = terminalPermissionFailure(event.error);
        if (terminal) enqueue.emit.permissionUnavailable({});
        enqueue.effect(() => {
          console.error("Failed to submit permission decision", event.error);
          notifyUser({
            message: permissionDecisionFailureMessage(event.error),
            tone: "error",
          });
        });
        return { ...context, decision: null, phase: terminal ? "unavailable" : "idle" };
      },
    },
  });
}

function permissionDecisionFailureMessage(error: Error): string {
  const terminal = terminalPermissionFailure(error);
  if (terminal === "expired")
    return "This permission request expired. The action was not approved.";
  if (terminal === "canceled")
    return "This permission request was canceled. The action was not approved.";
  return "Failed to send permission response. The agent may continue waiting.";
}

function terminalPermissionFailure(error: Error): "expired" | "canceled" | null {
  if (error instanceof SessionApiError && error.status === 409) {
    if (error.message.includes("permission request expired")) {
      return "expired";
    }
    if (error.message.includes("permission request canceled")) {
      return "canceled";
    }
  }
  return null;
}

function enqueueDecision(
  enqueue: SessionPermissionDecisionEnqueue,
  event: SessionPermissionDecisionEventPayloadMap["decisionSubmitted"],
  requestId: number
) {
  enqueue.effect(async ({ trigger }) => {
    try {
      await event.execute(event);
      trigger.decisionResolved({ requestId });
    } catch (error) {
      trigger.decisionFailed({
        error: normalizePermissionError(error),
        requestId,
      });
    }
  });
}

function normalizePermissionError(error: unknown): Error {
  return error instanceof Error ? error : new Error("Failed to submit permission decision");
}
