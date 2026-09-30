import { useAuiState } from "@assistant-ui/react";
import { useIsMutating } from "@tanstack/react-query";

import { sessionKeys } from "../lib/query-keys";
import { useSessionRuntimeRenderContext } from "./use-session-runtime-render-context";

interface CurrentMessageState {
  id?: string;
  role?: string;
}

/**
 * The gates "Rewind to here" and "Fork from here" share. `durable`: a user
 * message the daemon-owned transcript holds (never an optimistic tail or an
 * assistant message). `busy`: a moving transcript has no fence to act on —
 * the thread runs (locally, or per the daemon for a turn started elsewhere), a
 * rewind is pending anywhere in this workspace, or rewind is blocked by an open
 * decision.
 */
export function useSessionMessageActionGate() {
  const context = useSessionRuntimeRenderContext();
  const currentMessage = useAuiState(state => state.message as CurrentMessageState);
  const isThreadRunning = useAuiState(state => state.thread.isRunning);
  const workspaceId = context?.workspaceId ?? "";
  const rewindsPending = useIsMutating({
    mutationKey: sessionKeys.rewindConversation(workspaceId),
  });
  const messageId = typeof currentMessage.id === "string" ? currentMessage.id : "";
  const durable =
    context !== null &&
    currentMessage.role === "user" &&
    messageId.length > 0 &&
    context.durableMessageIds.has(messageId);
  const busy =
    isThreadRunning ||
    context?.sessionRunning === true ||
    rewindsPending > 0 ||
    (context?.rewindBlocked ?? true);

  return { context, messageId, durable, busy };
}
