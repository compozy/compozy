import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type { SessionCompactionReceipt } from "../types";
import { throwSessionRequestError } from "./session-api-errors";

/**
 * Asks the agent to compact its own context through the command it advertises
 * (experimental). The resolved 202 receipt means "accepted" only; progress and
 * outcome arrive as the session's Compaction item and usage markers. Refusals
 * (`session_busy`, `compaction_unsupported`) throw a `SessionApiError` carrying
 * the daemon's `code` and message.
 */
export async function compactSession(
  workspaceId: string,
  id: string,
  signal?: AbortSignal
): Promise<SessionCompactionReceipt> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/compact",
    {
      params: { path: { workspace_id: workspaceId, session_id: id } },
      body: {},
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to compact session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to compact session "${id}"`);
}
