import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type { SessionPayload } from "../types";
import { throwSessionRequestError } from "./session-api-errors";

export async function archiveSession(
  workspaceId: string,
  id: string,
  signal?: AbortSignal
): Promise<SessionPayload> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/archive",
    {
      params: { path: { workspace_id: workspaceId, session_id: id } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to archive session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to archive session "${id}"`).session;
}

export async function unarchiveSession(
  workspaceId: string,
  id: string,
  signal?: AbortSignal
): Promise<SessionPayload> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/unarchive",
    {
      params: { path: { workspace_id: workspaceId, session_id: id } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to unarchive session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to unarchive session "${id}"`).session;
}
