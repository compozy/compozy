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
  hasLocalRuntimeTail: boolean;
  liveTailEnabled: boolean;
  resetGeneration?: number;
}

export function useMergedSessionRuntimeTranscript({
  sessionId,
  workspaceId,
  eventSourceFactory,
  hasLocalRuntimeTail,
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
  const hasOptimisticRuntimeMessage = runtimeMessages.some(isOptimisticRuntimeMessage);
  const hasUnreconciledRuntimeTail = hasUnreconciledRuntimeMessages({
    transcriptMessages: transcript.messages,
    runtimeMessages,
  });

  const includeRuntimeTail =
    !conversationResetPending &&
    runtimeMessages.length > 0 &&
    hasUnreconciledRuntimeTail &&
    (runtimeIsRunning || hasLocalRuntimeTail || hasOptimisticRuntimeMessage);
  const messages = mergeSessionThreadReadModel({
    transcriptMessages: transcript.messages,
    runtimeMessages,
    includeRuntimeTail,
  });
  const durableMessageIds = new Set(transcript.messages.map(message => message.id));

  return {
    ...transcript,
    durableMessageIds,
    liveMessages: runtimeMessages,
    messages,
  };
}

function isOptimisticRuntimeMessage(message: ThreadMessage): boolean {
  return message.metadata?.isOptimistic === true;
}
