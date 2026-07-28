import { createStoreLogic } from "@xstate/store";

import { notifyUser } from "@/lib/user-feedback";
import type { BridgeVerifyResponse, BridgeWebhookRegistrationResponse } from "@/systems/bridges";
import { summarizeBridgeCheckStatus } from "@/systems/bridges/lib/bridge-setup";

export type BridgeSetupFlowPhase = "idle" | "verifying" | "registering" | "recovery";

export interface BridgeSetupFlowInput {
  bridgeId: string | null;
  registrationFingerprint: string;
  verificationFingerprint: string;
}

export interface BridgeSetupFlowState extends BridgeSetupFlowInput {
  attempt: number;
  pendingEvidenceCurrent: boolean;
  phase: BridgeSetupFlowPhase;
  registration: BridgeWebhookRegistrationResponse | null;
  verification: BridgeVerifyResponse | null;
}

export type BridgeRegistrationExecution = (
  bridgeId: string
) => Promise<BridgeWebhookRegistrationResponse>;
export type BridgeVerificationExecution = (bridgeId: string) => Promise<BridgeVerifyResponse>;

type BridgeSetupEvents = {
  factsChanged: BridgeSetupFlowInput;
  evidenceCleared: {};
  verificationCleared: {};
  verificationRequested: {
    bridgeId: string;
    bridgeName: string;
    verify: BridgeVerificationExecution;
  };
  registrationRequested: {
    bridgeId: string;
    bridgeName: string;
    register: BridgeRegistrationExecution;
  };
  verificationResolved: {
    attempt: number;
    bridgeId: string;
    bridgeName: string;
    result: BridgeVerifyResponse;
  };
  registrationResolved: {
    attempt: number;
    bridgeId: string;
    bridgeName: string;
    result: BridgeWebhookRegistrationResponse;
  };
  operationFailed: { attempt: number; bridgeId: string; error: string };
};

interface BridgeSetupTrigger {
  verificationResolved: (event: {
    attempt: number;
    bridgeId: string;
    bridgeName: string;
    result: BridgeVerifyResponse;
  }) => void;
  registrationResolved: (event: {
    attempt: number;
    bridgeId: string;
    bridgeName: string;
    result: BridgeWebhookRegistrationResponse;
  }) => void;
  operationFailed: (event: { attempt: number; bridgeId: string; error: string }) => void;
}

export function createBridgeSetupFlowLogic() {
  return createStoreLogic<
    BridgeSetupFlowState,
    BridgeSetupEvents,
    Record<never, never>,
    BridgeSetupFlowInput
  >({
    context: input => ({
      ...input,
      attempt: 0,
      pendingEvidenceCurrent: true,
      phase: "idle",
      registration: null,
      verification: null,
    }),
    on: {
      factsChanged: (context, event) => {
        if (context.bridgeId !== event.bridgeId) return;
        const registrationChanged =
          context.registrationFingerprint !== event.registrationFingerprint;
        const verificationChanged =
          context.verificationFingerprint !== event.verificationFingerprint;
        if (!registrationChanged && !verificationChanged) return;
        const pendingEvidenceCurrent =
          context.phase === "registering"
            ? context.pendingEvidenceCurrent && !registrationChanged
            : context.phase === "verifying"
              ? context.pendingEvidenceCurrent && !verificationChanged
              : true;
        return {
          ...context,
          ...event,
          pendingEvidenceCurrent,
          registration: registrationChanged ? null : context.registration,
          verification: verificationChanged ? null : context.verification,
        };
      },
      evidenceCleared: context => {
        const pending = context.phase === "registering" || context.phase === "verifying";
        return {
          ...context,
          pendingEvidenceCurrent: pending ? false : context.pendingEvidenceCurrent,
          registration: null,
          verification: null,
        };
      },
      verificationCleared: context => {
        const pending = context.phase === "verifying";
        return {
          ...context,
          pendingEvidenceCurrent: pending ? false : context.pendingEvidenceCurrent,
          verification: null,
        };
      },
      verificationRequested: (context, event, enqueue) => {
        if (
          context.bridgeId !== event.bridgeId ||
          (context.phase !== "idle" && context.phase !== "recovery")
        )
          return;
        const attempt = context.attempt + 1;
        enqueue.effect(async ({ trigger }) => {
          await executeVerification(
            event.verify,
            trigger,
            event.bridgeId,
            event.bridgeName,
            attempt
          );
        });
        return {
          ...context,
          attempt,
          pendingEvidenceCurrent: true,
          phase: "verifying",
          verification: null,
        };
      },
      registrationRequested: (context, event, enqueue) => {
        if (
          context.bridgeId !== event.bridgeId ||
          (context.phase !== "idle" && context.phase !== "recovery")
        )
          return;
        const attempt = context.attempt + 1;
        enqueue.effect(async ({ trigger }) => {
          await executeRegistration(
            event.register,
            trigger,
            event.bridgeId,
            event.bridgeName,
            attempt
          );
        });
        return {
          ...context,
          attempt,
          pendingEvidenceCurrent: true,
          phase: "registering",
          registration: null,
          verification: null,
        };
      },
      verificationResolved: (context, event, enqueue) => {
        if (!isActiveAttempt(context, event) || context.phase !== "verifying") return;
        if (!context.pendingEvidenceCurrent) {
          return { ...context, phase: "idle", pendingEvidenceCurrent: true };
        }
        if (event.result.bridge_instance_id !== context.bridgeId) {
          return recover(
            context,
            "The verification response did not match the selected bridge.",
            enqueue
          );
        }
        enqueue.effect(() => notifyVerificationResult(event.result, event.bridgeName));
        return {
          ...context,
          pendingEvidenceCurrent: true,
          phase: "idle",
          verification: event.result,
        };
      },
      registrationResolved: (context, event, enqueue) => {
        if (!isActiveAttempt(context, event) || context.phase !== "registering") return;
        if (!context.pendingEvidenceCurrent) {
          return { ...context, phase: "idle", pendingEvidenceCurrent: true };
        }
        if (event.result.bridge_instance_id !== context.bridgeId) {
          return recover(
            context,
            "The registration response did not match the selected bridge.",
            enqueue
          );
        }
        enqueue.effect(() => notifyRegistrationResult(event.result, event.bridgeName));
        return {
          ...context,
          pendingEvidenceCurrent: true,
          phase: "idle",
          registration: event.result,
        };
      },
      operationFailed: (context, event, enqueue) => {
        if (!isActiveAttempt(context, event)) return;
        if (!context.pendingEvidenceCurrent) {
          return { ...context, phase: "idle", pendingEvidenceCurrent: true };
        }
        return recover(context, event.error, enqueue);
      },
    },
  });
}

async function executeVerification(
  verify: BridgeVerificationExecution,
  trigger: BridgeSetupTrigger,
  bridgeId: string,
  bridgeName: string,
  attempt: number
): Promise<void> {
  try {
    const result = await verify(bridgeId);
    trigger.verificationResolved({
      attempt,
      bridgeId,
      bridgeName,
      result,
    });
  } catch (error) {
    trigger.operationFailed({
      attempt,
      bridgeId,
      error: errorMessage(error, "Failed to verify bridge"),
    });
  }
}

async function executeRegistration(
  register: BridgeRegistrationExecution,
  trigger: BridgeSetupTrigger,
  bridgeId: string,
  bridgeName: string,
  attempt: number
): Promise<void> {
  try {
    const result = await register(bridgeId);
    trigger.registrationResolved({
      attempt,
      bridgeId,
      bridgeName,
      result,
    });
  } catch (error) {
    trigger.operationFailed({
      attempt,
      bridgeId,
      error: errorMessage(error, "Failed to register bridge webhook"),
    });
  }
}

function isActiveAttempt(
  context: BridgeSetupFlowState,
  event: { attempt: number; bridgeId: string }
): boolean {
  return context.attempt === event.attempt && context.bridgeId === event.bridgeId;
}

function recover(
  context: BridgeSetupFlowState,
  message: string,
  enqueue: { effect: (effect: () => void) => void }
) {
  enqueue.effect(() => notifyUser({ message, tone: "error" }));
  return { ...context, pendingEvidenceCurrent: true, phase: "recovery" as const };
}

function notifyVerificationResult(result: BridgeVerifyResponse, bridgeName: string): void {
  const status = summarizeBridgeCheckStatus(result.checks);
  switch (status) {
    case "pass":
      notifyUser({ message: `Verified bridge ${bridgeName}.`, tone: "success" });
      return;
    case "warn":
      notifyUser({
        message: `Verification completed with warnings for ${bridgeName}.`,
        tone: "warning",
      });
      return;
    case "fail":
      notifyUser({
        message: `Verification found setup issues for ${bridgeName}.`,
        tone: "error",
      });
      return;
    case "skipped":
      notifyUser({
        message: `Verification checks were skipped for ${bridgeName}.`,
        tone: "info",
      });
      return;
    case null:
      notifyUser({
        message: `Verification returned no checks for ${bridgeName}.`,
        tone: "warning",
      });
  }
}

function notifyRegistrationResult(
  result: BridgeWebhookRegistrationResponse,
  bridgeName: string
): void {
  switch (result.status) {
    case "pass":
      notifyUser({ message: `Registered the webhook for ${bridgeName}.`, tone: "success" });
      return;
    case "warn":
      notifyUser({
        message: result.remediation || `Webhook registration is uncertain for ${bridgeName}.`,
        tone: "warning",
      });
      return;
    case "fail":
      notifyUser({
        message: result.remediation || "Telegram webhook registration failed.",
        tone: "error",
      });
      return;
    case "skipped":
      notifyUser({
        message: result.remediation || `Webhook registration was skipped for ${bridgeName}.`,
        tone: "info",
      });
  }
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim() !== "" ? error.message : fallback;
}
