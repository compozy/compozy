import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";
import type { OperationRequestBody, OperationResponse } from "@/lib/api-contract";

import { throwSessionRequestError } from "./session-api-errors";

/** The daemon's measurement of what a continue or fork of one session would carry. */
export type SessionDerivePreview = OperationResponse<"previewSessionDerive", 200>;
export type ContinueSessionRequest = OperationRequestBody<"continueSession">;
/** `201` for a new child, `200` for a replayed idempotency key — one shape. */
export type SessionDeriveResult = OperationResponse<"continueSession", 201>;
export type SessionDerivedOutcome = SessionDeriveResult["derived"];

export interface SessionDerivePreviewParams {
  /** Durable user message to cut through (fork from here); absent for the whole session. */
  messageId?: string;
}

export async function fetchSessionDerivePreview(
  workspaceId: string,
  sessionId: string,
  params: SessionDerivePreviewParams = {},
  signal?: AbortSignal
): Promise<SessionDerivePreview> {
  const messageId = params.messageId?.trim();
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/derive/preview",
    {
      params: {
        path: { workspace_id: workspaceId, session_id: sessionId },
        query: messageId ? { message_id: messageId } : undefined,
      },
      signal,
    }
  );
  const fallback = `Failed to measure session "${sessionId}"`;
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, fallback, sessionId);
  }
  return requireResponseData(data, response, fallback);
}

export async function continueSession(
  workspaceId: string,
  sessionId: string,
  body: ContinueSessionRequest,
  signal?: AbortSignal
): Promise<SessionDeriveResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/continue",
    {
      body,
      params: { path: { workspace_id: workspaceId, session_id: sessionId } },
      signal,
    }
  );
  const fallback = `Failed to continue session "${sessionId}"`;
  if (apiRequestFailed(response, error)) {
    // The source was found when the dialog opened; a 404 now names the daemon's
    // reason (agent or source gone) and must reach the operator verbatim.
    throwSessionRequestError(response, error, fallback);
  }
  return requireResponseData(data, response, fallback);
}
