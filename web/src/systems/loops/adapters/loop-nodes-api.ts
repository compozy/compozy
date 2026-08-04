import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import { LoopLifecycleConflictError, LoopsApiError } from "./loops-api-errors";
import type {
  LoopNodeInventoryFilter,
  LoopNodeInventoryResponse,
  LoopNodeMutationRequest,
  LoopNodeMutationResult,
  LoopNodePauseRequest,
  LoopNodeResumeRequest,
} from "../types";

/**
 * The node lifecycle API service (task_07 contract). Every verb posts to
 * `/loop-runs/{run_id}/nodes/{node_id}/<verb>` and returns the shared structured
 * answer, so the UI never has to guess what committed: `ok`, the node's post-commit
 * `control`, and the provenance the daemon recorded.
 *
 * Conflict status codes are preserved on the thrown error (409 idempotent /
 * already-in-that-state, 422 invalid transition) because the run page renders
 * those as *information*, not as failures — see `loopControlAnswer`.
 */

interface NodePath {
  workspaceId: string;
  runId: string;
  nodeId: string;
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null ? (value as Record<string, unknown>) : null;
}

/** Reads the daemon's `{code, details}` reason envelope off a rejected response. */
function reasonEnvelope(error: unknown): { code: string; details: Record<string, string> } {
  const body = asRecord(error);
  const code = typeof body?.code === "string" ? body.code : "";
  const rawDetails = asRecord(body?.details);
  const details: Record<string, string> = {};
  for (const [key, value] of Object.entries(rawDetails ?? {})) {
    if (typeof value === "string") details[key] = value;
  }
  return { code, details };
}

function nodeControlError(
  verb: string,
  nodeId: string,
  response: Response,
  error: unknown
): LoopsApiError {
  if (response.status === 404) {
    return new LoopsApiError(`Loop run or node not found: ${nodeId}`, 404);
  }
  // 409 and 422 are the daemon's deterministic answers, not transport failures.
  // Keep the reason code and its details so the page can say which state won.
  if (response.status === 409 || response.status === 422) {
    const { code, details } = reasonEnvelope(error);
    return new LoopLifecycleConflictError(
      defaultApiErrorMessage(`Cannot ${verb} node "${nodeId}"`, response, error),
      response.status,
      code,
      details
    );
  }
  return new LoopsApiError(
    defaultApiErrorMessage(`Failed to ${verb} node "${nodeId}"`, response, error),
    response.status
  );
}

export async function listLoopNodes(
  workspaceId: string,
  filters: LoopNodeInventoryFilter,
  signal?: AbortSignal
): Promise<LoopNodeInventoryResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/loop-nodes",
    {
      params: { path: { workspace_id: workspaceId }, query: filters },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new LoopsApiError(
      defaultApiErrorMessage(`Failed to fetch ${filters.state} nodes`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to fetch ${filters.state} nodes`);
}

export async function pauseLoopNode(
  { workspaceId, runId, nodeId }: NodePath,
  body: LoopNodePauseRequest,
  signal?: AbortSignal
): Promise<LoopNodeMutationResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes/{node_id}/pause",
    {
      params: { path: { workspace_id: workspaceId, run_id: runId, node_id: nodeId } },
      body,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) throw nodeControlError("pause", nodeId, response, error);
  return requireResponseData(data, response, `Failed to pause node "${nodeId}"`);
}

export async function resumeLoopNode(
  { workspaceId, runId, nodeId }: NodePath,
  body: LoopNodeResumeRequest,
  signal?: AbortSignal
): Promise<LoopNodeMutationResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes/{node_id}/resume",
    {
      params: { path: { workspace_id: workspaceId, run_id: runId, node_id: nodeId } },
      body,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) throw nodeControlError("resume", nodeId, response, error);
  return requireResponseData(data, response, `Failed to resume node "${nodeId}"`);
}

export async function cancelLoopNode(
  { workspaceId, runId, nodeId }: NodePath,
  body: LoopNodeMutationRequest = {},
  signal?: AbortSignal
): Promise<LoopNodeMutationResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes/{node_id}/cancel",
    {
      params: { path: { workspace_id: workspaceId, run_id: runId, node_id: nodeId } },
      body,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) throw nodeControlError("cancel", nodeId, response, error);
  return requireResponseData(data, response, `Failed to cancel node "${nodeId}"`);
}

export async function killLoopNode(
  { workspaceId, runId, nodeId }: NodePath,
  body: LoopNodeMutationRequest = {},
  signal?: AbortSignal
): Promise<LoopNodeMutationResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes/{node_id}/kill",
    {
      params: { path: { workspace_id: workspaceId, run_id: runId, node_id: nodeId } },
      body,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) throw nodeControlError("kill", nodeId, response, error);
  return requireResponseData(data, response, `Failed to kill node "${nodeId}"`);
}

export async function requeueLoopNode(
  { workspaceId, runId, nodeId }: NodePath,
  body: LoopNodeMutationRequest = {},
  signal?: AbortSignal
): Promise<LoopNodeMutationResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes/{node_id}/requeue",
    {
      params: { path: { workspace_id: workspaceId, run_id: runId, node_id: nodeId } },
      body,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) throw nodeControlError("requeue", nodeId, response, error);
  return requireResponseData(data, response, `Failed to requeue node "${nodeId}"`);
}
