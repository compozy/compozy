import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type { SessionGoalResponse } from "../types";
import { throwSessionRequestError } from "./session-api-errors";

export async function fetchSessionGoal(
  workspaceId: string,
  id: string,
  signal?: AbortSignal
): Promise<SessionGoalResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/goal",
    {
      params: { path: { workspace_id: workspaceId, session_id: id } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to fetch Goal for session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to fetch Goal for session "${id}"`);
}
