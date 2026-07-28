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
}

interface RuntimeTailState {
  transcriptMessages: readonly ThreadMessage[] | null;
  runtimeMessages: readonly ThreadMessage[] | null;
  hasLocalRuntimeTail: boolean;
}

export function useMergedSessionRuntimeTranscript({
  sessionId,
  workspaceId,
  eventSourceFactory,
}: UseMergedSessionRuntimeTranscriptOptions) {
  const runtimeMessages = useAuiState(state => state.thread.messages);
  const runtimeIsRunning = useAuiState(state => state.thread.isRunning);
  const transcript = useSessionLiveTail({ sessionId, workspaceId, eventSourceFactory });
  const conversationResetPending =
    useIsMutating({
      exact: true,
      mutationKey: sessionKeys.clearConversation(workspaceId),
      predicate: mutation => mutation.state.variables === sessionId,
    }) > 0;
  const hasOptimisticRuntimeMessage = runtimeMessages.some(isOptimisticRuntimeMessage);
  const [runtimeTailState, setRuntimeTailState] = useState<RuntimeTailState>({
    transcriptMessages: null,
    runtimeMessages: null,
    hasLocalRuntimeTail: false,
  });

  const transcriptMessagesChanged = runtimeTailState.transcriptMessages !== transcript.messages;
  const runtimeMessagesChanged = runtimeTailState.runtimeMessages !== runtimeMessages;
  const previousRuntimeCount = runtimeTailState.runtimeMessages?.length ?? 0;
  const hasPreviousRuntimeSnapshot = runtimeTailState.runtimeMessages !== null;
  const hasUnreconciledRuntimeTail = hasUnreconciledRuntimeMessages({
    transcriptMessages: transcript.messages,
    runtimeMessages,
  });
  // Local/fast SSE can finish in the same turn it starts, so we never observe
  // isRunning/isOptimistic mid-flight. Latch the tail when the runtime grows past
  // the durable transcript; stable durable identities clear each matching row.
  const runtimeGrewWithUnreconciledMessages =
    hasPreviousRuntimeSnapshot &&
    runtimeMessagesChanged &&
    runtimeMessages.length > previousRuntimeCount &&
    hasUnreconciledRuntimeTail;
  let hasLocalRuntimeTail = runtimeTailState.hasLocalRuntimeTail;
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
    runtimeTailState.hasLocalRuntimeTail !== hasLocalRuntimeTail
      ? { transcriptMessages: transcript.messages, runtimeMessages, hasLocalRuntimeTail }
      : runtimeTailState;
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
    messages,
  };
}

function isOptimisticRuntimeMessage(message: ThreadMessage): boolean {
  return message.metadata?.isOptimistic === true;
}
