import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  CreateNetworkChannelRequest,
  CreateNetworkChannelResponse,
  NetworkChannelUpdateRequest,
  NetworkChannelUpdateResponse,
  NetworkDirectRoomDetail,
  NetworkResolveDirectRoomRequest,
  NetworkSendRequest,
  NetworkSendResponse,
  NetworkSubscriptionRequest,
  NetworkSubscriptionResponse,
  PromoteNetworkThreadTaskRequest,
  PromoteNetworkThreadTaskResponse,
} from "../types";
import { NetworkApiError } from "./network-api-error";

export async function createNetworkChannel(
  workspaceId: string,
  body: CreateNetworkChannelRequest,
  signal?: AbortSignal
): Promise<CreateNetworkChannelResponse> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/network/channels",
    {
      params: { path: { workspace_id: workspaceId } },
      body: { ...body, workspace_id: workspaceId },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new NetworkApiError(
      defaultApiErrorMessage("Failed to create network channel", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to create network channel");
}

export async function updateNetworkChannel(
  workspaceId: string,
  channel: string,
  body: NetworkChannelUpdateRequest,
  signal?: AbortSignal
): Promise<NetworkChannelUpdateResponse["channel"]> {
  const { data, error, response } = await apiClient.PATCH(
    "/api/workspaces/{workspace_id}/network/channels/{channel}",
    {
      params: { path: { workspace_id: workspaceId, channel } },
      body,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new NetworkApiError(`Channel not found: ${channel}`, 404);
    }
    throw new NetworkApiError(
      defaultApiErrorMessage(`Failed to update channel "${channel}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to update channel "${channel}"`).channel;
}

export async function sendNetworkMessage(
  workspaceId: string,
  body: NetworkSendRequest,
  signal?: AbortSignal
): Promise<NetworkSendResponse> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/network/send",
    {
      params: { path: { workspace_id: workspaceId } },
      body: { ...body, workspace_id: workspaceId },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new NetworkApiError(
      defaultApiErrorMessage("Failed to send network message", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to send network message");
}

export async function resolveNetworkDirectRoom(
  workspaceId: string,
  channel: string,
  body: NetworkResolveDirectRoomRequest,
  signal?: AbortSignal
): Promise<NetworkDirectRoomDetail> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/network/channels/{channel}/directs/resolve",
    {
      params: { path: { workspace_id: workspaceId, channel } },
      body,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new NetworkApiError(
      defaultApiErrorMessage(`Failed to resolve direct room in "${channel}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to resolve direct room in "${channel}"`)
    .direct;
}

export async function promoteNetworkThreadTask(
  workspaceId: string,
  channel: string,
  threadId: string,
  body: PromoteNetworkThreadTaskRequest,
  signal?: AbortSignal
): Promise<PromoteNetworkThreadTaskResponse> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/network/channels/{channel}/threads/{thread_id}/promote-task",
    {
      params: { path: { workspace_id: workspaceId, channel, thread_id: threadId } },
      body,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new NetworkApiError(
      defaultApiErrorMessage(`Failed to promote thread "${threadId}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to promote thread "${threadId}"`);
}

export async function upsertNetworkSubscription(
  workspaceId: string,
  channel: string,
  body: NetworkSubscriptionRequest,
  signal?: AbortSignal
): Promise<NetworkSubscriptionResponse["subscription"]> {
  const { data, error, response } = await apiClient.PUT(
    "/api/workspaces/{workspace_id}/network/channels/{channel}/subscriptions",
    {
      params: { path: { workspace_id: workspaceId, channel } },
      body,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new NetworkApiError(
      defaultApiErrorMessage(`Failed to update subscription for "${channel}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to update subscription for "${channel}"`)
    .subscription;
}

export async function deleteNetworkSubscription(
  workspaceId: string,
  channel: string,
  sessionId: string,
  query: { thread_id?: string } = {},
  signal?: AbortSignal
): Promise<void> {
  const { error, response } = await apiClient.DELETE(
    "/api/workspaces/{workspace_id}/network/channels/{channel}/subscriptions/{session_id}",
    {
      params: {
        path: { workspace_id: workspaceId, channel, session_id: sessionId },
        query,
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new NetworkApiError(
      defaultApiErrorMessage(`Failed to delete subscription for "${channel}"`, response, error),
      response.status
    );
  }
}
