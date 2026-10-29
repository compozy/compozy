import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type { WorkspaceFolderPayload, WorkspaceLayoutResponse } from "../types";
import { throwSessionRequestError } from "./session-api-errors";

/**
 * The acting profile that owns the project folders; omitted resolves the daemon
 * default. A mutation's `signal` only stops waiting: a write the daemon already
 * accepted still lands.
 */
function profileQuery(profile: string | undefined) {
  return { query: profile ? { profile } : {} };
}

export async function fetchWorkspaceLayout(
  profile: string | undefined,
  signal?: AbortSignal
): Promise<WorkspaceLayoutResponse> {
  const { data, error, response } = await apiClient.GET("/api/workspace-folders", {
    params: profileQuery(profile),
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to fetch project folders");
  }
  return requireResponseData(data, response, "Failed to fetch project folders");
}

export async function createWorkspaceFolder(
  profile: string | undefined,
  name: string,
  signal?: AbortSignal
): Promise<WorkspaceFolderPayload> {
  const { data, error, response } = await apiClient.POST("/api/workspace-folders", {
    signal,
    params: profileQuery(profile),
    body: { name },
  });
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to create folder");
  }
  return requireResponseData(data, response, "Failed to create folder").folder;
}

export async function renameWorkspaceFolder(
  profile: string | undefined,
  folderId: string,
  name: string,
  signal?: AbortSignal
): Promise<WorkspaceFolderPayload> {
  const { data, error, response } = await apiClient.PATCH("/api/workspace-folders/{folder_id}", {
    params: { ...profileQuery(profile), path: { folder_id: folderId } },
    body: { name },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to rename folder");
  }
  return requireResponseData(data, response, "Failed to rename folder").folder;
}

/** Deletes a folder; its projects return to the list. */
export async function deleteWorkspaceFolder(
  profile: string | undefined,
  folderId: string,
  signal?: AbortSignal
): Promise<WorkspaceLayoutResponse> {
  const { data, error, response } = await apiClient.DELETE("/api/workspace-folders/{folder_id}", {
    params: { ...profileQuery(profile), path: { folder_id: folderId } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to delete folder");
  }
  return requireResponseData(data, response, "Failed to delete folder");
}

/** Sets the complete folder order; every folder of the profile is listed once. */
export async function reorderWorkspaceFolders(
  profile: string | undefined,
  folderIds: readonly string[],
  signal?: AbortSignal
): Promise<WorkspaceLayoutResponse> {
  const { data, error, response } = await apiClient.PUT("/api/workspace-order/folders", {
    signal,
    params: profileQuery(profile),
    body: { folder_ids: [...folderIds] },
  });
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to reorder folders");
  }
  return requireResponseData(data, response, "Failed to reorder folders");
}

/**
 * Pins `workspaceIds` (project ids or `@user`) in order at the head of one
 * folder, or of the root when `folderId` is null; they move there if needed.
 */
export async function reorderWorkspaces(
  profile: string | undefined,
  folderId: string | null,
  workspaceIds: readonly string[],
  signal?: AbortSignal
): Promise<WorkspaceLayoutResponse> {
  const { data, error, response } = await apiClient.PUT("/api/workspace-order/workspaces", {
    signal,
    params: profileQuery(profile),
    body: { workspace_ids: [...workspaceIds], ...(folderId ? { folder_id: folderId } : {}) },
  });
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to reorder projects");
  }
  return requireResponseData(data, response, "Failed to reorder projects");
}

/** Moves a project to the end of a folder, or of the root when `folderId` is null. */
export async function moveWorkspace(
  profile: string | undefined,
  workspaceId: string,
  folderId: string | null,
  signal?: AbortSignal
): Promise<WorkspaceLayoutResponse> {
  const { data, error, response } = await apiClient.POST("/api/workspace-order/move", {
    signal,
    params: profileQuery(profile),
    body: { workspace_id: workspaceId, ...(folderId ? { folder_id: folderId } : {}) },
  });
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to move project");
  }
  return requireResponseData(data, response, "Failed to move project");
}
