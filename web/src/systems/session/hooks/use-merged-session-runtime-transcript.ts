import { useState } from "react";
import { type ThreadMessage, useAuiState } from "@assistant-ui/react";
import { useIsMutating } from "@tanstack/react-query";

import {
  hasUnreconciledRuntimeMessages,
  mergeSessionThreadReadModel,
} from "../lib/session-thread-read-model";
import { sessionKeys } from "../lib/query-keys";
import { useSessionLiveTail, type SessionStreamEventSourceFactory } from "./use-session-live-tail";

interface UseMergedSessionRuntimeTranscriptOptions {
  sessionId: string;
  workspaceId: string;
  eventSourceFactory?: SessionStreamEventSourceFactory;
  liveTailEnabled: boolean;
  resetGeneration?: number;
}

interface RuntimeTailState {
  hasLocalRuntimeTail: boolean;
  runtimeMessages: readonly ThreadMessage[] | null;
  sessionKey: string;
  transcriptMessages: readonly ThreadMessage[] | null;
}

export function useMergedSessionRuntimeTranscript({
  sessionId,
  workspaceId,
  eventSourceFactory,
  liveTailEnabled,
  resetGeneration,
}: UseMergedSessionRuntimeTranscriptOptions) {
  const runtimeMessages = useAuiState(state => state.thread.messages);
  const runtimeIsRunning = useAuiState(state => state.thread.isRunning);
  const transcript = useSessionLiveTail({
    enabled: liveTailEnabled,
    eventSourceFactory,
    resetGeneration,
    sessionId,
    workspaceId,
  });
  const clearConversationPending =
    useIsMutating({
      exact: true,
      mutationKey: sessionKeys.clearConversation(workspaceId),
      predicate: mutation => mutation.state.variables === sessionId,
    }) > 0;
  const rewindConversationPending =
    useIsMutating({
      exact: true,
      mutationKey: sessionKeys.rewindConversation(workspaceId),
      predicate: mutation => {
        const variables = mutation.state.variables;
        return (
          typeof variables === "object" &&
          variables !== null &&
          "sessionId" in variables &&
          variables.sessionId === sessionId
        );
      },
    }) > 0;
  const conversationResetPending = clearConversationPending || rewindConversationPending;
  const sessionKey = `${workspaceId}\u0000${sessionId}`;
  const hasOptimisticRuntimeMessage = runtimeMessages.some(isOptimisticRuntimeMessage);
  const [runtimeTailState, setRuntimeTailState] = useState<RuntimeTailState>(() => ({
    hasLocalRuntimeTail: false,
    runtimeMessages: null,
    sessionKey,
    transcriptMessages: null,
  }));
  const scopedTailState =
    runtimeTailState.sessionKey === sessionKey
      ? runtimeTailState
      : {
          hasLocalRuntimeTail: false,
          runtimeMessages: null,
          sessionKey,
          transcriptMessages: null,
        };
  const transcriptMessagesChanged = scopedTailState.transcriptMessages !== transcript.messages;
  const runtimeMessagesChanged = scopedTailState.runtimeMessages !== runtimeMessages;
  const previousRuntimeCount = scopedTailState.runtimeMessages?.length ?? 0;
  const hasUnreconciledRuntimeTail = hasUnreconciledRuntimeMessages({
    transcriptMessages: transcript.messages,
    runtimeMessages,
  });
  // Local/fast SSE can finish in the same turn it starts, so isRunning and the
  // optimistic marker may never be observed. Growth beyond the durable transcript
  // is therefore a load-bearing latch; durable identities clear it after reconcile.
  const runtimeGrewWithUnreconciledMessages =
    scopedTailState.runtimeMessages !== null &&
    runtimeMessagesChanged &&
    runtimeMessages.length > previousRuntimeCount &&
    hasUnreconciledRuntimeTail;
  let hasLocalRuntimeTail = scopedTailState.hasLocalRuntimeTail;
  if (
    runtimeIsRunning ||
    (runtimeMessagesChanged && hasOptimisticRuntimeMessage) ||
    runtimeGrewWithUnreconciledMessages
  ) {
    hasLocalRuntimeTail = true;
  } else if (hasLocalRuntimeTail) {
    hasLocalRuntimeTail = hasUnreconciledRuntimeTail;
  }
  if (runtimeMessages.length === 0) {
    hasLocalRuntimeTail = false;
  }
  const nextRuntimeTailState =
    transcriptMessagesChanged ||
    runtimeMessagesChanged ||
    scopedTailState.hasLocalRuntimeTail !== hasLocalRuntimeTail
      ? {
          hasLocalRuntimeTail,
          runtimeMessages,
          sessionKey,
          transcriptMessages: transcript.messages,
        }
      : scopedTailState;
  if (nextRuntimeTailState !== runtimeTailState) {
    setRuntimeTailState(nextRuntimeTailState);
  }

  const includeRuntimeTail =
    !conversationResetPending &&
    runtimeMessages.length > 0 &&
    (runtimeIsRunning ||
      nextRuntimeTailState.hasLocalRuntimeTail ||
      (runtimeMessagesChanged && hasOptimisticRuntimeMessage));
  const messages = mergeSessionThreadReadModel({
    transcriptMessages: transcript.messages,
    runtimeMessages,
    includeRuntimeTail,
  });

  return {
    ...transcript,
    liveMessages: runtimeMessages,
    messages,
  };
}

function isOptimisticRuntimeMessage(message: ThreadMessage): boolean {
  return message.metadata?.isOptimistic === true;
}
