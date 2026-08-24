import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type { SessionOwnerResponse, SessionPayload } from "../types";
import { throwSessionRequestError } from "./session-api-errors";
import type { ProfileScopeParams } from "@/systems/profiles";

type SessionByIDQuery =
  | (ProfileScopeParams & { include_health: true })
  | { all_profiles: true; include_health: true };

async function fetchSessionByIDWithQuery(
  sessionId: string,
  query: SessionByIDQuery,
  fallback: string,
  signal?: AbortSignal
): Promise<SessionPayload> {
  const { data, error, response } = await apiClient.GET("/api/sessions/{session_id}", {
    params: { path: { session_id: sessionId }, query },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, fallback, sessionId);
  }
  return requireResponseData(data, response, fallback).session;
}

/**
 * Workspace-agnostic ownership projection (`{session_id, workspace_id, workspace_name}`).
 * It is the only session read the deep-link flows may issue before the operator confirms a
 * workspace switch — no agent, provider, transcript, or activity fields exist on this surface.
 * A 404 surfaces as `SessionNotFoundError`, which keeps the not-found route outcome unchanged.
 */
export async function fetchSessionOwner(
  sessionId: string,
  signal?: AbortSignal
): Promise<SessionOwnerResponse> {
  const { data, error, response } = await apiClient.GET("/api/sessions/{session_id}/owner", {
    params: { path: { session_id: sessionId } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(
      response,
      error,
      `Failed to fetch session owner "${sessionId}"`,
      sessionId
    );
  }
  return requireResponseData(data, response, `Failed to fetch session owner "${sessionId}"`);
}

/**
 * The profile-enforced by-id read.
 *
 * `getSessionByID` compares the session's owner against the scope and answers
 * 404 when they differ. The selector is never omitted: omitting it resolves
 * `default` at the boundary, which would quietly hide every other profile's work
 * behind a 404 that means the wrong thing.
 */
export async function fetchSessionById(
  sessionId: string,
  scope: ProfileScopeParams,
  signal?: AbortSignal
): Promise<SessionPayload> {
  return fetchSessionByIDWithQuery(
    sessionId,
    { ...scope, include_health: true },
    `Failed to fetch session "${sessionId}"`,
    signal
  );
}

/**
 * The labeled aggregate-by-id read behind the deep-link owner banner.
 *
 * A scoped get answers 404 for another profile's session, which is the correct
 * default. This is the explicit second read mode: it widens once, on purpose,
 * and returns the session with its owner labelled — never a client-side
 * exception carved out of a scoped list (ADR-005, US-009.EC-2).
 */
export async function fetchSessionAcrossProfiles(
  sessionId: string,
  signal?: AbortSignal
): Promise<SessionPayload> {
  return fetchSessionByIDWithQuery(
    sessionId,
    { all_profiles: true, include_health: true },
    `Failed to fetch session "${sessionId}" across profiles`,
    signal
  );
}
