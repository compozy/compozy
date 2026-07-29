import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type { SessionOwnerResponse } from "../types";
import { throwSessionRequestError } from "./session-api-errors";

/**
 * Workspace-agnostic ownership projection (`{session_id, workspace_id, workspace_name}`).
 * It is the only session read the deep-link flows may issue before the operator confirms a
 * workspace switch — no agent, provider, transcript, or activity fields exist on this surface.
 * A 404 surfaces as `SessionNotFoundError`, which keeps the not-found route outcome unchanged.
 */
export async function fetchSessionOwner(
  sessionId: string,
  signal?: AbortSignal
): Promise<SessionOwnerResponse> {
  const { data, error, response } = await apiClient.GET("/api/sessions/{session_id}/owner", {
    params: { path: { session_id: sessionId } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(
      response,
      error,
      `Failed to fetch session owner "${sessionId}"`,
      sessionId
    );
  }
  return requireResponseData(data, response, `Failed to fetch session owner "${sessionId}"`);
}
