import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type { SessionPayload } from "../types";
import type { SessionProcessPolicy } from "./session-processes-api";
import { throwSessionRequestError } from "./session-api-errors";
import { sessionWorkspacePath } from "./session-workspace-path";

/**
 * Archives a session using its explicit owner profile. `stop` lets the daemon
 * stop a live session through the normal stop path before archiving it;
 * `processes` decides whether the processes it left running stop with it.
 */
export async function archiveSession(
  workspaceId: string,
  id: string,
  profile: string,
  signal?: AbortSignal,
  stop = false,
  processes?: SessionProcessPolicy
): Promise<SessionPayload> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/archive",
    {
      params: {
        path: { workspace_id: sessionWorkspacePath(workspaceId), session_id: id },
        query: {
          profile,
          ...(stop ? { stop: true } : {}),
          ...(processes ? { processes } : {}),
        },
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to archive session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to archive session "${id}"`).session;
}

/** Archives a session with its child sessions, as the daemon reads the tree. */
export async function archiveSessionTree(
  workspaceId: string,
  id: string,
  profile: string,
  signal?: AbortSignal,
  stop = false,
  processes?: SessionProcessPolicy
): Promise<SessionPayload> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/archive",
    {
      params: {
        path: { workspace_id: sessionWorkspacePath(workspaceId), session_id: id },
        query: {
          profile,
          with_children: true,
          ...(stop ? { stop: true } : {}),
          ...(processes ? { processes } : {}),
        },
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to archive session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to archive session "${id}"`).session;
}

/** Deletes a session with its child sessions, as the daemon reads the tree. */
export async function deleteSessionTree(
  workspaceId: string,
  id: string,
  profile: string,
  signal?: AbortSignal,
  processes?: SessionProcessPolicy
): Promise<void> {
  const { error, response } = await apiClient.DELETE(
    "/api/workspaces/{workspace_id}/sessions/{session_id}",
    {
      params: {
        path: { workspace_id: sessionWorkspacePath(workspaceId), session_id: id },
        query: { profile, with_children: true, ...(processes ? { processes } : {}) },
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to delete session "${id}"`, id);
  }
}

/** Lists every child session, recursively; the paginated catalog cannot answer this. */
export async function listSessionDescendants(
  workspaceId: string,
  id: string,
  profile: string,
  signal?: AbortSignal
): Promise<SessionPayload[]> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/descendants",
    {
      params: {
        path: { workspace_id: sessionWorkspacePath(workspaceId), session_id: id },
        query: { profile },
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to read children of session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to read children of session "${id}"`).sessions;
}

/** Restores an archived session within its owning workspace and profile. */
export async function unarchiveSession(
  workspaceId: string,
  id: string,
  profile: string,
  signal?: AbortSignal
): Promise<SessionPayload> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/unarchive",
    {
      params: {
        path: { workspace_id: sessionWorkspacePath(workspaceId), session_id: id },
        query: { profile },
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to unarchive session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to unarchive session "${id}"`).session;
}
