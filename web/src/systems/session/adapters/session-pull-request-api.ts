import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type { SessionPullRequestRefreshResult } from "../lib/session-pull-request";
import { throwSessionRequestError } from "./session-api-errors";

/**
 * Refreshes the session's PR facts synchronously (10 s, daemon-side). A forge
 * failure with cached facts still answers 200 with `lookup.state = "error"`; a
 * failure without them is a 502 `forge_unavailable` carrying the cause.
 */
export async function refreshSessionPullRequest(
  sessionId: string,
  profile: string | undefined,
  signal?: AbortSignal
): Promise<SessionPullRequestRefreshResult> {
  const fallback = `Failed to refresh the pull request of session "${sessionId}"`;
  const { data, error, response } = await apiClient.POST(
    "/api/sessions/{session_id}/pull-request/refresh",
    {
      params: { path: { session_id: sessionId }, ...(profile ? { query: { profile } } : {}) },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, fallback, sessionId);
  }
  return requireResponseData(data, response, fallback);
}
