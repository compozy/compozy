import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { taskRunResultPageOptions } from "../lib/query-options";
import type { TaskRunResultPage } from "../types";

const TASK_RUN_RESULT_PAGE_BYTES = 16 * 1024;

type CopyState = "idle" | "copying" | "copied" | "error";

export interface TaskRunResultController {
  canGoNext: boolean;
  canGoPrevious: boolean;
  copyState: CopyState;
  errorMessage: string | null;
  isLoading: boolean;
  offset: number;
  onCopy: () => Promise<void>;
  onNextPage: () => void;
  onOpenChange: (open: boolean) => void;
  onPreviousPage: () => void;
  onRetry: () => void;
  open: boolean;
  page: TaskRunResultPage | undefined;
  pageText: string;
}

interface UseTaskRunResultOptions {
  resultBytes: number;
  resultRef: string;
  runId: string;
  workspaceId: string;
}

interface IdentityState<T> {
  identity: string;
  value: T;
}

export function useTaskRunResult({
  resultBytes,
  resultRef,
  runId,
  workspaceId,
}: UseTaskRunResultOptions): TaskRunResultController {
  const identity = `${workspaceId}\u0000${runId}\u0000${resultRef}`;
  const queryClient = useQueryClient();
  const [disclosure, setDisclosure] = useState<IdentityState<boolean>>({
    identity,
    value: false,
  });
  const [cursor, setCursor] = useState<IdentityState<number>>({ identity, value: 0 });
  const [copy, setCopy] = useState<IdentityState<CopyState>>({ identity, value: "idle" });
  const open = disclosure.identity === identity && disclosure.value;
  const offset = cursor.identity === identity ? cursor.value : 0;
  const copyState = copy.identity === identity ? copy.value : "idle";
  const query = useQuery(
    taskRunResultPageOptions(
      workspaceId,
      runId,
      resultRef,
      offset,
      TASK_RUN_RESULT_PAGE_BYTES,
      open
    )
  );

  const onOpenChange = (nextOpen: boolean) => {
    setDisclosure({ identity, value: nextOpen });
    if (nextOpen && cursor.identity !== identity) {
      setCursor({ identity, value: 0 });
    }
  };

  const onNextPage = () => {
    const nextOffset = query.data?.next_offset;
    if (typeof nextOffset === "number" && nextOffset > offset) {
      setCursor({ identity, value: nextOffset });
    }
  };

  const onPreviousPage = () => {
    setCursor({ identity, value: Math.max(0, offset - TASK_RUN_RESULT_PAGE_BYTES) });
  };

  const onRetry = () => {
    void query.refetch();
  };

  const onCopy = async () => {
    if (copyState === "copying") return;
    setCopy({ identity, value: "copying" });
    try {
      const text = await readCompleteResult({
        expectedBytes: resultBytes,
        queryClient,
        resultRef,
        runId,
        workspaceId,
      });
      await navigator.clipboard.writeText(text);
      setCopy({ identity, value: "copied" });
    } catch {
      setCopy({ identity, value: "error" });
    }
  };

  return {
    canGoNext: Boolean(query.data && !query.data.eof && query.data.next_offset),
    canGoPrevious: offset > 0,
    copyState,
    errorMessage: query.isError ? "Couldn't load result. Try again." : null,
    isLoading: query.isPending && open,
    offset,
    onCopy,
    onNextPage,
    onOpenChange,
    onPreviousPage,
    onRetry,
    open,
    page: query.data,
    pageText: query.data ? decodePage(query.data.data_base64) : "",
  };
}

interface CompleteResultRequest {
  expectedBytes: number;
  queryClient: ReturnType<typeof useQueryClient>;
  resultRef: string;
  runId: string;
  workspaceId: string;
}

async function readCompleteResult({
  expectedBytes,
  queryClient,
  resultRef,
  runId,
  workspaceId,
}: CompleteResultRequest): Promise<string> {
  const chunks: Uint8Array[] = [];
  let offset = 0;
  let totalBytes: number | null = null;
  while (totalBytes === null || offset < totalBytes) {
    const page = await queryClient.fetchQuery(
      taskRunResultPageOptions(workspaceId, runId, resultRef, offset, TASK_RUN_RESULT_PAGE_BYTES)
    );
    if (totalBytes === null) totalBytes = page.total_bytes;
    if (page.total_bytes !== totalBytes || page.offset !== offset) {
      throw new Error("Task run result changed while copying");
    }
    const chunk = decodeBase64(page.data_base64);
    if (chunk.byteLength !== page.bytes) {
      throw new Error("Task run result page size mismatch");
    }
    chunks.push(chunk);
    if (page.eof) {
      offset += chunk.byteLength;
      break;
    }
    if (typeof page.next_offset !== "number" || page.next_offset <= offset) {
      throw new Error("Task run result page did not advance");
    }
    offset = page.next_offset;
  }
  if (totalBytes === null || offset !== totalBytes || totalBytes !== expectedBytes) {
    throw new Error("Task run result byte count mismatch");
  }
  const combined = new Uint8Array(totalBytes);
  let destination = 0;
  for (const chunk of chunks) {
    combined.set(chunk, destination);
    destination += chunk.byteLength;
  }
  return new TextDecoder().decode(combined);
}

function decodePage(dataBase64: string): string {
  return new TextDecoder().decode(decodeBase64(dataBase64));
}

function decodeBase64(dataBase64: string): Uint8Array {
  const binary = globalThis.atob(dataBase64);
  return Uint8Array.from(binary, character => character.charCodeAt(0));
}
