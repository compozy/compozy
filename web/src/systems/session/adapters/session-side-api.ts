import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";
import type { OperationRequestBody, OperationResponse } from "@/lib/api-contract";

import { throwSessionRequestError } from "./session-api-errors";
import { throwSessionDeriveError } from "./session-derive-api";
import { sessionWorkspacePath } from "./session-workspace-path";

/** `POST …/sides` body. `message` is required and non-blank; the key makes a retry idempotent. */
export type OpenSessionSideRequest = OperationRequestBody<"openSessionSide">;
/** `GET …/sides`: the complete list, newest `updated_at` first, never paginated. */
export type SessionSidesResponse = OperationResponse<"listSessionSides", 200>;
/** One side chat of a session. */
export type SessionSideItem = SessionSidesResponse["sides"][number];
/** `201` for a new side chat, `200` for a replayed key: the derive response with `derived.kind = "side"`. */
export type OpenSessionSideResult = OperationResponse<"openSessionSide", 201>;

/**
 * Opens a side chat on `sessionId`. A refusal after the side chat committed throws
 * `SessionDeriveCommittedError` naming it; a retry with the same key returns that side chat.
 */
export async function openSessionSide(
  workspaceId: string,
  sessionId: string,
  body: OpenSessionSideRequest,
  signal?: AbortSignal
): Promise<OpenSessionSideResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/sides",
    {
      body,
      params: { path: { workspace_id: sessionWorkspacePath(workspaceId), session_id: sessionId } },
      signal,
    }
  );
  const fallback = `Failed to open a side chat on session "${sessionId}"`;
  if (apiRequestFailed(response, error)) throwSessionDeriveError(response, error, fallback);
  return requireResponseData(data, response, fallback);
}

/** Lists a session's side chats; `archived` includes the closed ones. */
export async function fetchSessionSides(
  workspaceId: string,
  sessionId: string,
  options: { archived?: boolean } = {},
  signal?: AbortSignal
): Promise<SessionSidesResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/sides",
    {
      params: {
        path: { workspace_id: sessionWorkspacePath(workspaceId), session_id: sessionId },
        query: options.archived ? { archived: "true" } : undefined,
      },
      signal,
    }
  );
  const fallback = `Failed to list side chats of session "${sessionId}"`;
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, fallback, sessionId);
  }
  return requireResponseData(data, response, fallback);
}
