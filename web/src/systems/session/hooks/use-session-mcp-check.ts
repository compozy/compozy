import { hashKey, useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query";

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
 * Sessions whose overlapping checks answered while another was still running: those answers were
 * not written, and the last check to finish rereads the session's MCP state instead.
 */
const overlappedChecks = new WeakMap<QueryClient, Set<string>>();

function markOverlapped(queryClient: QueryClient, key: string) {
  const marked = overlappedChecks.get(queryClient) ?? new Set<string>();
  marked.add(key);
  overlappedChecks.set(queryClient, marked);
}

function takeOverlapped(queryClient: QueryClient, key: string): boolean {
  return overlappedChecks.get(queryClient)?.delete(key) ?? false;
}

/**
 * Check again for one of the session's servers. The daemon tries the server again without
 * restarting the runtime and answers with the session's refreshed MCP read, which replaces the
 * cached one so the row and its panel show the new state at once. A read already in flight is
 * cancelled first so its older answer cannot overwrite the check's, and the session payload is
 * reread so the closed control's summary follows. Checks of different servers can overlap, and
 * each answer is a whole-session snapshot taken at its own time: while checks overlap none of
 * their answers is written, and the last one to finish rereads the session's MCP state, so an
 * older snapshot never replaces a newer one.
 */
export function useCheckSessionMCPServer({
  workspaceId,
  sessionId,
  profile,
}: SessionMCPCheckInput) {
  const queryClient = useQueryClient();
  const queryKey = sessionKeys.mcp(workspaceId, sessionId);
  const mutationKey = [...queryKey, "check"];
  const overlapKey = hashKey(queryKey);
  // This check is still pending while its own callbacks run, so it counts itself.
  const othersRunning = () => queryClient.isMutating({ mutationKey, exact: true }) > 1;
  return useMutation({
    mutationKey,
    mutationFn: (target: SessionMCPServerTarget) =>
      checkSessionMCPServer(workspaceId, sessionId, profile, target),
    onSuccess: async data => {
      if (othersRunning() || overlappedChecks.get(queryClient)?.has(overlapKey)) return;
      await queryClient.cancelQueries({ queryKey, exact: true });
      queryClient.setQueryData(queryKey, data);
    },
    onSettled: () => {
      if (othersRunning()) {
        markOverlapped(queryClient, overlapKey);
        return;
      }
      if (takeOverlapped(queryClient, overlapKey)) {
        void queryClient.invalidateQueries({ queryKey, exact: true });
      }
      void invalidateSessionMCPSummary(queryClient, workspaceId, sessionId);
    },
  });
}
