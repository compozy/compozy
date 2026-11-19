import { daemonFetch } from "@/lib/api-client";

import { throwSessionRequestError } from "./session-api-errors";
import { type SessionDeriveResult, throwSessionDeriveError } from "./session-derive-api";
import { sessionWorkspacePath } from "./session-workspace-path";

/*
 * Side chat transport (`_dx.md` HTTP / UDS API). The daemon routes ship with the backend slice;
 * until the generated client carries `openSessionSide` / `listSessionSides`, these two calls go
 * through `daemonFetch` against the documented paths and the DTOs below mirror the contract.
 * Swapping to `apiClient.POST/GET` and `OperationResponse<…>` types changes only this file.
 */

/** `POST …/sides` body. `message` is required and non-blank; the key makes a retry idempotent. */
export interface OpenSessionSideRequest {
  message: string;
  idempotency_key: string;
  name?: string;
}

/** One side chat of a session (`SessionSideItemPayload`). */
export interface SessionSideItem {
  id: string;
  name: string;
  state: string;
  prompting: boolean;
  pending_permission_count: number;
  pending_clarify_count: number;
  archived_at: string | null;
  created_at: string;
  updated_at: string;
}

/** `GET …/sides`: the complete list, newest `updated_at` first, never paginated. */
export interface SessionSidesResponse {
  sides: SessionSideItem[];
}

/** `201` for a new side chat, `200` for a replayed key: the derive response with `derived.kind = "side"`. */
export type OpenSessionSideResult = SessionDeriveResult;

function sidesUrl(workspaceId: string, sessionId: string, query?: URLSearchParams): URL {
  const path = `/api/workspaces/${encodeURIComponent(sessionWorkspacePath(workspaceId))}/sessions/${encodeURIComponent(sessionId)}/sides`;
  const url = new URL(
    path,
    typeof window === "undefined" ? "http://localhost" : window.location.origin
  );
  if (query) url.search = query.toString();
  return url;
}

async function readJson(response: Response): Promise<unknown> {
  try {
    return await response.json();
  } catch {
    return undefined;
  }
}

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
  const response = await daemonFetch(
    new Request(sidesUrl(workspaceId, sessionId), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
      signal,
    })
  );
  const payload = await readJson(response);
  const fallback = `Failed to open a side chat on session "${sessionId}"`;
  if (!response.ok) throwSessionDeriveError(response, payload, fallback);
  if (payload === undefined) throw new Error(`${fallback}: empty response (${response.status})`);
  return payload as OpenSessionSideResult;
}

/** Lists a session's side chats; `archived` includes the closed ones. */
export async function fetchSessionSides(
  workspaceId: string,
  sessionId: string,
  options: { archived?: boolean } = {},
  signal?: AbortSignal
): Promise<SessionSidesResponse> {
  const query = options.archived ? new URLSearchParams({ archived: "true" }) : undefined;
  const response = await daemonFetch(
    new Request(sidesUrl(workspaceId, sessionId, query), { method: "GET", signal })
  );
  const payload = await readJson(response);
  const fallback = `Failed to list side chats of session "${sessionId}"`;
  if (!response.ok) throwSessionRequestError(response, payload, fallback, sessionId);
  if (payload === undefined) throw new Error(`${fallback}: empty response (${response.status})`);
  const sides = (payload as Partial<SessionSidesResponse>).sides;
  return { sides: Array.isArray(sides) ? sides : [] };
}
