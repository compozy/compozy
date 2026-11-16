import { useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query";

import { checkSessionMCPServer, type SessionMCPServerTarget } from "../adapters/session-mcp-api";
import { sessionKeys } from "../lib/query-keys";
import { invalidateSessionByIdReads } from "../lib/session-query-invalidation";

interface SessionMCPCheckInput {
  workspaceId: string;
  sessionId: string;
  profile: string | undefined;
}

/** Rereads the session payload whose `mcp_summary` drives the closed composer control. */
export function invalidateSessionMCPSummary(
  queryClient: QueryClient,
  workspaceId: string,
  sessionId: string
): Promise<unknown> {
  return Promise.all([
    queryClient.invalidateQueries({
      queryKey: sessionKeys.detail(workspaceId, sessionId),
      exact: true,
    }),
    invalidateSessionByIdReads(queryClient, sessionId),
  ]);
}

/**
 * Check again for one of the session's servers. The daemon tries the server again without
 * restarting the runtime and answers with the session's refreshed MCP read, which replaces the
 * cached one so the row and its panel show the new state at once. A read already in flight is
 * cancelled first so its older answer cannot overwrite the check's, and the session payload is
 * reread so the closed control's summary follows.
 */
export function useCheckSessionMCPServer({
  workspaceId,
  sessionId,
  profile,
}: SessionMCPCheckInput) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (target: SessionMCPServerTarget) =>
      checkSessionMCPServer(workspaceId, sessionId, profile, target),
    onSuccess: async data => {
      const queryKey = sessionKeys.mcp(workspaceId, sessionId);
      await queryClient.cancelQueries({ queryKey, exact: true });
      queryClient.setQueryData(queryKey, data);
      void invalidateSessionMCPSummary(queryClient, workspaceId, sessionId);
    },
  });
}
