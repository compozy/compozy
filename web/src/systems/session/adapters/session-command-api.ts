import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type { SessionCommandsResponse } from "../types";
import { throwSessionRequestError } from "./session-api-errors";

/** Reads the daemon-owned command projection for one workspace-fenced session. */
export async function fetchSessionCommands(
  workspaceId: string,
  sessionId: string,
  signal?: AbortSignal
): Promise<SessionCommandsResponse> {
  const workspace = workspaceId.trim();
  const session = sessionId.trim();
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/commands",
    {
      params: { path: { workspace_id: workspace, session_id: session } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(
      response,
      error,
      `Failed to fetch commands for session "${session}"`,
      session
    );
  }
  return requireResponseData(data, response, `Failed to fetch commands for session "${session}"`);
}
