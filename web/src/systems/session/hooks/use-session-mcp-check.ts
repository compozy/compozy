import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  checkSessionMCPServer,
  type SessionMCPResponse,
  type SessionMCPServerTarget,
} from "../adapters/session-mcp-api";
import { sessionKeys } from "../lib/query-keys";

interface SessionMCPCheckInput {
  workspaceId: string;
  sessionId: string;
  profile: string | undefined;
}

/** The row a check answered for, found by its full identity (names repeat across owners). */
export function sessionMCPCheckedRow(data: SessionMCPResponse, target: SessionMCPServerTarget) {
  return data.compozy_servers.find(row => row.name === target.name && row.owner === target.owner);
}

/**
 * Check again for one of the session's servers. The daemon tries the server again without
 * restarting the runtime and answers with the session's refreshed MCP read, which replaces the
 * cached one so the row and its panel show the new state at once.
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
    onSuccess: data => {
      queryClient.setQueryData(sessionKeys.mcp(workspaceId, sessionId), data);
    },
  });
}
