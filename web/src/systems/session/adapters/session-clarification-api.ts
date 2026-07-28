import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type {
  AnswerClarificationBody,
  AnswerClarificationResult,
  ClarificationPending,
} from "../types";
import { SessionApiError, throwSessionRequestError } from "./session-api-errors";

/**
 * A clarification answer was rejected because the request already left the live broker map
 * (answered elsewhere, timed out, canceled, or the session stopped). Distinct from a retryable
 * failure so the card can hide instead of offering a doomed retry.
 */
export class ClarificationNotAnswerableError extends SessionApiError {
  constructor(id: string, requestId: string) {
    super(`Clarification "${requestId}" is no longer answerable`, 409, id);
    this.name = "ClarificationNotAnswerableError";
  }
}

/**
 * The live pending clarification projection for one session — the exact authority for pending
 * truth (zero or one request). Transcript `clarify` events are only wake signals into this read.
 */
export async function fetchSessionClarifications(
  workspaceId: string,
  id: string,
  signal?: AbortSignal
): Promise<ClarificationPending[]> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/clarifications",
    {
      params: { path: { workspace_id: workspaceId, session_id: id } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(
      response,
      error,
      `Failed to fetch clarifications for session "${id}"`,
      id
    );
  }
  return requireResponseData(data, response, `Failed to fetch clarifications for session "${id}"`)
    .clarifications;
}

/**
 * Resolve one live clarification. The body carries exactly one of a zero-based `choice_index` or a
 * non-empty `text`; the daemon returns the frozen `{choice, text, fallback}` structured answer.
 */
export async function answerSessionClarification(
  workspaceId: string,
  id: string,
  requestId: string,
  body: AnswerClarificationBody,
  signal?: AbortSignal
): Promise<AnswerClarificationResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/clarifications/{request_id}/answer",
    {
      params: {
        path: { workspace_id: workspaceId, session_id: id, request_id: requestId },
      },
      body,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    if (response.status === 409) {
      throw new ClarificationNotAnswerableError(id, requestId);
    }
    throwSessionRequestError(
      response,
      error,
      `Failed to answer clarification "${requestId}" for session "${id}"`,
      id
    );
  }
  return requireResponseData(
    data,
    response,
    `Failed to answer clarification "${requestId}" for session "${id}"`
  );
}
