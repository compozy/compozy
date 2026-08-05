import { useMutation, useQueryClient } from "@tanstack/react-query";

import { rewindSession, SessionApiError, type SessionRewindResult } from "../adapters/session-api";
import { sessionKeys } from "../lib/query-keys";
import { invalidateSessionLiveQueries } from "../lib/session-query-invalidation";
import { type SessionTranscriptData } from "../lib/session-transcript-query";
import { sessionTranscriptOptions } from "../lib/query-options";

export interface SessionRewindVariables {
  idempotencyKey: string;
  messageId: string;
  sessionId: string;
  signal: AbortSignal;
}

function rewindFence(
  queryClient: ReturnType<typeof useQueryClient>,
  workspaceId: string,
  sessionId: string
) {
  const transcript = queryClient.getQueryData<SessionTranscriptData>(
    sessionKeys.transcript(workspaceId, sessionId)
  );
  const head = transcript?.pages[0];
  if (!head) {
    throw new SessionApiError(
      `Cannot rewind session "${sessionId}" before its transcript has loaded`,
      409,
      sessionId
    );
  }

  return {
    expected_epoch: head.epoch,
    expected_generation: head.generation,
    expected_max_sequence: head.max_sequence,
  };
}

/**
 * Rewinds one workspace-scoped session from a durable transcript message. The
 * returned transcript fence is daemon-owned; the client never derives it from
 * display order or a local runtime tail.
 */
export function useSessionRewind(workspaceId: string) {
  const queryClient = useQueryClient();

  return useMutation<SessionRewindResult, Error, SessionRewindVariables>({
    mutationKey: sessionKeys.rewindConversation(workspaceId),
    mutationFn: ({ idempotencyKey, messageId, sessionId, signal }) =>
      rewindSession(
        workspaceId,
        sessionId,
        {
          ...rewindFence(queryClient, workspaceId, sessionId),
          idempotency_key: idempotencyKey,
          message_id: messageId,
        },
        signal
      ),
    onSuccess: async (result, { sessionId }) => {
      queryClient.setQueryData(sessionKeys.detail(workspaceId, sessionId), result.session);
      queryClient.removeQueries({
        exact: true,
        queryKey: sessionKeys.transcript(workspaceId, sessionId),
      });
      await Promise.all([
        invalidateSessionLiveQueries(queryClient, workspaceId, sessionId),
        queryClient.fetchInfiniteQuery(sessionTranscriptOptions(workspaceId, sessionId)),
      ]);
    },
  });
}
