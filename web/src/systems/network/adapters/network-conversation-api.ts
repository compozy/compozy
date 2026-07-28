import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  NetworkConversationMessagesQuery,
  NetworkDirectRoomDetail,
  NetworkDirectRoomMessagesResponse,
  NetworkDirectRoomsResponse,
  NetworkDirectsListQuery,
  NetworkSubscriptionsResponse,
  NetworkThreadDetail,
  NetworkThreadMessagesResponse,
  NetworkThreadsListQuery,
  NetworkThreadsResponse,
} from "../types";
import { NetworkApiError } from "./network-api-error";

export interface NetworkSubscriptionsListQuery {
  session_id?: string;
  thread_id?: string;
}

export async function listNetworkThreads(
  workspaceId: string,
  channel: string,
  query: NetworkThreadsListQuery = {},
  signal?: AbortSignal
): Promise<NetworkThreadsResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/network/channels/{channel}/threads",
    {
      params: { path: { workspace_id: workspaceId, channel }, query },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new NetworkApiError(`Channel not found: ${channel}`, 404);
    }
    throw new NetworkApiError(
      defaultApiErrorMessage(`Failed to load threads for "${channel}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to load threads for "${channel}"`);
}

export async function getNetworkThread(
  workspaceId: string,
  channel: string,
  threadId: string,
  signal?: AbortSignal
): Promise<NetworkThreadDetail> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/network/channels/{channel}/threads/{thread_id}",
    {
      params: { path: { workspace_id: workspaceId, channel, thread_id: threadId } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new NetworkApiError(`Thread not found: ${threadId}`, 404);
    }
    throw new NetworkApiError(
      defaultApiErrorMessage(`Failed to load thread "${threadId}"`, response, error),
      response.status
    );
  }
  const payload = requireResponseData(data, response, `Failed to load thread "${threadId}"`);
  return { ...payload.thread, task_links: payload.task_links ?? [] };
}

export async function listNetworkThreadMessages(
  workspaceId: string,
  channel: string,
  threadId: string,
  query: NetworkConversationMessagesQuery = {},
  signal?: AbortSignal
): Promise<NetworkThreadMessagesResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/network/channels/{channel}/threads/{thread_id}/messages",
    {
      params: { path: { workspace_id: workspaceId, channel, thread_id: threadId }, query },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new NetworkApiError(`Thread not found: ${threadId}`, 404);
    }
    throw new NetworkApiError(
      defaultApiErrorMessage(`Failed to load thread messages for "${threadId}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to load thread messages for "${threadId}"`);
}

export async function listNetworkDirectRooms(
  workspaceId: string,
  channel: string,
  query: NetworkDirectsListQuery = {},
  signal?: AbortSignal
): Promise<NetworkDirectRoomsResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/network/channels/{channel}/directs",
    {
      params: { path: { workspace_id: workspaceId, channel }, query },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new NetworkApiError(`Channel not found: ${channel}`, 404);
    }
    throw new NetworkApiError(
      defaultApiErrorMessage(`Failed to load direct rooms for "${channel}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to load direct rooms for "${channel}"`);
}

export async function getNetworkDirectRoom(
  workspaceId: string,
  channel: string,
  directId: string,
  signal?: AbortSignal
): Promise<NetworkDirectRoomDetail> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/network/channels/{channel}/directs/{direct_id}",
    {
      params: { path: { workspace_id: workspaceId, channel, direct_id: directId } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new NetworkApiError(`Direct room not found: ${directId}`, 404);
    }
    throw new NetworkApiError(
      defaultApiErrorMessage(`Failed to load direct room "${directId}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to load direct room "${directId}"`).direct;
}

export async function listNetworkDirectRoomMessages(
  workspaceId: string,
  channel: string,
  directId: string,
  query: NetworkConversationMessagesQuery = {},
  signal?: AbortSignal
): Promise<NetworkDirectRoomMessagesResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/network/channels/{channel}/directs/{direct_id}/messages",
    {
      params: { path: { workspace_id: workspaceId, channel, direct_id: directId }, query },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new NetworkApiError(`Direct room not found: ${directId}`, 404);
    }
    throw new NetworkApiError(
      defaultApiErrorMessage(`Failed to load direct messages for "${directId}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to load direct messages for "${directId}"`);
}

export async function listNetworkSubscriptions(
  workspaceId: string,
  channel: string,
  query: NetworkSubscriptionsListQuery = {},
  signal?: AbortSignal
): Promise<NetworkSubscriptionsResponse["subscriptions"]> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/network/channels/{channel}/subscriptions",
    {
      params: { path: { workspace_id: workspaceId, channel }, query },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new NetworkApiError(
      defaultApiErrorMessage(`Failed to load subscriptions for "${channel}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to load subscriptions for "${channel}"`)
    .subscriptions;
}
