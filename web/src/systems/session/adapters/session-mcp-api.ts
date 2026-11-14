import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";
import type { OperationResponse } from "@/lib/api-contract";

import { throwSessionRequestError } from "./session-api-errors";
import { sessionWorkspacePath } from "./session-workspace-path";

export type SessionMCPResponse = OperationResponse<"getSessionMCP", 200>;
export type SessionMCPServerRow = SessionMCPResponse["compozy"][number];
export type SessionRuntimeMCPRow = SessionMCPResponse["runtime_servers"]["servers"][number];
export type SessionMCPReloadRecord = NonNullable<SessionMCPResponse["last_reload"]>;

/**
 * Reads what MCP servers one session can use: CompozyOS rows in the session's own profile and
 * project, and the servers its coding agent loads on its own. Cached state only; never probes.
 */
export async function fetchSessionMCP(
  workspaceId: string,
  sessionId: string,
  profile: string | undefined,
  signal?: AbortSignal
): Promise<SessionMCPResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/mcp",
    {
      params: {
        path: { workspace_id: sessionWorkspacePath(workspaceId), session_id: sessionId },
        query: profile?.trim() ? { profile: profile.trim() } : undefined,
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to fetch MCP servers", sessionId);
  }
  return requireResponseData(data, response, "Failed to fetch MCP servers");
}
