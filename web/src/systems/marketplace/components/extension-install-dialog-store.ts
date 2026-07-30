import { createStoreLogic, type EnqueueObject } from "@xstate/store";

import type { ExtensionInstallRequest } from "../types";

/**
 * Two gates, in daemon order: the form builds a valid source-union request, then an unverified
 * archive needs an explicit consent decision before anything is installed.
 */
export type ExtensionInstallState =
  | { phase: "closed"; requestId: number }
  | { phase: "form"; requestId: number; error: string | null }
  | {
      phase: "consent";
      requestId: number;
      request: ExtensionInstallRequest;
      reason: string | null;
      error: string | null;
    }
  | {
      phase: "submitting";
      requestId: number;
      request: ExtensionInstallRequest;
      consented: boolean;
    };

type Events = {
  installClosed: {};
  installOpened: {};
  installRequested: {
    execute: (request: ExtensionInstallRequest) => Promise<void>;
    request: ExtensionInstallRequest;
  };
  installConsentRequested: { execute: (request: ExtensionInstallRequest) => Promise<void> };
  installFailed: { requestId: number; error: string; consentRequired: boolean };
  installSucceeded: { requestId: number };
};

type Enqueue = EnqueueObject<ExtensionInstallState, never, Events>;

const CONSENT_STATUS = 422;
const CONSENT_DIAGNOSTIC_CODE = "extension_checksum_unverified";

export function isExtensionConsentRequired(error: unknown): boolean {
  return (
    typeof error === "object" &&
    error !== null &&
    "status" in error &&
    "diagnosticCode" in error &&
    (error as { status?: number }).status === CONSENT_STATUS &&
    (error as { diagnosticCode?: string }).diagnosticCode === CONSENT_DIAGNOSTIC_CODE
  );
}

export function createExtensionInstallLogic() {
  return createStoreLogic<ExtensionInstallState, Events>({
    context: { phase: "closed", requestId: 0 },
    on: {
      installClosed: context =>
        context.phase === "submitting"
          ? undefined
          : { phase: "closed", requestId: context.requestId + 1 },
      installOpened: context => ({
        error: null,
        phase: "form",
        requestId: context.requestId + 1,
      }),
      installRequested: (context, event, enqueue) => {
        if (context.phase !== "form") return;
        if (event.request.allow_unverified === true) {
          return {
            error: null,
            phase: "consent",
            reason: null,
            request: event.request,
            requestId: context.requestId + 1,
          };
        }
        const requestId = context.requestId + 1;
        enqueueInstall(enqueue, event.request, requestId, event.execute);
        return { consented: false, phase: "submitting", request: event.request, requestId };
      },
      installConsentRequested: (context, event, enqueue) => {
        if (context.phase !== "consent") return;
        const requestId = context.requestId + 1;
        const request = { ...context.request, allow_unverified: true };
        enqueueInstall(enqueue, request, requestId, event.execute);
        return { consented: true, phase: "submitting", request, requestId };
      },
      installFailed: (context, event) => {
        if (context.phase !== "submitting" || context.requestId !== event.requestId) return;
        // The daemon asks for consent with 422; surface its reason on the consent gate instead of
        // silently retrying with `allow_unverified`.
        if (event.consentRequired && !context.consented) {
          return {
            error: null,
            phase: "consent",
            reason: event.error,
            request: context.request,
            requestId: context.requestId,
          };
        }
        return context.consented
          ? {
              error: event.error,
              phase: "consent",
              reason: null,
              request: context.request,
              requestId: context.requestId,
            }
          : { error: event.error, phase: "form", requestId: context.requestId };
      },
      installSucceeded: (context, event) => {
        if (context.phase !== "submitting" || context.requestId !== event.requestId) return;
        return { phase: "closed", requestId: context.requestId };
      },
    },
  });
}

function enqueueInstall(
  enqueue: Enqueue,
  request: ExtensionInstallRequest,
  requestId: number,
  execute: (request: ExtensionInstallRequest) => Promise<void>
) {
  enqueue.effect(async ({ trigger }) => {
    try {
      await execute(request);
      trigger.installSucceeded({ requestId });
    } catch (error) {
      trigger.installFailed({
        consentRequired: isExtensionConsentRequired(error),
        error: installErrorMessage(error),
        requestId,
      });
    }
  });
}

function installErrorMessage(error: unknown): string {
  return error instanceof Error && error.message.trim() !== ""
    ? error.message
    : "The extension could not be installed.";
}
