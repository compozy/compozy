import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type { SessionPayload } from "../types";
import { throwSessionRequestError } from "./session-api-errors";
import { sessionWorkspacePath } from "./session-workspace-path";

/**
 * Archives a session using its explicit owner profile. `stop` lets the daemon
 * stop a live session through the normal stop path before archiving it.
 */
export async function archiveSession(
  workspaceId: string,
  id: string,
  profile: string,
  signal?: AbortSignal,
  stop = false
): Promise<SessionPayload> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/archive",
    {
      params: {
        path: { workspace_id: sessionWorkspacePath(workspaceId), session_id: id },
        query: { profile },
      },
      ...(stop ? { body: { stop: true } } : {}),
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to archive session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to archive session "${id}"`).session;
}

/** Restores an archived session within its owning workspace and profile. */
export async function unarchiveSession(
  workspaceId: string,
  id: string,
  profile: string,
  signal?: AbortSignal
): Promise<SessionPayload> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/unarchive",
    {
      params: {
        path: { workspace_id: sessionWorkspacePath(workspaceId), session_id: id },
        query: { profile },
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to unarchive session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to unarchive session "${id}"`).session;
}
