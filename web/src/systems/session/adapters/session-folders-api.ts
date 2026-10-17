import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type { SessionFolderPayload, SessionFoldersResponse } from "../types";
import { throwSessionRequestError } from "./session-api-errors";

/** The workspace and acting profile that own one session layout. */
export interface SessionLayoutScope {
  workspaceId: string;
  profile?: string;
}

function layoutParams(scope: SessionLayoutScope) {
  return {
    path: { workspace_id: scope.workspaceId },
    query: scope.profile ? { profile: scope.profile } : {},
  };
}

export async function fetchSessionFolders(
  scope: SessionLayoutScope,
  signal?: AbortSignal
): Promise<SessionFolderPayload[]> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/session-folders",
    { params: layoutParams(scope), signal }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to fetch session folders");
  }
  return requireResponseData(data, response, "Failed to fetch session folders").folders;
}

export async function createSessionFolder(
  scope: SessionLayoutScope,
  name: string
): Promise<SessionFolderPayload> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/session-folders",
    { params: layoutParams(scope), body: { name } }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to create folder");
  }
  return requireResponseData(data, response, "Failed to create folder").folder;
}

export async function renameSessionFolder(
  scope: SessionLayoutScope,
  folderId: string,
  name: string
): Promise<SessionFolderPayload> {
  const { data, error, response } = await apiClient.PATCH(
    "/api/workspaces/{workspace_id}/session-folders/{folder_id}",
    {
      params: {
        ...layoutParams(scope),
        path: { workspace_id: scope.workspaceId, folder_id: folderId },
      },
      body: { name },
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to rename folder");
  }
  return requireResponseData(data, response, "Failed to rename folder").folder;
}

export async function deleteSessionFolder(
  scope: SessionLayoutScope,
  folderId: string
): Promise<SessionFoldersResponse["folders"]> {
  const { data, error, response } = await apiClient.DELETE(
    "/api/workspaces/{workspace_id}/session-folders/{folder_id}",
    {
      params: {
        ...layoutParams(scope),
        path: { workspace_id: scope.workspaceId, folder_id: folderId },
      },
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to delete folder");
  }
  return requireResponseData(data, response, "Failed to delete folder").folders;
}

export async function reorderSessionFolders(
  scope: SessionLayoutScope,
  folderIds: readonly string[]
): Promise<SessionFoldersResponse["folders"]> {
  const { data, error, response } = await apiClient.PUT(
    "/api/workspaces/{workspace_id}/session-order/folders",
    { params: layoutParams(scope), body: { folder_ids: [...folderIds] } }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to reorder folders");
  }
  return requireResponseData(data, response, "Failed to reorder folders").folders;
}

/** Pins `sessionIds` in order inside one folder, or the workspace root when `folderId` is null. */
export async function reorderSessions(
  scope: SessionLayoutScope,
  folderId: string | null,
  sessionIds: readonly string[]
): Promise<SessionFoldersResponse["folders"]> {
  const { data, error, response } = await apiClient.PUT(
    "/api/workspaces/{workspace_id}/session-order/sessions",
    {
      params: layoutParams(scope),
      body: { session_ids: [...sessionIds], ...(folderId ? { folder_id: folderId } : {}) },
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to reorder sessions");
  }
  return requireResponseData(data, response, "Failed to reorder sessions").folders;
}

/** Moves one session to the end of a folder, or back to the workspace root when `folderId` is null. */
export async function moveSessionToFolder(
  scope: SessionLayoutScope,
  sessionId: string,
  folderId: string | null
): Promise<SessionFoldersResponse["folders"]> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/move",
    {
      params: {
        ...layoutParams(scope),
        path: { workspace_id: scope.workspaceId, session_id: sessionId },
      },
      body: folderId ? { folder_id: folderId } : {},
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to move session");
  }
  return requireResponseData(data, response, "Failed to move session").folders;
}
