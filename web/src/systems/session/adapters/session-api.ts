import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type {
  ApproveSessionParams,
  CreateSessionParams,
  SessionPayload,
  SessionPromptRequest,
  SessionPromptResponse,
  SessionPromptSendResult,
  SessionRepairPayload,
  SessionRepairQuery,
  RenameSessionRequest,
  SetSessionRuntimeRequest,
  SessionStopResult,
} from "../types";
import {
  SessionApiError,
  SessionNotFoundError,
  throwSessionRequestError,
} from "./session-api-errors";

export { fetchSessions } from "./session-catalog-api";
export { archiveSession, unarchiveSession } from "./session-archive-api";
export { fetchSessionCommands } from "./session-command-api";
export { fetchSessionGoal, mutateSessionGoal } from "./session-goal-api";
export {
  cancelQueuedSessionPrompt,
  fetchSessionInputs,
  promoteSessionInputToSteer,
  replaceSessionInput,
} from "./session-input-api";
export { buildSessionStreamUrl, fetchSessionTranscript } from "./session-transcript-api";
export type { SessionStreamCursor } from "./session-transcript-api";
export { rewindSession } from "./session-rewind-api";
export type { SessionRewindRequest, SessionRewindResult } from "./session-rewind-api";
export { SessionApiError, SessionNotFoundError } from "./session-api-errors";
export {
  answerSessionClarification,
  ClarificationNotAnswerableError,
  fetchSessionClarifications,
} from "./session-clarification-api";
export {
  fetchSessionInteractions,
  type FetchSessionInteractionsOptions,
} from "./session-interactions-api";
export {
  fetchSessionEvents,
  fetchSessionHistory,
  fetchSessionLedger,
  fetchSessionRecap,
  fetchSessionUsage,
  SessionLedgerUnavailableError,
} from "./session-history-api";

export type {
  ApproveSessionParams,
  CreateSessionParams,
  FetchSessionEventsParams,
  PermissionDecision,
  SessionRepairQuery,
} from "../types";

export async function createSession(
  params: CreateSessionParams,
  /** The profile that will own the session. Omitting it files into `default`. */
  profile?: string,
  signal?: AbortSignal
): Promise<SessionPayload> {
  const { data, error, response } = await apiClient.POST("/api/sessions", {
    body: params,
    params: { query: profile ? { profile } : {} },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 409) {
      throw new SessionApiError("Max sessions reached", 409);
    }
    throwSessionRequestError(response, error, "Failed to create session");
  }
  return requireResponseData(data, response, "Failed to create session").session;
}

export async function fetchSession(
  workspaceId: string,
  id: string,
  signal?: AbortSignal
): Promise<SessionPayload> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}",
    {
      params: {
        path: { workspace_id: workspaceId, session_id: id },
        query: { include_health: true },
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to fetch session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to fetch session "${id}"`).session;
}

export async function deleteSession(
  workspaceId: string,
  id: string,
  signal?: AbortSignal
): Promise<void> {
  const { error, response } = await apiClient.DELETE(
    "/api/workspaces/{workspace_id}/sessions/{session_id}",
    {
      params: { path: { workspace_id: workspaceId, session_id: id } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to delete session "${id}"`, id);
  }
}

export async function renameSession(
  workspaceId: string,
  id: string,
  request: RenameSessionRequest,
  signal?: AbortSignal
): Promise<SessionPayload> {
  const { data, error, response } = await apiClient.PATCH(
    "/api/workspaces/{workspace_id}/sessions/{session_id}",
    {
      params: { path: { workspace_id: workspaceId, session_id: id } },
      body: request,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to rename session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to rename session "${id}"`).session;
}

export interface StopSessionOptions {
  signal?: AbortSignal;
  /**
   * Block on the shared stop manager until the ladder settles (verified or
   * unverified). Off by default: an ordinary stop is accepted with 202 and the
   * session read model owns the settled state.
   */
  wait?: boolean;
}

/**
 * Requests a session stop. Without `wait` the daemon answers 202 once the stop
 * is accepted (or 200 when the session is already stopped) and settles the
 * session through the shared stop manager; with `wait` it answers 200 with the
 * settled outcome, including an unverified one. The answer is evidence that
 * this request settled — session state stays with the read model.
 */
export async function stopSession(
  workspaceId: string,
  id: string,
  options: StopSessionOptions = {}
): Promise<SessionStopResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/stop",
    {
      params: { path: { workspace_id: workspaceId, session_id: id } },
      body: { wait: options.wait === true },
      signal: options.signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to stop session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to stop session "${id}"`);
}

export async function cancelSessionPrompt(
  workspaceId: string,
  id: string,
  signal?: AbortSignal
): Promise<void> {
  const request = new Request(
    new URL(
      `/api/workspaces/${encodeURIComponent(workspaceId)}/sessions/${encodeURIComponent(id)}/prompt/cancel`,
      typeof window === "undefined" ? "http://localhost" : window.location.origin
    ),
    {
      method: "POST",
      signal,
    }
  );
  const response = await globalThis.fetch(request);
  if (!response.ok) {
    if (response.status === 404) {
      throw new SessionNotFoundError(id);
    }
    throw new SessionApiError(
      `Failed to cancel prompt for session "${id}": ${response.status}`,
      response.status,
      id
    );
  }
}

function isPromptEventStream(response: Response): boolean {
  return (response.headers.get("content-type") ?? "").toLowerCase().includes("text/event-stream");
}

/**
 * Busy-send entry point: the daemon answers a busy session with the JSON
 * disposition envelope. When the session turned idle before admission it
 * starts a direct turn and streams it instead; the stream is released
 * immediately (the turn runs detached and the live tail renders it) and the
 * caller learns the send became a direct turn.
 */
export async function sendSessionPrompt(
  workspaceId: string,
  id: string,
  params: SessionPromptRequest,
  signal?: AbortSignal
): Promise<SessionPromptSendResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/prompt",
    {
      params: { path: { workspace_id: workspaceId, session_id: id } },
      body: params,
      parseAs: "stream",
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to send prompt to session "${id}"`, id);
  }
  const body = requireResponseData(data, response, `Failed to send prompt to session "${id}"`);
  if (isPromptEventStream(response)) {
    await body?.cancel();
    return {
      direct_turn: true,
      idempotency_key: params.idempotency_key,
      message_id: params.message_id,
    };
  }
  // The daemon's JSON envelope; the generated response type owns its shape.
  const result: SessionPromptResponse = await new Response(body).json();
  if (!("prompt" in result)) {
    throw new SessionApiError(
      `Failed to send prompt to session "${id}": invalid response payload`,
      500,
      id
    );
  }
  return result.prompt.goal ?? result.prompt;
}

export async function resumeSession(
  workspaceId: string,
  id: string,
  signal?: AbortSignal
): Promise<SessionPayload> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/attach",
    {
      params: { path: { workspace_id: workspaceId, session_id: id } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to resume session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to resume session "${id}"`).session;
}

export async function setSessionRuntime(
  workspaceId: string,
  id: string,
  params: SetSessionRuntimeRequest,
  signal?: AbortSignal
): Promise<SessionPayload> {
  const { data, error, response } = await apiClient.PUT(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/runtime",
    {
      params: { path: { workspace_id: workspaceId, session_id: id } },
      body: params,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to save runtime for session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to save runtime for session "${id}"`).session;
}

export async function clearSessionRuntime(
  workspaceId: string,
  id: string,
  expectedRevision: number,
  signal?: AbortSignal
): Promise<SessionPayload> {
  const { data, error, response } = await apiClient.DELETE(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/runtime",
    {
      params: {
        path: { workspace_id: workspaceId, session_id: id },
        query: { expected_revision: expectedRevision },
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to clear runtime for session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to clear runtime for session "${id}"`).session;
}

export async function repairSession(
  workspaceId: string,
  id: string,
  query: SessionRepairQuery = {},
  signal?: AbortSignal
): Promise<SessionRepairPayload> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/repair",
    {
      params: {
        path: { workspace_id: workspaceId, session_id: id },
        query,
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to repair session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to repair session "${id}"`).repair;
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isSessionEnvelope(value: unknown): value is { session: SessionPayload } {
  if (!isPlainObject(value) || !("session" in value)) {
    return false;
  }

  const session = value.session;
  return isPlainObject(session) && typeof session.id === "string";
}

export async function clearSessionConversation(
  workspaceId: string,
  id: string,
  signal?: AbortSignal
): Promise<SessionPayload> {
  const request = new Request(
    new URL(
      `/api/workspaces/${encodeURIComponent(workspaceId)}/sessions/${encodeURIComponent(id)}/clear`,
      typeof window === "undefined" ? "http://localhost" : window.location.origin
    ),
    {
      method: "POST",
      signal,
    }
  );

  const response = await globalThis.fetch(request);
  if (!response.ok) {
    if (response.status === 404) {
      throw new SessionNotFoundError(id);
    }
    if (response.status === 409) {
      throw new SessionApiError(
        `Cannot clear session "${id}" while a prompt is still running`,
        409,
        id
      );
    }
    throw new SessionApiError(
      `Failed to clear session "${id}": ${response.status}`,
      response.status,
      id
    );
  }

  const body: unknown = await response.json();
  if (!isSessionEnvelope(body)) {
    throw new SessionApiError(
      `Failed to clear session "${id}": invalid response payload`,
      response.status,
      id
    );
  }

  return body.session;
}

export async function approveSession(
  workspaceId: string,
  id: string,
  params: ApproveSessionParams,
  signal?: AbortSignal
): Promise<void> {
  const { error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/approve",
    {
      params: { path: { workspace_id: workspaceId, session_id: id } },
      body: params,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to approve permission", id);
  }
}
