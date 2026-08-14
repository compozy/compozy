import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";
import type { OperationRequestBody } from "@/lib/api-contract";

import type { WorktreePayload, WorktreesResponse } from "../types";

export type CreateWorktreeParams = OperationRequestBody<"createWorktree">;
export type AdoptWorktreeParams = OperationRequestBody<"adoptWorktree">;

/**
 * Carries the raw response body so `lib/worktree-refusal.ts` can decode the
 * machine-readable 409 refusal without re-issuing the request.
 */
export class WorktreeApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly body: unknown
  ) {
    super(message);
    this.name = "WorktreeApiError";
  }
}

export async function fetchWorktrees(
  workspaceID: string,
  options: { refresh?: boolean } = {},
  signal?: AbortSignal
): Promise<WorktreesResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/worktrees",
    {
      params: {
        path: { workspace_id: workspaceID },
        query: options.refresh === undefined ? undefined : { refresh: options.refresh },
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new WorktreeApiError(
      defaultApiErrorMessage("Failed to fetch worktrees", response, error),
      response.status,
      error
    );
  }

  return requireResponseData(data, response, "Failed to fetch worktrees");
}

/**
 * Returns the durable pending row. Materialization continues daemon-side after
 * the 202, so dismissing the dialog never aborts the create.
 */
export async function createWorktree(
  workspaceID: string,
  params: CreateWorktreeParams,
  signal?: AbortSignal
): Promise<WorktreePayload> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/worktrees",
    {
      params: { path: { workspace_id: workspaceID } },
      body: params,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new WorktreeApiError(
      defaultApiErrorMessage("Failed to create worktree", response, error),
      response.status,
      error
    );
  }

  return requireResponseData(data, response, "Failed to create worktree").worktree;
}

export async function cancelWorktreeCreate(
  workspaceID: string,
  worktreeID: string,
  signal?: AbortSignal
): Promise<void> {
  const { error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/worktrees/{worktree_id}/cancel",
    {
      params: { path: { workspace_id: workspaceID, worktree_id: worktreeID } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new WorktreeApiError(
      defaultApiErrorMessage("Failed to cancel worktree creation", response, error),
      response.status,
      error
    );
  }
}

/**
 * Idempotent. Adopting a registered path whose record is `missing` revalidates
 * Git identity and restores that record to `ready` — this is the "It's back"
 * resolution path (US-028 EC-1); a different repository stays refused.
 */
export async function adoptWorktree(
  workspaceID: string,
  params: AdoptWorktreeParams,
  signal?: AbortSignal
): Promise<WorktreePayload> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/worktrees/adopt",
    {
      params: { path: { workspace_id: workspaceID } },
      body: params,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new WorktreeApiError(
      defaultApiErrorMessage("Failed to adopt worktree", response, error),
      response.status,
      error
    );
  }

  return requireResponseData(data, response, "Failed to adopt worktree").worktree;
}

/** Without `force`, a dirty or unpushed worktree refuses with a 409 risk payload. */
export async function removeWorktree(
  workspaceID: string,
  worktreeID: string,
  options: { force?: boolean } = {},
  signal?: AbortSignal
): Promise<void> {
  const { error, response } = await apiClient.DELETE(
    "/api/workspaces/{workspace_id}/worktrees/{worktree_id}",
    {
      params: {
        path: { workspace_id: workspaceID, worktree_id: worktreeID },
        query: options.force === undefined ? undefined : { force: options.force },
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new WorktreeApiError(
      defaultApiErrorMessage("Failed to remove worktree", response, error),
      response.status,
      error
    );
  }
}

/** Drops the tombstone record only. Sessions and run history are preserved. */
export async function dismissWorktree(
  workspaceID: string,
  worktreeID: string,
  signal?: AbortSignal
): Promise<void> {
  const { error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/worktrees/{worktree_id}/dismiss",
    {
      params: { path: { workspace_id: workspaceID, worktree_id: worktreeID } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new WorktreeApiError(
      defaultApiErrorMessage("Failed to dismiss worktree", response, error),
      response.status,
      error
    );
  }
}
