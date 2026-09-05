import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type { SessionInteractionRecord, SessionInteractionStatus } from "../types";
import { throwSessionRequestError } from "./session-api-errors";

export interface FetchSessionInteractionsOptions {
  /** Daemon status filter; the daemon defaults to the actionable set (`pending` + `orphaned`). */
  status?: SessionInteractionStatus;
  signal?: AbortSignal;
}

/**
 * Restart-durable interaction records for one session. The session payload only
 * embeds actionable rows, so a settled decision — expired by a daemon restart,
 * timed out, or resolved — is read here by explicit status. The daemon owns the
 * status vocabulary and rejects unknown values with 400.
 */
export async function fetchSessionInteractions(
  workspaceId: string,
  id: string,
  options: FetchSessionInteractionsOptions = {}
): Promise<SessionInteractionRecord[]> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/interactions",
    {
      params: {
        path: { workspace_id: workspaceId, session_id: id },
        query: options.status === undefined ? {} : { status: options.status },
      },
      signal: options.signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(
      response,
      error,
      `Failed to fetch interactions for session "${id}"`,
      id
    );
  }
  return requireResponseData(data, response, `Failed to fetch interactions for session "${id}"`)
    .interactions;
}
