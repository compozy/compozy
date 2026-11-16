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
 * Checks still running per session, counted synchronously when each starts and settles, and
 * whether any of them overlapped: overlapping answers are not written, and the last check to
 * settle rereads the session's MCP state instead.
 */
interface SessionChecks {
  running: number;
  overlapped: boolean;
}

const sessionChecks = new WeakMap<QueryClient, Map<string, SessionChecks>>();

function checksFor(queryClient: QueryClient, key: string): SessionChecks {
  const sessions = sessionChecks.get(queryClient) ?? new Map<string, SessionChecks>();
  sessionChecks.set(queryClient, sessions);
  const checks = sessions.get(key) ?? { running: 0, overlapped: false };
  sessions.set(key, checks);
  return checks;
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
  return useMutation({
    mutationKey,
    mutationFn: (target: SessionMCPServerTarget) =>
      checkSessionMCPServer(workspaceId, sessionId, profile, target),
    onMutate: () => {
      const checks = checksFor(queryClient, overlapKey);
      checks.running += 1;
      if (checks.running > 1) checks.overlapped = true;
    },
    onSuccess: async data => {
      const checks = checksFor(queryClient, overlapKey);
      if (checks.running > 1 || checks.overlapped) return;
      await queryClient.cancelQueries({ queryKey, exact: true });
      queryClient.setQueryData(queryKey, data);
    },
    onSettled: () => {
      const checks = checksFor(queryClient, overlapKey);
      checks.running -= 1;
      if (checks.running > 0) return;
      const overlapped = checks.overlapped;
      sessionChecks.get(queryClient)?.delete(overlapKey);
      if (overlapped) void queryClient.invalidateQueries({ queryKey, exact: true });
      void invalidateSessionMCPSummary(queryClient, workspaceId, sessionId);
    },
  });
}
