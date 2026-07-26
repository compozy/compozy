import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { answerSessionClarification } from "../adapters/session-clarification-api";
import { sessionClarificationsOptions } from "../lib/query-options";
import { sessionKeys } from "../lib/query-keys";
import type { AnswerClarificationBody, AnswerClarificationResult } from "../types";

/** Read the live pending clarification projection for one session. */
export function useSessionClarifications(workspaceId: string, id: string) {
  return useQuery(sessionClarificationsOptions(workspaceId, id));
}

export interface AnswerClarificationVariables {
  requestId: string;
  body: AnswerClarificationBody;
}

/**
 * Resolve one live clarification and reconcile the exact owner key. Invalidation runs in `onSettled`
 * so the pending list re-reads on both success and the terminal-gone (409/404) races — the
 * `clarifications` key is a child of `detail`, so it needs an explicit exact invalidation that the
 * broader session-surface refresh does not reach.
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
    },
  });
}
