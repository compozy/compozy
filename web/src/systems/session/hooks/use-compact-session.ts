import { useMutation, useQueryClient } from "@tanstack/react-query";

import { compactSession } from "../adapters/session-compaction-api";
import {
  invalidateSessionLiveQueries,
  invalidateSessionUsageQueries,
} from "../lib/session-query-invalidation";
import type { SessionCompactionReceipt } from "../types";

/**
 * Requests the agent's own compaction for one workspace-scoped session
 * (experimental). Resolving means the daemon accepted the request, nothing
 * more: there is no optimistic state, and the outcome is observed through the
 * Compaction timeline item and the usage markers. Settling rereads the session
 * and the context rail's usage reads, so a refusal also refreshes a stale view
 * of the session. The request is not tied to the component's lifetime, so a
 * rail that closes mid-flight never leaves the outcome unknown.
 */
export function useCompactSession(workspaceId: string, sessionId: string) {
  const queryClient = useQueryClient();

  return useMutation<SessionCompactionReceipt, Error>({
    mutationFn: () => compactSession(workspaceId, sessionId),
    onSettled: () =>
      Promise.all([
        invalidateSessionLiveQueries(queryClient, workspaceId, sessionId),
        invalidateSessionUsageQueries(queryClient, workspaceId, sessionId),
      ]),
  });
}
