import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type { SessionPayload } from "../types";
import { throwSessionRequestError } from "./session-api-errors";
import { sessionWorkspacePath } from "./session-workspace-path";

/**
 * Pins (`pinned`) or unpins a session using its explicit owner profile. A pin is organization
 * only: the daemon keeps the session's folder, manual position and activity order, so an unpin
 * returns it to the place it held. Pinning a pinned session and unpinning an unpinned one change
 * nothing.
 */
export async function setSessionPinned(
  workspaceId: string,
  id: string,
  profile: string,
  pinned: boolean,
  signal?: AbortSignal
): Promise<SessionPayload> {
  const options = {
    params: {
      path: { workspace_id: sessionWorkspacePath(workspaceId), session_id: id },
      query: { profile },
    },
    signal,
  };
  const path = "/api/workspaces/{workspace_id}/sessions/{session_id}/pin";
  const { data, error, response } = pinned
    ? await apiClient.POST(path, options)
    : await apiClient.DELETE(path, options);
  const failure = pinned ? `Failed to pin session "${id}"` : `Failed to unpin session "${id}"`;
  if (apiRequestFailed(response, error)) throwSessionRequestError(response, error, failure, id);
  return requireResponseData(data, response, failure).session;
}
