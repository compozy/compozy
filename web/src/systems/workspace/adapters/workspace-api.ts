import {
  apiClientFor,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
  type ComputerId,
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
  archived = false,
  computerId?: ComputerId
): Promise<WorkspacePayload[]> {
  const { data, error, response } = await apiClientFor(computerId).GET("/api/workspaces", {
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
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<WorkspaceDetailPayload> {
  const { data, error, response } = await apiClientFor(computerId).GET("/api/workspaces/{id}", {
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

export async function deleteWorkspace(
  workspaceID: string,
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<void> {
  const { error, response } = await apiClientFor(computerId).DELETE("/api/workspaces/{id}", {
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
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<WorkspacePayload> {
  const { data, error, response } = await apiClientFor(computerId).POST("/api/workspaces", {
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
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<WorkspacePayload> {
  const { data, error, response } = await apiClientFor(computerId).POST("/api/workspaces/resolve", {
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
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<WorkspacePayload> {
  const path = archived ? "/api/workspaces/{id}/archive" : "/api/workspaces/{id}/unarchive";
  const { data, error, response } = await apiClientFor(computerId).POST(path, {
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
