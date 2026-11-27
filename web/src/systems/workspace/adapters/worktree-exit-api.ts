import {
  apiClientFor,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
  type ComputerId,
} from "@/lib/api-client";

import type {
  RunWorktreeExitActionParams,
  WorktreeExitPlanPayload,
  WorktreeInspectionPayload,
  WorktreeStatusPayload,
} from "../types";
import { WorktreeApiError } from "./worktree-api";

export interface WorktreeStatusQuery {
  /** Forces a fresh git read instead of serving the cached status row. */
  refresh?: boolean;
  /** Refreshes the forge tier. User-gesture only — never on mount. */
  forge?: boolean;
}

export async function inspectWorktree(
  workspaceID: string,
  worktreeID: string,
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<WorktreeInspectionPayload> {
  const { data, error, response } = await apiClientFor(computerId).GET(
    "/api/workspaces/{workspace_id}/worktrees/{worktree_id}",
    {
      params: { path: { workspace_id: workspaceID, worktree_id: worktreeID } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new WorktreeApiError(
      defaultApiErrorMessage("Failed to inspect worktree", response, error),
      response.status,
      error
    );
  }

  return requireResponseData(data, response, "Failed to inspect worktree");
}

/**
 * `status.read_error` is a first-class fact, not a transport failure: a 200 with
 * a read error means git could not be read and every dependent action blocks.
 */
export async function fetchWorktreeStatus(
  workspaceID: string,
  worktreeID: string,
  query: WorktreeStatusQuery = {},
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<WorktreeStatusPayload> {
  const hasQuery = query.refresh !== undefined || query.forge !== undefined;
  const { data, error, response } = await apiClientFor(computerId).GET(
    "/api/workspaces/{workspace_id}/worktrees/{worktree_id}/status",
    {
      params: {
        path: { workspace_id: workspaceID, worktree_id: worktreeID },
        query: hasQuery ? { refresh: query.refresh, forge: query.forge } : undefined,
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new WorktreeApiError(
      defaultApiErrorMessage("Failed to read worktree status", response, error),
      response.status,
      error
    );
  }

  return requireResponseData(data, response, "Failed to read worktree status");
}

/**
 * The exit ladder is computed daemon-side. Action labels, the primary position,
 * and every `blocked_reason` are rendered verbatim — the web never re-derives them.
 */
export async function fetchWorktreeExitPlan(
  workspaceID: string,
  worktreeID: string,
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<WorktreeExitPlanPayload> {
  const { data, error, response } = await apiClientFor(computerId).GET(
    "/api/workspaces/{workspace_id}/worktrees/{worktree_id}/exit",
    {
      params: { path: { workspace_id: workspaceID, worktree_id: worktreeID } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new WorktreeApiError(
      defaultApiErrorMessage("Failed to read the exit plan", response, error),
      response.status,
      error
    );
  }

  return requireResponseData(data, response, "Failed to read the exit plan");
}

/** Resolve native PR support at the explicit action-preparation boundary. */
export async function prepareWorktreeExitPlan(
  workspaceID: string,
  worktreeID: string,
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<WorktreeExitPlanPayload> {
  const { data, error, response } = await apiClientFor(computerId).POST(
    "/api/workspaces/{workspace_id}/worktrees/{worktree_id}/exit/prepare",
    {
      params: { path: { workspace_id: workspaceID, worktree_id: worktreeID } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new WorktreeApiError(
      defaultApiErrorMessage("Failed to prepare the exit plan", response, error),
      response.status,
      error
    );
  }
  return requireResponseData(data, response, "Failed to prepare the exit plan");
}

/**
 * Returns the durable operation id. Execution is detached from this request, so
 * progress arrives on the per-worktree stream, never from the response body.
 */
export async function runWorktreeExitAction(
  workspaceID: string,
  worktreeID: string,
  params: RunWorktreeExitActionParams,
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<string> {
  const { data, error, response } = await apiClientFor(computerId).POST(
    "/api/workspaces/{workspace_id}/worktrees/{worktree_id}/exit/actions",
    {
      params: { path: { workspace_id: workspaceID, worktree_id: worktreeID } },
      body: params,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new WorktreeApiError(
      defaultApiErrorMessage("Failed to start the exit action", response, error),
      response.status,
      error
    );
  }

  return requireResponseData(data, response, "Failed to start the exit action").op_id;
}

/** Compare-and-cancel on the exact operation; a finished op is a deterministic no-op. */
export async function cancelWorktreeExitAction(
  workspaceID: string,
  worktreeID: string,
  operationID: string,
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<void> {
  const { error, response } = await apiClientFor(computerId).POST(
    "/api/workspaces/{workspace_id}/worktrees/{worktree_id}/exit/cancel",
    {
      params: { path: { workspace_id: workspaceID, worktree_id: worktreeID } },
      body: { op_id: operationID },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new WorktreeApiError(
      defaultApiErrorMessage("Failed to cancel the exit action", response, error),
      response.status,
      error
    );
  }
}
