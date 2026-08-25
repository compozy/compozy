import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { answerSessionClarification } from "../adapters/session-clarification-api";
import { sessionClarificationsOptions } from "../lib/query-options";
import { sessionKeys } from "../lib/query-keys";
import type { AnswerClarificationBody, AnswerClarificationResult } from "../types";

/** Read the live pending clarification projection for one session. */
export function useSessionClarifications(
  workspaceId: string,
  id: string,
  options: { enabled?: boolean; refetchInterval?: number | false } = {}
) {
  return useQuery({
    ...sessionClarificationsOptions(workspaceId, id, options.enabled ?? true),
    refetchInterval: options.refetchInterval ?? false,
  });
}

export interface AnswerClarificationVariables {
  requestId: string;
  body: AnswerClarificationBody;
}

/**
 * Resolve one live clarification and reconcile the exact owner keys. Invalidation runs in
 * `onSettled` so the pending list and its durable transcript receipt re-read on both success and
 * terminal-gone (409/404) races.
 */
export function useAnswerSessionClarification(workspaceId: string, id: string) {
  const queryClient = useQueryClient();
  return useMutation<AnswerClarificationResult, Error, AnswerClarificationVariables>({
    mutationFn: ({ requestId, body }) =>
      answerSessionClarification(workspaceId, id, requestId, body),
    onSettled: () => {
      void queryClient.invalidateQueries({
        queryKey: sessionKeys.clarifications(workspaceId, id),
        exact: true,
      });
      void queryClient.invalidateQueries({
        queryKey: sessionKeys.transcript(workspaceId, id),
        exact: true,
      });
    },
  });
}
