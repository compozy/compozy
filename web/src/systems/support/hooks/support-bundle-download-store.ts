import { createStoreLogic, type TriggerObject } from "@xstate/store";

import type { SupportBundleDownloadResult, SupportBundleOperation } from "../types";

export interface SupportBundleDownloadInput {
  includeStatus?: boolean;
  yes: true;
}

export type SupportBundleDownloadExecution = (
  input: SupportBundleDownloadInput,
  signal: AbortSignal,
  onOperation: (operation: SupportBundleOperation) => void
) => Promise<SupportBundleDownloadResult>;

export interface SupportBundleDownloadOptions {
  timeoutMs?: number;
}

export interface SupportBundleDownloadState {
  error: Error | null;
  operation: SupportBundleOperation | null;
  phase: "idle" | "pending" | "failed" | "completed";
  requestId: number;
}

type SupportBundleDownloadEventPayloadMap = {
  downloadRequested: {
    execute: SupportBundleDownloadExecution;
    input: SupportBundleDownloadInput;
  };
  operationObserved: { operation: SupportBundleOperation; requestId: number };
  downloadCompleted: { requestId: number; result: SupportBundleDownloadResult };
  downloadFailed: { error: Error; requestId: number };
  requestCanceled: { error: Error; requestId: number };
  lifecycleReset: {};
};

export type SupportBundleRequestSettlement =
  | { outcome: "completed"; requestId: number; result: SupportBundleDownloadResult }
  | { outcome: "failed"; error: Error; requestId: number };

type SupportBundleDownloadEmittedPayloadMap = {
  requestAccepted: { requestId: number };
  requestSettled: SupportBundleRequestSettlement;
};

interface ActiveRequest {
  canceled: boolean;
  controller: AbortController;
  requestId: number;
  timeoutError: Error | null;
  timeoutId: ReturnType<typeof globalThis.setTimeout> | undefined;
}

type SupportBundleDownloadTrigger = TriggerObject<SupportBundleDownloadEventPayloadMap>;

const DEFAULT_OPERATION_TIMEOUT_MS = 120_000;

export function createSupportBundleDownloadLogic(options: SupportBundleDownloadOptions = {}) {
  const activeRequests = new WeakMap<SupportBundleDownloadTrigger, ActiveRequest>();
  const timeoutMs = options.timeoutMs ?? DEFAULT_OPERATION_TIMEOUT_MS;

  return createStoreLogic<
    SupportBundleDownloadState,
    SupportBundleDownloadEventPayloadMap,
    SupportBundleDownloadEmittedPayloadMap
  >({
    context: { error: null, operation: null, phase: "idle", requestId: 0 },
    on: {
      downloadRequested: (context, event, enqueue) => {
        const requestId = context.requestId + 1;
        enqueue.emit.requestAccepted({ requestId });
        enqueue.effect(({ trigger }) => {
          startRequest(event.execute, activeRequests, trigger, event.input, requestId, timeoutMs);
        });
        return { error: null, operation: null, phase: "pending", requestId };
      },
      operationObserved: (context, event) => {
        if (context.phase !== "pending" || context.requestId !== event.requestId) return;
        return { ...context, operation: event.operation };
      },
      downloadCompleted: (context, event, enqueue) => {
        enqueue.emit.requestSettled({
          outcome: "completed",
          requestId: event.requestId,
          result: event.result,
        });
        if (context.phase !== "pending" || context.requestId !== event.requestId) return;
        return { ...context, operation: event.result.operation, phase: "completed" };
      },
      downloadFailed: (context, event, enqueue) => {
        enqueue.emit.requestSettled({
          outcome: "failed",
          error: event.error,
          requestId: event.requestId,
        });
        if (context.phase !== "pending" || context.requestId !== event.requestId) return;
        return { ...context, error: event.error, phase: "failed" };
      },
      requestCanceled: (_context, event, enqueue) => {
        enqueue.emit.requestSettled({
          outcome: "failed",
          error: event.error,
          requestId: event.requestId,
        });
      },
      lifecycleReset: (context, _event, enqueue) => {
        enqueue.effect(({ trigger }) => {
          cancelActiveRequest(activeRequests, trigger, "Support bundle request was canceled");
        });
        return { error: null, operation: null, phase: "idle", requestId: context.requestId + 1 };
      },
    },
  });
}

function startRequest(
  execute: SupportBundleDownloadExecution,
  activeRequests: WeakMap<SupportBundleDownloadTrigger, ActiveRequest>,
  trigger: SupportBundleDownloadTrigger,
  input: SupportBundleDownloadInput,
  requestId: number,
  timeoutMs: number
) {
  cancelActiveRequest(activeRequests, trigger, "Support bundle request was replaced");
  const controller = new AbortController();
  const activeRequest: ActiveRequest = {
    canceled: false,
    controller,
    requestId,
    timeoutError: null,
    timeoutId: undefined,
  };
  activeRequests.set(trigger, activeRequest);

  void executeRequest(execute, activeRequests, trigger, activeRequest, input, timeoutMs);
}

async function executeRequest(
  execute: SupportBundleDownloadExecution,
  activeRequests: WeakMap<SupportBundleDownloadTrigger, ActiveRequest>,
  trigger: SupportBundleDownloadTrigger,
  activeRequest: ActiveRequest,
  input: SupportBundleDownloadInput,
  timeoutMs: number
) {
  const timeout = new Promise<never>((_resolve, reject) => {
    activeRequest.timeoutId = globalThis.setTimeout(() => {
      const error = new Error("Support bundle did not complete within 2 minutes");
      activeRequest.timeoutError = error;
      activeRequest.controller.abort(error);
      reject(error);
    }, timeoutMs);
  });

  try {
    const result = await Promise.race([
      execute(input, activeRequest.controller.signal, operation => {
        trigger.operationObserved({ operation, requestId: activeRequest.requestId });
      }),
      timeout,
    ]);
    if (!activeRequest.canceled) {
      trigger.downloadCompleted({ requestId: activeRequest.requestId, result });
    }
  } catch (error) {
    if (!activeRequest.canceled) {
      trigger.downloadFailed({
        requestId: activeRequest.requestId,
        error: activeRequest.timeoutError ?? normalizeDownloadError(error),
      });
    }
  } finally {
    clearRequestTimer(activeRequest);
    if (activeRequests.get(trigger) === activeRequest) {
      activeRequests.delete(trigger);
    }
  }
}

function cancelActiveRequest(
  activeRequests: WeakMap<SupportBundleDownloadTrigger, ActiveRequest>,
  trigger: SupportBundleDownloadTrigger,
  message: string
) {
  const request = activeRequests.get(trigger);
  if (!request) return;

  request.canceled = true;
  clearRequestTimer(request);
  activeRequests.delete(trigger);
  const error = new DOMException(message, "AbortError");
  request.controller.abort(error);
  trigger.requestCanceled({ error, requestId: request.requestId });
}

function clearRequestTimer(request: ActiveRequest) {
  if (request.timeoutId === undefined) return;
  globalThis.clearTimeout(request.timeoutId);
  request.timeoutId = undefined;
}

function normalizeDownloadError(error: unknown): Error {
  return error instanceof Error ? error : new Error("Failed to download support bundle");
}
