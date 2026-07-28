import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type {
  ApproveSessionParams,
  CreateSessionParams,
  FetchSessionEventsParams,
  SessionLedgerResponse,
  SessionEventPayload,
  SessionPayload,
  SessionPromptPayload,
  SessionPromptRequest,
  SessionPromptResult,
  SessionRecapPayload,
  SessionRepairPayload,
  SessionRepairQuery,
  SessionUsagePayload,
  TurnHistoryPayload,
} from "../types";
import {
  SessionApiError,
  SessionNotFoundError,
  throwSessionRequestError,
} from "./session-api-errors";

export { fetchSessions } from "./session-catalog-api";
export { fetchSessionGoal } from "./session-goal-api";
export { buildSessionStreamUrl, fetchSessionTranscript } from "./session-transcript-api";
export type { SessionStreamCursor } from "./session-transcript-api";
export { SessionApiError, SessionNotFoundError } from "./session-api-errors";
export {
  answerSessionClarification,
  ClarificationNotAnswerableError,
  fetchSessionClarifications,
} from "./session-clarification-api";

export type {
  ApproveSessionParams,
  CreateSessionParams,
  FetchSessionEventsParams,
  PermissionDecision,
  SessionRepairQuery,
} from "../types";

export async function createSession(
  params: CreateSessionParams,
  signal?: AbortSignal
): Promise<SessionPayload> {
  const { data, error, response } = await apiClient.POST("/api/sessions", {
    body: params,
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
      params: { path: { workspace_id: workspaceId, session_id: id } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to fetch session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to fetch session "${id}"`).session;
}

export async function fetchSessionById(id: string, signal?: AbortSignal): Promise<SessionPayload> {
  const { data, error, response } = await apiClient.GET("/api/sessions/{session_id}", {
    params: { path: { session_id: id } },
    signal,
  });
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

export async function stopSession(
  workspaceId: string,
  id: string,
  signal?: AbortSignal
): Promise<void> {
  const { error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/stop",
    {
      params: { path: { workspace_id: workspaceId, session_id: id } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to stop session "${id}"`, id);
  }
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

export async function sendSessionPrompt(
  workspaceId: string,
  id: string,
  params: SessionPromptRequest,
  signal?: AbortSignal
): Promise<SessionPromptResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/prompt",
    {
      params: { path: { workspace_id: workspaceId, session_id: id } },
      body: params,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to send prompt to session "${id}"`, id);
  }
  const result = requireResponseData(data, response, `Failed to send prompt to session "${id}"`);
  return "prompt" in result ? result.prompt : result;
}

export async function interruptSessionPrompt(
  workspaceId: string,
  id: string,
  signal?: AbortSignal
): Promise<SessionPromptPayload> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/interrupt",
    {
      params: { path: { workspace_id: workspaceId, session_id: id } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to interrupt session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to interrupt session "${id}"`).prompt;
}

export async function steerSessionPrompt(
  workspaceId: string,
  id: string,
  text: string,
  signal?: AbortSignal
): Promise<SessionPromptPayload> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/steer",
    {
      params: { path: { workspace_id: workspaceId, session_id: id } },
      body: { text },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to steer session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to steer session "${id}"`).prompt;
}

export async function cancelQueuedSessionPrompt(
  workspaceId: string,
  id: string,
  queueEntryId: string,
  signal?: AbortSignal
): Promise<SessionPromptPayload> {
  const { data, error, response } = await apiClient.DELETE(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/prompt/queue/{queue_entry_id}",
    {
      params: {
        path: {
          workspace_id: workspaceId,
          session_id: id,
          queue_entry_id: queueEntryId,
        },
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(
      response,
      error,
      `Failed to cancel queued prompt "${queueEntryId}" for session "${id}"`,
      id
    );
  }
  return requireResponseData(
    data,
    response,
    `Failed to cancel queued prompt "${queueEntryId}" for session "${id}"`
  ).prompt;
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

export async function fetchSessionRecap(
  workspaceId: string,
  id: string,
  limit?: number,
  signal?: AbortSignal
): Promise<SessionRecapPayload> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/recap",
    {
      params: {
        path: { workspace_id: workspaceId, session_id: id },
        query: limit === undefined ? undefined : { limit },
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to fetch session recap "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to fetch session recap "${id}"`).recap;
}

export async function fetchSessionUsage(
  workspaceId: string,
  id: string,
  signal?: AbortSignal
): Promise<SessionUsagePayload> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/usage",
    {
      params: { path: { workspace_id: workspaceId, session_id: id } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to fetch session usage "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to fetch session usage "${id}"`).usage;
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

export async function fetchSessionEvents(
  workspaceId: string,
  id: string,
  params?: FetchSessionEventsParams,
  signal?: AbortSignal
): Promise<SessionEventPayload[]> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/events",
    {
      params: {
        path: { workspace_id: workspaceId, session_id: id },
        query: params,
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to fetch session events "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to fetch session events "${id}"`).events;
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

export async function fetchSessionHistory(
  workspaceId: string,
  id: string,
  signal?: AbortSignal
): Promise<TurnHistoryPayload[]> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/history",
    {
      params: { path: { workspace_id: workspaceId, session_id: id } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to fetch session history "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to fetch session history "${id}"`).history;
}

export class SessionLedgerUnavailableError extends SessionApiError {
  constructor(id: string) {
    super(`Session ledger not materialized: ${id}`, 404, id);
    this.name = "SessionLedgerUnavailableError";
  }
}

export async function fetchSessionLedger(
  workspaceId: string,
  id: string,
  signal?: AbortSignal
): Promise<SessionLedgerResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/memory/sessions/{session_id}/ledger",
    {
      params: { path: { workspace_id: workspaceId, session_id: id } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new SessionLedgerUnavailableError(id);
    }
    throwSessionRequestError(response, error, `Failed to fetch session ledger "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to fetch session ledger "${id}"`);
}
