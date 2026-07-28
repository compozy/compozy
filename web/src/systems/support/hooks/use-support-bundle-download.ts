import { useSelector, useStore } from "@xstate/store-react";
import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { awaitStoreRequest } from "@/lib/store-request";

import { supportApi } from "../adapters/support-api";
import { supportKeys } from "../lib/query-keys";
import type { SupportBundleDownloadResult, SupportBundleOperation } from "../types";
import {
  createSupportBundleDownloadLogic,
  type SupportBundleDownloadInput,
  type SupportBundleRequestSettlement,
} from "./support-bundle-download-store";

const supportBundleDownloadLogic = createSupportBundleDownloadLogic();

const SUPPORT_BUNDLE_POLL_INTERVAL_MS = 750;

function supportBundleFileName(operation: SupportBundleOperation): string {
  const fileName = operation.file_name?.trim();
  if (fileName) return fileName;
  return `compozy-support-bundle-${operation.operation_id}.tar.gz`;
}

function abortError(): DOMException {
  return new DOMException("Support bundle request was canceled", "AbortError");
}

function delay(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal.aborted) {
      reject(signal.reason ?? abortError());
      return;
    }
    const onAbort = () => {
      globalThis.clearTimeout(timer);
      reject(signal.reason ?? abortError());
    };
    const timer = globalThis.setTimeout(() => {
      signal.removeEventListener("abort", onAbort);
      resolve();
    }, ms);
    signal.addEventListener("abort", onAbort, { once: true });
  });
}

async function waitForSupportBundle(
  operationId: string,
  signal: AbortSignal
): Promise<SupportBundleOperation> {
  while (true) {
    const operation = await supportApi.get(operationId, signal);
    if (operation.status === "completed") return operation;
    if (operation.status === "failed") {
      throw new Error(operation.failure_reason?.trim() || "Support bundle failed");
    }
    await delay(SUPPORT_BUNDLE_POLL_INTERVAL_MS, signal);
  }
}

function downloadBlob(blob: Blob, fileName: string) {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = fileName;
  document.body.append(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}

async function createAndDownloadSupportBundle(
  { includeStatus = true, yes }: SupportBundleDownloadInput,
  signal: AbortSignal,
  onOperation: (operation: SupportBundleOperation) => void
): Promise<SupportBundleDownloadResult> {
  const created = await supportApi.create({ include_status: includeStatus, yes }, signal);
  onOperation(created);
  const completed = await waitForSupportBundle(created.operation_id, signal);
  onOperation(completed);
  const blob = await supportApi.download(completed.operation_id, signal);
  const fileName = supportBundleFileName(completed);
  downloadBlob(blob, fileName);
  return { operation: completed, fileName };
}

export function useSupportBundleDownload() {
  const queryClient = useQueryClient();
  const store = useStore(supportBundleDownloadLogic);
  const state = useSelector(store, snapshot => snapshot.context);

  useEffect(() => () => store.trigger.lifecycleReset(), [store]);

  return {
    create: (input: SupportBundleDownloadInput) =>
      awaitStoreRequest<
        { requestId: number },
        SupportBundleRequestSettlement,
        SupportBundleDownloadResult
      >({
        notAcceptedMessage: "Support bundle request was not accepted",
        request: () =>
          store.trigger.downloadRequested({ execute: createAndDownloadSupportBundle, input }),
        resolveSettlement: event => {
          if (event.outcome === "completed") {
            queryClient.setQueryData(
              supportKeys.bundle(event.result.operation.operation_id),
              event.result.operation
            );
            return event.result;
          }
          throw event.error;
        },
        subscribeAccepted: listener => store.on("requestAccepted", listener),
        subscribeSettled: listener => store.on("requestSettled", listener),
      }),
    operation: state.operation,
    isPending: state.phase === "pending",
    error: state.error,
    reset: () => store.trigger.lifecycleReset(),
  };
}
