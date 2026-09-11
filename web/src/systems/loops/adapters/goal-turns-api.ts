import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type { GoalTurnFilter, GoalTurnPage } from "../types";
import { LoopsApiError } from "./loops-api-errors";

export async function listGoalTurns(
  workspaceId: string,
  runId: string,
  filters: GoalTurnFilter = {},
  signal?: AbortSignal
): Promise<GoalTurnPage> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/turns",
    {
      params: {
        path: { workspace_id: workspaceId, run_id: runId },
        query: filters,
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new LoopsApiError(
      defaultApiErrorMessage(`Failed to fetch Goal turns for run "${runId}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to fetch Goal turns for run "${runId}"`);
}
