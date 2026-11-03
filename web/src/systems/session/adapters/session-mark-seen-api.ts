import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type { SessionPayload } from "../types";
import { throwSessionRequestError } from "./session-api-errors";
import { sessionWorkspacePath } from "./session-workspace-path";

/**
 * Marks a session's Needs you as seen until it happens again. The daemon
 * refuses a session that still waits on an answer, a sign-in, or a stop.
 */
export async function markSessionSeen(
  workspaceId: string,
  id: string,
  profile: string,
  signal?: AbortSignal
): Promise<SessionPayload> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/mark-seen",
    {
      params: {
        path: { workspace_id: sessionWorkspacePath(workspaceId), session_id: id },
        query: { profile },
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to mark session "${id}" as seen`, id);
  }
  return requireResponseData(data, response, `Failed to mark session "${id}" as seen`).session;
}
