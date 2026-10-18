import {
  apiClient,
  apiErrorCode,
  apiErrorDiagnosticMessage,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";
import type { OperationRequestBody, OperationResponse } from "@/lib/api-contract";

import { SessionApiError, throwSessionRequestError } from "./session-api-errors";

/** The daemon's measurement of what a continue or fork of one session would carry. */
export type SessionDerivePreview = OperationResponse<"previewSessionDerive", 200>;
export type ContinueSessionRequest = OperationRequestBody<"continueSession">;
/** Same agent, runtime and account; a `message_id` cuts through that message's turn. */
export type ForkSessionRequest = OperationRequestBody<"forkSession">;
/** `201` for a new child, `200` for a replayed idempotency key — one shape. */
export type SessionDeriveResult = OperationResponse<"continueSession", 201>;
export type SessionDerivedOutcome = SessionDeriveResult["derived"];

/**
 * A continue or fork refused after the daemon already created the new session
 * (its first message could not be admitted, say): `childSessionId` names it so
 * the dialog can still open it. Retrying with the same idempotency key returns it.
 */
export class SessionDeriveCommittedError extends SessionApiError {
  constructor(
    message: string,
    status: number,
    code: string | undefined,
    public readonly childSessionId: string
  ) {
    super(message, status, undefined, code ? { code } : {});
    this.name = "SessionDeriveCommittedError";
  }
}

function committedChildSessionId(error: unknown): string {
  if (typeof error !== "object" || error === null || !("child_session_id" in error)) return "";
  const id = (error as { child_session_id?: unknown }).child_session_id;
  return typeof id === "string" ? id.trim() : "";
}

function throwSessionDeriveError(response: Response, error: unknown, fallback: string): never {
  const childSessionId = committedChildSessionId(error);
  if (!childSessionId) throwSessionRequestError(response, error, fallback);
  // After the commit the daemon's structured diagnostic is the refusal to show; the raw
  // error text is only the fallback.
  throw new SessionDeriveCommittedError(
    apiErrorDiagnosticMessage(error) ?? defaultApiErrorMessage(fallback, response, error),
    response.status,
    apiErrorCode(error),
    childSessionId
  );
}

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
    throwSessionDeriveError(response, error, fallback);
  }
  return requireResponseData(data, response, fallback);
}

export async function forkSession(
  workspaceId: string,
  sessionId: string,
  body: ForkSessionRequest,
  signal?: AbortSignal
): Promise<SessionDeriveResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/fork",
    {
      body,
      params: { path: { workspace_id: workspaceId, session_id: sessionId } },
      signal,
    }
  );
  const fallback = `Failed to fork session "${sessionId}"`;
  if (apiRequestFailed(response, error)) {
    // A 404 here is the daemon's `message_not_found` (or the source gone): its
    // text names the reason and reaches the operator verbatim.
    throwSessionDeriveError(response, error, fallback);
  }
  return requireResponseData(data, response, fallback);
}
