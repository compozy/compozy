import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type { components } from "@compozy/client";
import { throwSessionRequestError } from "./session-api-errors";
import { sessionWorkspacePath } from "./session-workspace-path";

export type MoveSessionToProfileResponse = components["schemas"]["MoveSessionToProfileResponse"];

/**
 * Copies a stopped session into `targetProfile` and archives the original. The
 * daemon releases the session's worktree and checks its branch out again for the
 * target profile, so the request can take a few seconds.
 */
export async function moveSessionToProfile(
  workspaceId: string,
  id: string,
  profile: string,
  targetProfile: string,
  signal?: AbortSignal
): Promise<MoveSessionToProfileResponse> {
  const fallback = `Failed to move session "${id}" to ${targetProfile}`;
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/move-to-profile",
    {
      params: {
        path: { workspace_id: sessionWorkspacePath(workspaceId), session_id: id },
        query: { profile },
      },
      body: { target_profile: targetProfile },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, fallback, id);
  }
  return requireResponseData(data, response, fallback);
}
