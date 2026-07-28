import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  NetworkChannel,
  NetworkChannelsQuery,
  NetworkChannelsResponse,
  NetworkPeerDetail,
  NetworkPeerSummary,
  NetworkStatus,
  NetworkWorkDetail,
} from "../types";
import { NetworkApiError } from "./network-api-error";

export async function getNetworkStatus(signal?: AbortSignal): Promise<NetworkStatus> {
  const { data, error, response } = await apiClient.GET("/api/network/status", { signal });
  if (apiRequestFailed(response, error)) {
    throw new NetworkApiError(
      defaultApiErrorMessage("Failed to fetch network status", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to fetch network status").network;
}

export async function listNetworkChannels(
  workspaceId: string,
  query: NetworkChannelsQuery = {},
  signal?: AbortSignal
): Promise<NetworkChannelsResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/network/channels",
    {
      params: { path: { workspace_id: workspaceId }, query },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new NetworkApiError(
      defaultApiErrorMessage("Failed to fetch network channels", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to fetch network channels");
}

export async function getNetworkChannel(
  workspaceId: string,
  channel: string,
  signal?: AbortSignal
): Promise<NetworkChannel> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/network/channels/{channel}",
    {
      params: { path: { workspace_id: workspaceId, channel } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new NetworkApiError(`Channel not found: ${channel}`, 404);
    }
    throw new NetworkApiError(
      defaultApiErrorMessage(`Failed to load channel "${channel}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to load channel "${channel}"`).channel;
}

export async function getNetworkWork(
  workspaceId: string,
  workId: string,
  signal?: AbortSignal
): Promise<NetworkWorkDetail> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/network/work/{work_id}",
    {
      params: { path: { workspace_id: workspaceId, work_id: workId } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new NetworkApiError(`Network work not found: ${workId}`, 404);
    }
    throw new NetworkApiError(
      defaultApiErrorMessage(`Failed to load network work "${workId}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to load network work "${workId}"`).work;
}

export async function listNetworkPeers(
  workspaceId: string,
  channel?: string,
  signal?: AbortSignal
): Promise<NetworkPeerSummary[]> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/network/peers",
    {
      params: {
        path: { workspace_id: workspaceId },
        query: channel ? { channel } : undefined,
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new NetworkApiError(
      defaultApiErrorMessage("Failed to fetch network peers", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to fetch network peers").peers;
}

export async function getNetworkPeer(
  workspaceId: string,
  peerId: string,
  signal?: AbortSignal
): Promise<NetworkPeerDetail> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/network/peers/{peer_id}",
    {
      params: { path: { workspace_id: workspaceId, peer_id: peerId } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new NetworkApiError(`Peer not found: ${peerId}`, 404);
    }
    throw new NetworkApiError(
      defaultApiErrorMessage(`Failed to load peer "${peerId}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to load peer "${peerId}"`).peer;
}
