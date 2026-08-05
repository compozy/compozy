import { useAuiState } from "@assistant-ui/react";
import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";

import { useSessionComposerPrefill } from "@/components/assistant-ui/hooks/use-session-composer-prefill";
import { createClientId } from "@/lib/client-id";
import { useSessionRewind } from "./use-session-rewind";
import { useSessionRuntimeRenderContext } from "./use-session-runtime-render-context";

interface CurrentMessageState {
  id?: string;
  role?: string;
}

export function useSessionRewindMessageAction() {
  const context = useSessionRuntimeRenderContext();
  const composerPrefill = useSessionComposerPrefill();
  const currentMessage = useAuiState(state => state.message as CurrentMessageState);
  const composerText = useAuiState(state => state.composer.text);
  const isThreadRunning = useAuiState(state => state.thread.isRunning);
  const [open, updateOpen] = useState(false);
  const idempotencyKeyRef = useRef<string | null>(null);
  const abortControllerRef = useRef<AbortController | null>(null);
  const resetRuntime = context?.resetRuntime;
  const workspaceId = context?.workspaceId ?? "";
  const sessionId = context?.sessionId ?? "";
  const rewind = useSessionRewind(workspaceId);
  const messageId = typeof currentMessage.id === "string" ? currentMessage.id : "";
  const busy = isThreadRunning || rewind.isPending || (context?.rewindBlocked ?? true);
  const available =
    Boolean(context && resetRuntime && composerPrefill) &&
    currentMessage.role === "user" &&
    messageId.length > 0 &&
    (context?.durableMessageIds.has(messageId) ?? false);

  useEffect(
    () => () => {
      abortControllerRef.current?.abort();
    },
    []
  );

  const setOpen = (nextOpen: boolean) => {
    if (!nextOpen) {
      abortControllerRef.current?.abort();
      abortControllerRef.current = null;
      idempotencyKeyRef.current = null;
    }
    updateOpen(nextOpen);
  };

  const trigger = () => {
    if (busy) return;
    if (composerText.trim().length > 0) {
      toast.warning("Clear the draft before rewinding.");
      return;
    }
    idempotencyKeyRef.current = createClientId();
    updateOpen(true);
  };

  const confirm = async () => {
    if (!resetRuntime || !composerPrefill) return;
    if (busy) {
      setOpen(false);
      return;
    }
    if (composerText.trim().length > 0) {
      toast.warning("Clear the draft before rewinding.");
      return;
    }
    const idempotencyKey = idempotencyKeyRef.current ?? createClientId();
    idempotencyKeyRef.current = idempotencyKey;
    abortControllerRef.current?.abort();
    const controller = new AbortController();
    abortControllerRef.current = controller;
    let result;
    try {
      result = await rewind.mutateAsync({
        idempotencyKey,
        messageId,
        sessionId,
        signal: controller.signal,
      });
    } catch {
      if (abortControllerRef.current === controller) {
        abortControllerRef.current = null;
      }
      if (!controller.signal.aborted) {
        toast.error("Couldn't rewind this session. Refresh the conversation and try again.");
      }
      return;
    }
    if (controller.signal.aborted) {
      if (abortControllerRef.current === controller) {
        abortControllerRef.current = null;
      }
      return;
    }
    resetRuntime();
    composerPrefill(result.rewind.draft_text);
    setOpen(false);
  };

  return { available, busy, confirm, isPending: rewind.isPending, open, setOpen, trigger };
}
