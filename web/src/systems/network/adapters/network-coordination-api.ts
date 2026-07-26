import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";
import type { OperationRequestBody } from "@/lib/api-contract";

import { NetworkApiError } from "./network-api-error";
import type { NetworkUsageQuery } from "../types";

export type NetworkCoordinationResponse = Awaited<ReturnType<typeof getNetworkCoordination>>;
export type NetworkUsageResponse = Awaited<ReturnType<typeof getNetworkUsage>>;

export type NetworkCoordinationRef =
  | { scope: "workspace"; taskId?: never; runId?: string }
  | { scope: "task"; taskId: string; runId?: string };

export type PutNetworkCoordinationRequest = OperationRequestBody<"putNetworkCoordination">;
export type PutNetworkCoordinationInvitationRequest =
  OperationRequestBody<"putNetworkCoordinationInvitation">;

export async function getNetworkCoordination(
  workspaceId: string,
  ref: NetworkCoordinationRef,
  signal?: AbortSignal
) {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/network-coordination",
    {
      params: {
        path: { workspace_id: workspaceId },
        query: {
          scope: ref.scope,
          ...(ref.taskId ? { task_id: ref.taskId } : {}),
          ...(ref.runId ? { run_id: ref.runId } : {}),
        },
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new NetworkApiError(
      defaultApiErrorMessage("Failed to fetch network coordination", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to fetch network coordination").coordination;
}

export async function putNetworkCoordination(
  workspaceId: string,
  body: PutNetworkCoordinationRequest,
  signal?: AbortSignal
) {
  const { data, error, response } = await apiClient.PUT(
    "/api/workspaces/{workspace_id}/network-coordination",
    {
      params: {
        path: { workspace_id: workspaceId },
      },
      body,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new NetworkApiError(
      defaultApiErrorMessage("Failed to update network coordination", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to update network coordination").coordination;
}

export async function putNetworkCoordinationInvitation(
  workspaceId: string,
  body: PutNetworkCoordinationInvitationRequest,
  options: { signal?: AbortSignal } = {}
) {
  const { data, error, response } = await apiClient.PUT(
    "/api/workspaces/{workspace_id}/network-coordination/invitation",
    {
      params: { path: { workspace_id: workspaceId } },
      body,
      signal: options.signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new NetworkApiError(
      defaultApiErrorMessage("Failed to update coordination invitation", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to update coordination invitation")
    .coordination;
}

export async function getNetworkUsage(
  workspaceId: string,
  query: NetworkUsageQuery = {},
  signal?: AbortSignal
) {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/network/usage",
    {
      params: { path: { workspace_id: workspaceId }, query },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new NetworkApiError(
      defaultApiErrorMessage("Failed to fetch network usage", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to fetch network usage");
}
