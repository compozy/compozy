import type { SessionPayload } from "../types";
import { SessionApiError, throwSessionRequestError } from "./session-api-errors";
import { sessionWorkspacePath } from "./session-workspace-path";

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

/** Rewinds an owner-scoped session using a daemon-issued transcript fence. */
export async function rewindSession(
  workspaceId: string,
  id: string,
  profile: string,
  params: SessionRewindRequest,
  signal: AbortSignal
): Promise<SessionRewindResult> {
  const request = new Request(
    new URL(
      `/api/workspaces/${encodeURIComponent(sessionWorkspacePath(workspaceId))}/sessions/${encodeURIComponent(id)}/rewind?${new URLSearchParams({ profile })}`,
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
    const error: unknown = await response.json().catch(() => null);
    throwSessionRequestError(response, error, "Couldn't rewind this session", id);
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
