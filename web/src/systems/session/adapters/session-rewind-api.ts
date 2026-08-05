import type { SessionPayload } from "../types";
import { SessionApiError, SessionNotFoundError } from "./session-api-errors";

export interface SessionRewindRequest {
  expected_epoch: number;
  expected_generation: number;
  expected_max_sequence: number;
  idempotency_key: string;
  message_id: string;
}

export interface SessionRewindResult {
  rewind: {
    draft_text: string;
  };
  session: SessionPayload;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isSessionRewindResult(value: unknown): value is SessionRewindResult {
  if (!isRecord(value) || !isRecord(value.session) || !isRecord(value.rewind)) {
    return false;
  }
  return typeof value.session.id === "string" && typeof value.rewind.draft_text === "string";
}

export async function rewindSession(
  workspaceId: string,
  id: string,
  params: SessionRewindRequest,
  signal: AbortSignal
): Promise<SessionRewindResult> {
  const request = new Request(
    new URL(
      `/api/workspaces/${encodeURIComponent(workspaceId)}/sessions/${encodeURIComponent(id)}/rewind`,
      typeof window === "undefined" ? "http://localhost" : window.location.origin
    ),
    {
      body: JSON.stringify(params),
      headers: { "Content-Type": "application/json" },
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
      `Failed to rewind session "${id}": ${response.status}`,
      response.status,
      id
    );
  }

  const payload: unknown = await response.json();
  if (!isSessionRewindResult(payload)) {
    throw new SessionApiError(
      `Failed to rewind session "${id}": invalid response payload`,
      response.status,
      id
    );
  }

  return payload;
}
