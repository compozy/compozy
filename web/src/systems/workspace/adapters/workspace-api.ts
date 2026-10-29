import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";
import type { OperationRequestBody, OperationResponse } from "@/lib/api-contract";

import type { WorkspaceDetailPayload, WorkspacePayload } from "../types";

export type ResolveWorkspaceParams = OperationRequestBody<"resolveWorkspace">;
export type CreateWorkspaceParams = OperationRequestBody<"createWorkspace">;

export class WorkspaceApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly code?: string,
    public readonly blockers: NonNullable<
      OperationResponse<"archiveWorkspace", 409>["blockers"]
    > = []
  ) {
    super(message);
    this.name = "WorkspaceApiError";
  }
}

export async function fetchWorkspaces(
  signal?: AbortSignal,
  archived = false
): Promise<WorkspacePayload[]> {
  const { data, error, response } = await apiClient.GET("/api/workspaces", {
    signal,
    ...(archived ? { params: { query: { archived: true } } } : {}),
  });
  if (apiRequestFailed(response, error)) {
    throw new WorkspaceApiError(
      defaultApiErrorMessage("Failed to fetch workspaces", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to fetch workspaces").workspaces;
}

export async function fetchWorkspace(
  workspaceID: string,
  signal?: AbortSignal
): Promise<WorkspaceDetailPayload> {
  const { data, error, response } = await apiClient.GET("/api/workspaces/{id}", {
    params: { path: { id: workspaceID } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new WorkspaceApiError(
      defaultApiErrorMessage("Failed to fetch workspace", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to fetch workspace");
}

export async function deleteWorkspace(workspaceID: string, signal?: AbortSignal): Promise<void> {
  const { error, response } = await apiClient.DELETE("/api/workspaces/{id}", {
    params: { path: { id: workspaceID } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new WorkspaceApiError(
      defaultApiErrorMessage("Failed to delete workspace", response, error),
      response.status
    );
  }
}

/**
 * Registers a workspace with its full configuration in one write.
 *
 * `resolveWorkspace` is the get-or-create path used for a bare directory;
 * this is the create path that also carries name, extra dirs, and defaults.
 */
export async function createWorkspace(
  params: CreateWorkspaceParams,
  signal?: AbortSignal
): Promise<WorkspacePayload> {
  const { data, error, response } = await apiClient.POST("/api/workspaces", {
    body: params,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new WorkspaceApiError(
      defaultApiErrorMessage("Failed to create workspace", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to create workspace").workspace;
}

export async function resolveWorkspace(
  params: ResolveWorkspaceParams,
  signal?: AbortSignal
): Promise<WorkspacePayload> {
  const { data, error, response } = await apiClient.POST("/api/workspaces/resolve", {
    body: params,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new WorkspaceApiError(
      defaultApiErrorMessage("Failed to resolve workspace", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to resolve workspace").workspace;
}

export async function setWorkspaceArchived(
  workspaceID: string,
  archived: boolean,
  signal?: AbortSignal
): Promise<WorkspacePayload> {
  const path = archived ? "/api/workspaces/{id}/archive" : "/api/workspaces/{id}/unarchive";
  const { data, error, response } = await apiClient.POST(path, {
    signal,
    params: { path: { id: workspaceID } },
  });
  if (apiRequestFailed(response, error)) {
    throw new WorkspaceApiError(
      defaultApiErrorMessage("Failed to update project", response, error),
      response.status,
      error?.code,
      error?.blockers ?? []
    );
  }
  return requireResponseData(data, response, "Failed to update project").workspace;
}
