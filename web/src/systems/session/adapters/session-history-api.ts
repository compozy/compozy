import { apiClient, apiErrorCode, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type {
  FetchSessionEventsParams,
  SessionEventPayload,
  SessionLedgerResponse,
  SessionRecapPayload,
  SessionUsagePayload,
  TurnHistoryPayload,
} from "../types";
import { SessionApiError, throwSessionRequestError } from "./session-api-errors";

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
  /** Preserves the distinction between absent ledger data and unsupported memory. */
  constructor(
    id: string,
    public readonly reason: "not-materialized" | "unsupported" = "not-materialized"
  ) {
    super(
      reason === "unsupported"
        ? `Session memory unavailable: ${id}`
        : `Session ledger not materialized: ${id}`,
      reason === "unsupported" ? 501 : 404,
      id,
      reason === "unsupported" ? { code: "memory.unsupported" } : {}
    );
    this.name = "SessionLedgerUnavailableError";
  }
}

/** Reads a session ledger while distinguishing unsupported memory from an absent ledger. */
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
    if (response.status === 501 && apiErrorCode(error) === "memory.unsupported") {
      throw new SessionLedgerUnavailableError(id, "unsupported");
    }
    throwSessionRequestError(response, error, `Failed to fetch session ledger "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to fetch session ledger "${id}"`);
}
