import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type { SessionPayload } from "../types";
import { throwSessionRequestError } from "./session-api-errors";

/** Archives a stopped session using its explicit owner profile. */
export async function archiveSession(
  workspaceId: string,
  id: string,
  profile: string,
  signal?: AbortSignal
): Promise<SessionPayload> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/archive",
    {
      params: { path: { workspace_id: workspaceId, session_id: id }, query: { profile } },
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
      params: { path: { workspace_id: workspaceId, session_id: id }, query: { profile } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to unarchive session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to unarchive session "${id}"`).session;
}
