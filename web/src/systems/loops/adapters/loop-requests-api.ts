import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import {
  inputValidationPayload,
  LoopInputValidationError,
  LoopRequestError,
  LoopsApiError,
  reasonEnvelope,
} from "./loops-api-errors";
import type {
  LoopRequestDetail,
  LoopRequestFilter,
  LoopRequestListResult,
  LoopRespondRequest,
  LoopRespondResult,
} from "../types";

interface RequestPath {
  workspaceId: string;
  runId: string;
  nodeId: string;
  itemIndex?: number;
}

interface RequestDetailPath extends RequestPath {
  generation: number;
}

const REQUEST_ANSWER_STATUSES = new Set([403, 409, 410, 422]);

function requestError(nodeId: string, response: Response, error: unknown): LoopsApiError {
  if (response.status === 404) {
    return new LoopsApiError(`Loop request not found: ${nodeId}`, 404);
  }
  if (REQUEST_ANSWER_STATUSES.has(response.status)) {
    const validation = inputValidationPayload(error);
    if (validation) return new LoopInputValidationError(validation);
    const { code, details } = reasonEnvelope(error);
    return new LoopRequestError(
      defaultApiErrorMessage(`Cannot answer request on "${nodeId}"`, response, error),
      response.status,
      code,
      details
    );
  }
  return new LoopsApiError(
    defaultApiErrorMessage(`Failed to answer request on "${nodeId}"`, response, error),
    response.status
  );
}

export async function listLoopRequests(
  workspaceId: string,
  filters: LoopRequestFilter = {},
  signal?: AbortSignal
): Promise<LoopRequestListResult> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/loop-requests",
    {
      params: { path: { workspace_id: workspaceId }, query: filters },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new LoopsApiError(
      defaultApiErrorMessage("Failed to fetch loop requests", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to fetch loop requests");
}

export async function getLoopRequest(
  { workspaceId, runId, generation, nodeId, itemIndex }: RequestDetailPath,
  signal?: AbortSignal
): Promise<LoopRequestDetail> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes/{node_id}/request",
    {
      params: {
        path: { workspace_id: workspaceId, run_id: runId, node_id: nodeId },
        query: { generation, ...(itemIndex === undefined ? {} : { item_index: itemIndex }) },
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new LoopsApiError(`Loop request not found: ${nodeId}`, 404);
    }
    throw new LoopsApiError(
      defaultApiErrorMessage(`Failed to fetch request on "${nodeId}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to fetch request on "${nodeId}"`);
}

export async function respondLoopRequest(
  { workspaceId, runId, nodeId }: RequestPath,
  body: LoopRespondRequest,
  signal?: AbortSignal
): Promise<LoopRespondResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes/{node_id}/respond",
    {
      params: { path: { workspace_id: workspaceId, run_id: runId, node_id: nodeId } },
      body,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) throw requestError(nodeId, response, error);
  return requireResponseData(data, response, `Failed to answer request on "${nodeId}"`);
}
