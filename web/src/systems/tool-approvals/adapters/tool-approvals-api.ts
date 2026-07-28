import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  ToolApprovalGrant,
  ToolApprovalGrantsListResponse,
  ToolApprovalGrantSetRequest,
} from "../types";

export class ToolApprovalGrantsApiError extends Error {
  constructor(
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = "ToolApprovalGrantsApiError";
  }
}

/**
 * List the remembered native-tool approval decisions for one workspace.
 *
 * Returns the full daemon-owned envelope (`grants` + exact `total`); the view model
 * flattens it. `workspace_id` is required by the contract and always sent.
 */
export async function listToolApprovalGrants(
  workspaceId: string,
  signal?: AbortSignal
): Promise<ToolApprovalGrantsListResponse> {
  const { data, error, response } = await apiClient.GET("/api/tool-approval-grants", {
    params: { query: { workspace_id: workspaceId } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new ToolApprovalGrantsApiError(
      defaultApiErrorMessage("Failed to load remembered decisions", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load remembered decisions");
}

/** Set one explicit agent-wide or tool-wide decision in one workspace. */
export async function setToolApprovalGrant(
  workspaceId: string,
  request: ToolApprovalGrantSetRequest,
  signal?: AbortSignal
): Promise<ToolApprovalGrant> {
  const { data, error, response } = await apiClient.PUT("/api/tool-approval-grants", {
    params: { query: { workspace_id: workspaceId } },
    body: request,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new ToolApprovalGrantsApiError(
      defaultApiErrorMessage("Failed to set remembered decision", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to set remembered decision").grant;
}

/**
 * Revoke one remembered decision in one workspace. A missing or cross-workspace id
 * returns `404`, surfaced as a typed error so the caller can keep the row and report it.
 */
export async function revokeToolApprovalGrant(
  id: string,
  workspaceId: string,
  signal?: AbortSignal
): Promise<void> {
  const { error, response } = await apiClient.DELETE("/api/tool-approval-grants/{id}", {
    params: { path: { id }, query: { workspace_id: workspaceId } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new ToolApprovalGrantsApiError("Remembered decision not found", 404);
    }
    throw new ToolApprovalGrantsApiError(
      defaultApiErrorMessage("Failed to revoke remembered decision", response, error),
      response.status
    );
  }
}
