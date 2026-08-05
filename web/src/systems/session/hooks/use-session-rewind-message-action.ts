import { useAuiState } from "@assistant-ui/react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { useRef, useState } from "react";
import { toast } from "sonner";

import { useSessionComposerPrefill } from "@/components/assistant-ui/hooks/use-session-composer-prefill";
import { createClientId } from "@/lib/client-id";
import { sessionTranscriptOptions } from "../lib/query-options";
import type { SessionTranscriptData } from "../lib/session-transcript-query";
import { useSessionRewind } from "./use-session-rewind";
import { useSessionRuntimeRenderContext } from "./use-session-runtime-render-context";

interface CurrentMessageState {
  id?: string;
  role?: string;
}

function messageIsDurable(
  transcript: SessionTranscriptData | undefined,
  messageId: string
): boolean {
  return (
    transcript?.pages.some(page => page.entries.some(entry => entry.message.id === messageId)) ??
    false
  );
}

export function useSessionRewindMessageAction() {
  const context = useSessionRuntimeRenderContext();
  const composerPrefill = useSessionComposerPrefill();
  const currentMessage = useAuiState(state => state.message as CurrentMessageState);
  const composerText = useAuiState(state => state.composer.text);
  const isThreadRunning = useAuiState(state => state.thread.isRunning);
  const [open, updateOpen] = useState(false);
  const idempotencyKeyRef = useRef<string | null>(null);
  const resetRuntime = context?.resetRuntime;
  const workspaceId = context?.workspaceId ?? "";
  const sessionId = context?.sessionId ?? "";
  const transcript = useInfiniteQuery(sessionTranscriptOptions(workspaceId, sessionId)).data;
  const rewind = useSessionRewind(workspaceId);
  const messageId = typeof currentMessage.id === "string" ? currentMessage.id : "";
  const busy = isThreadRunning || rewind.isPending || (context?.rewindBlocked ?? true);
  const available =
    Boolean(context && resetRuntime && composerPrefill) &&
    currentMessage.role === "user" &&
    messageId.length > 0 &&
    messageIsDurable(transcript, messageId);

  const setOpen = (nextOpen: boolean) => {
    if (!nextOpen) idempotencyKeyRef.current = null;
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
    const idempotencyKey = idempotencyKeyRef.current ?? createClientId();
    idempotencyKeyRef.current = idempotencyKey;
    try {
      const result = await rewind.mutateAsync({ idempotencyKey, messageId, sessionId });
      resetRuntime();
      composerPrefill(result.rewind.draft_text);
      setOpen(false);
    } catch {
      toast.error("Couldn't rewind this session. Refresh the conversation and try again.");
    }
  };

  return { available, busy, confirm, isPending: rewind.isPending, open, setOpen, trigger };
}
