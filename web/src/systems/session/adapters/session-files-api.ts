import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";
import type { OperationQuery, OperationResponse } from "@/lib/api-contract";

import { throwSessionRequestError } from "./session-api-errors";

export type SessionFilesResponse = OperationResponse<"searchSessionFiles", 200>;
export type SessionFileEntry = SessionFilesResponse["entries"][number];
export type SessionFilesQuery = NonNullable<OperationQuery<"searchSessionFiles">>;

/**
 * Searches files and folders under the session's working directory (the
 * directory its agent runs in). Ranking, the 25,000-entry index cap and
 * `.gitignore` handling are the daemon's (ADR-004); the client only asks.
 */
export async function searchSessionFiles(
  workspaceId: string,
  sessionId: string,
  query: SessionFilesQuery,
  signal?: AbortSignal
): Promise<SessionFilesResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/files",
    {
      params: { path: { workspace_id: workspaceId, session_id: sessionId }, query },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Couldn't search files", sessionId);
  }
  return requireResponseData(data, response, "Couldn't search files");
}
