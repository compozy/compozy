import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type {
  SessionLinkedWorkResponse,
  SessionWorkCandidates,
  SessionWorkLinkRequest,
} from "../lib/session-linked-work";
import { SessionApiError, throwSessionRequestError } from "./session-api-errors";
import { sessionWorkspacePath } from "./session-workspace-path";

/** The pull request belongs to a repository other than this session's project. */
export function isRepositoryMismatch(error: unknown): boolean {
  return error instanceof SessionApiError && error.code === "session_link_repository_mismatch";
}

/** The pull request's head lives in a fork, whose status this repository can't report. */
export function isForkUnsupported(error: unknown): boolean {
  return error instanceof SessionApiError && error.code === "session_link_fork_unsupported";
}

export type SessionLinkCandidateKind = "pull_requests" | "branches";

export interface SessionLinkCandidateQuery {
  q: string;
  kind: SessionLinkCandidateKind;
  page: number;
  limit: number;
}

function linksPath(workspaceId: string, sessionId: string) {
  return { workspace_id: sessionWorkspacePath(workspaceId), session_id: sessionId };
}

/** A write acts as the profile that owns the session, whichever lens is showing it. */
function mutationParams(workspaceId: string, sessionId: string, profile: string) {
  return { path: linksPath(workspaceId, sessionId), query: profile ? { profile } : {} };
}

export async function fetchSessionLinkedWork(
  workspaceId: string,
  sessionId: string,
  signal?: AbortSignal
): Promise<SessionLinkedWorkResponse> {
  const fallback = `Failed to read what is linked to session "${sessionId}"`;
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/links",
    { params: { path: linksPath(workspaceId, sessionId), query: { all_profiles: true } }, signal }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, fallback, sessionId);
  }
  return requireResponseData(data, response, fallback);
}

export async function addSessionWorkLink(
  workspaceId: string,
  sessionId: string,
  profile: string,
  body: SessionWorkLinkRequest,
  signal?: AbortSignal
): Promise<SessionLinkedWorkResponse> {
  const fallback = "Couldn't link this to the session";
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/links",
    { params: mutationParams(workspaceId, sessionId, profile), body, signal }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, fallback, sessionId);
  }
  return requireResponseData(data, response, fallback);
}

export async function removeSessionWorkLink(
  workspaceId: string,
  sessionId: string,
  profile: string,
  body: SessionWorkLinkRequest,
  signal?: AbortSignal
): Promise<SessionLinkedWorkResponse> {
  const fallback = "Couldn't unlink this from the session";
  const { data, error, response } = await apiClient.DELETE(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/links",
    { params: mutationParams(workspaceId, sessionId, profile), body, signal }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, fallback, sessionId);
  }
  return requireResponseData(data, response, fallback);
}

/** Asks the forge again about one item, then answers with the whole list. */
export async function refreshSessionWorkLink(
  workspaceId: string,
  sessionId: string,
  profile: string,
  body: SessionWorkLinkRequest,
  signal?: AbortSignal
): Promise<SessionLinkedWorkResponse> {
  const fallback = "Couldn't refresh this";
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/links/refresh",
    { params: mutationParams(workspaceId, sessionId, profile), body, signal }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, fallback, sessionId);
  }
  return requireResponseData(data, response, fallback);
}

export async function searchSessionLinkCandidates(
  workspaceId: string,
  sessionId: string,
  { q, kind, page, limit }: SessionLinkCandidateQuery,
  signal?: AbortSignal
): Promise<SessionWorkCandidates> {
  const fallback = "Couldn't search this project";
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/link-candidates",
    {
      params: {
        path: linksPath(workspaceId, sessionId),
        query: { q, kind, page: String(page), limit: String(limit), all_profiles: true },
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, fallback, sessionId);
  }
  return requireResponseData(data, response, fallback);
}
