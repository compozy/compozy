import { useEffect, useRef, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { supportApi } from "../adapters/support-api";
import { supportKeys } from "../lib/query-keys";
import type { SupportBundleDownloadResult, SupportBundleOperation } from "../types";

interface SupportBundleDownloadInput {
  includeStatus?: boolean;
  yes: true;
}

const SUPPORT_BUNDLE_POLL_INTERVAL_MS = 750;
const SUPPORT_BUNDLE_POLL_TIMEOUT_MS = 120_000;

function supportBundleFileName(operation: SupportBundleOperation): string {
  const fileName = operation.file_name?.trim();
  if (fileName) return fileName;
  return `agh-support-bundle-${operation.operation_id}.tar.gz`;
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
  const deadline = Date.now() + SUPPORT_BUNDLE_POLL_TIMEOUT_MS;
  while (Date.now() < deadline) {
    const operation = await supportApi.get(operationId, signal);
    if (operation.status === "completed") return operation;
    if (operation.status === "failed") {
      throw new Error(operation.failure_reason?.trim() || "Support bundle failed");
    }
    await delay(SUPPORT_BUNDLE_POLL_INTERVAL_MS, signal);
  }
  throw new Error("Support bundle did not complete within 2 minutes");
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
  controller: AbortController,
  onOperation: (operation: SupportBundleOperation) => void,
  onSettled: () => void
): Promise<SupportBundleDownloadResult> {
  const timeout = globalThis.setTimeout(
    () => controller.abort(new Error("Support bundle did not complete within 2 minutes")),
    SUPPORT_BUNDLE_POLL_TIMEOUT_MS
  );
  try {
    const created = await supportApi.create(
      { include_status: includeStatus, yes },
      controller.signal
    );
    onOperation(created);
    const completed = await waitForSupportBundle(created.operation_id, controller.signal);
    onOperation(completed);
    const blob = await supportApi.download(completed.operation_id, controller.signal);
    const fileName = supportBundleFileName(completed);
    downloadBlob(blob, fileName);
    return { operation: completed, fileName };
  } finally {
    globalThis.clearTimeout(timeout);
    onSettled();
  }
}

export function useSupportBundleDownload() {
  const queryClient = useQueryClient();
  const [operation, setOperation] = useState<SupportBundleOperation | null>(null);
  const activeRequestRef = useRef<AbortController | null>(null);

  useEffect(() => () => activeRequestRef.current?.abort(), []);

  const mutation = useMutation({
    mutationFn: (input: SupportBundleDownloadInput): Promise<SupportBundleDownloadResult> => {
      activeRequestRef.current?.abort();
      const controller = new AbortController();
      activeRequestRef.current = controller;
      return createAndDownloadSupportBundle(input, controller, setOperation, () => {
        if (activeRequestRef.current === controller) activeRequestRef.current = null;
      });
    },
    onSuccess: result => {
      queryClient.setQueryData(supportKeys.bundle(result.operation.operation_id), result.operation);
    },
  });

  const create = (input: SupportBundleDownloadInput) => mutation.mutateAsync(input);
  const reset = () => {
    activeRequestRef.current?.abort();
    activeRequestRef.current = null;
    setOperation(null);
    mutation.reset();
  };

  return {
    create,
    operation,
    isPending: mutation.isPending,
    error: mutation.error,
    reset,
  };
}
