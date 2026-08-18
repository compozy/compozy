import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import {
  LoopRequestError,
  LoopsApiError,
  LoopTimetravelError,
  reasonEnvelope,
} from "./loops-api-errors";
import type {
  LoopAmendRequest,
  LoopAmendResult,
  LoopDiff,
  LoopDiffQuery,
  LoopForkRequest,
  LoopForkResult,
  LoopRerunRequest,
  LoopRerunResult,
} from "../types";

interface RunPath {
  workspaceId: string;
  runId: string;
}

interface NodePath extends RunPath {
  nodeId: string;
}

const TIMETRAVEL_REASON_STATUSES = new Set([403, 404, 409, 422]);

function timetravelError(action: string, response: Response, error: unknown): LoopsApiError {
  if (TIMETRAVEL_REASON_STATUSES.has(response.status)) {
    const { code, details } = reasonEnvelope(error);
    return new LoopTimetravelError(
      defaultApiErrorMessage(`Cannot ${action}`, response, error),
      response.status,
      code,
      details
    );
  }
  return new LoopsApiError(
    defaultApiErrorMessage(`Failed to ${action}`, response, error),
    response.status
  );
}

export async function diffLoopRun(
  { workspaceId, runId }: RunPath,
  query: LoopDiffQuery = {},
  signal?: AbortSignal
): Promise<LoopDiff> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/diff",
    {
      params: { path: { workspace_id: workspaceId, run_id: runId }, query },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) throw timetravelError("compare this run", response, error);
  return requireResponseData(data, response, `Failed to compare run "${runId}"`);
}

export async function rerunLoopRun(
  { workspaceId, runId }: RunPath,
  body: LoopRerunRequest,
  signal?: AbortSignal
): Promise<LoopRerunResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/rerun",
    {
      params: { path: { workspace_id: workspaceId, run_id: runId } },
      body,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw timetravelError(`rerun from "${body.from_node}"`, response, error);
  }
  return requireResponseData(data, response, `Failed to rerun run "${runId}"`);
}

export async function forkLoopRun(
  { workspaceId, runId }: RunPath,
  body: LoopForkRequest,
  signal?: AbortSignal
): Promise<LoopForkResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/fork",
    {
      params: { path: { workspace_id: workspaceId, run_id: runId } },
      body,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw timetravelError(`fork from generation ${body.generation}`, response, error);
  }
  return requireResponseData(data, response, `Failed to fork run "${runId}"`);
}

const AMEND_REASON_STATUSES = new Set([403, 409, 422]);

export async function amendLoopNode(
  { workspaceId, runId, nodeId }: NodePath,
  body: LoopAmendRequest,
  signal?: AbortSignal
): Promise<LoopAmendResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes/{node_id}/amend",
    {
      params: { path: { workspace_id: workspaceId, run_id: runId, node_id: nodeId } },
      body,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new LoopsApiError(`Loop run or node not found: ${nodeId}`, 404);
    }
    if (AMEND_REASON_STATUSES.has(response.status)) {
      const { code, details } = reasonEnvelope(error);
      throw new LoopRequestError(
        defaultApiErrorMessage(`Cannot amend node "${nodeId}"`, response, error),
        response.status,
        code,
        details
      );
    }
    throw new LoopsApiError(
      defaultApiErrorMessage(`Failed to amend node "${nodeId}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to amend node "${nodeId}"`);
}
