import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type {
  SessionTranscriptOutlineResponse,
  SessionTranscriptSearchQuery,
  SessionTranscriptSearchResponse,
} from "../types";
import { throwSessionRequestError } from "./session-api-errors";

/*
 * Navigation reads over the target session's materialized transcript (task_08):
 * a bounded case-insensitive literal search across the full retained history
 * and the operator-message outline. Both are session-scoped inside the
 * workspace through the same route binder as every session read.
 */

/** The daemon's default page; the route caps at 1000 and reports `truncated` past the bound. */
export const SESSION_TRANSCRIPT_SEARCH_LIMIT = 200;
/** The route refuses longer queries; the bar stops before the request leaves. */
export const SESSION_TRANSCRIPT_SEARCH_MAX_BYTES = 4_096;

export async function searchSessionTranscript(
  workspaceId: string,
  id: string,
  query: SessionTranscriptSearchQuery,
  signal?: AbortSignal
): Promise<SessionTranscriptSearchResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/transcript/search",
    {
      params: { path: { workspace_id: workspaceId, session_id: id }, query },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to search session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to search session "${id}"`);
}

export async function fetchSessionTranscriptOutline(
  workspaceId: string,
  id: string,
  signal?: AbortSignal
): Promise<SessionTranscriptOutlineResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/transcript/outline",
    {
      params: { path: { workspace_id: workspaceId, session_id: id } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to read the outline of session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to read the outline of session "${id}"`);
}
