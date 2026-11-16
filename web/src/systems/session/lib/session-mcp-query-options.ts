import { queryOptions } from "@tanstack/react-query";

import { fetchSessionMCP } from "../adapters/session-mcp-api";
import { sessionKeys } from "./query-keys";

/** How long a session MCP read stays fresh before opening the popover rereads it. */
export const SESSION_MCP_STALE_TIME_MS = 5_000;

/**
 * The session's MCP read model, keyed under the session's detail so a detail sweep reaches it.
 * `session.mcp_changed` and `session.mcp_reloaded` on the session stream invalidate it, and the
 * composer reads it while its popover is open or the session has a pending change. The profile is part of the request, not the key: one
 * session belongs to exactly one profile. A User session reports an empty `workspaceId`; the
 * adapter reads it under `@user`, so only the session ID gates the read.
 */
export function sessionMCPOptions(workspaceId: string, sessionId: string, profile?: string) {
  return queryOptions({
    queryKey: sessionKeys.mcp(workspaceId, sessionId),
    queryFn: ({ signal }) => fetchSessionMCP(workspaceId, sessionId, profile, signal),
    enabled: sessionId !== "",
    staleTime: SESSION_MCP_STALE_TIME_MS,
    refetchOnWindowFocus: false,
    retry: false,
  });
}
