import { createStoreLogic, type EnqueueObject } from "@xstate/store";

import { notifyUser } from "@/lib/user-feedback";

import type { PermissionDecision } from "../adapters/session-api";
import type { PermissionRequest } from "../types";

type PermissionDecisionPhase = "idle" | "submitting" | "resolved";

interface SessionPermissionDecisionState {
  phase: PermissionDecisionPhase;
  requestId: number;
  permissionKey: string;
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
  resolutionAccepted: {};
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
    context: { phase: "idle", permissionKey: "", requestId: 0 },
    on: {
      decisionSubmitted: (context, event, enqueue) => {
        if (context.permissionKey === event.permissionKey && context.phase !== "idle") return;
        const requestId = context.requestId + 1;
        enqueueDecision(enqueue, event, requestId);
        return {
          phase: "submitting",
          permissionKey: event.permissionKey,
          requestId,
        };
      },
      decisionResolved: (context, event, enqueue) => {
        if (context.phase !== "submitting" || context.requestId !== event.requestId) return;
        enqueue.emit.resolutionAccepted({});
        return { ...context, phase: "resolved" };
      },
      decisionFailed: (context, event, enqueue) => {
        if (context.phase !== "submitting" || context.requestId !== event.requestId) return;
        enqueue.effect(() => {
          console.error("Failed to submit permission decision", event.error);
          notifyUser({
            message: "Failed to send permission response. The agent may continue waiting.",
            tone: "error",
          });
        });
        return { ...context, phase: "idle" };
      },
    },
  });
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
