import {
  apiClient,
  apiErrorCode,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  RemoteDevicePatchRequest,
  RemoteDevicePayload,
  RemoteDevicesResponse,
  RemoteLinkRequest,
  RemoteLinkResponse,
  RemotePatchRequest,
  RemoteStatusPayload,
} from "../types";

export class RemoteHostApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly code?: string
  ) {
    super(message);
    this.name = "RemoteHostApiError";
  }
}

function throwRemoteError(fallback: string, response: Response, error: unknown): never {
  throw new RemoteHostApiError(
    defaultApiErrorMessage(fallback, response, error),
    response.status,
    apiErrorCode(error)
  );
}

export async function fetchRemoteStatus(signal?: AbortSignal): Promise<RemoteStatusPayload> {
  const { data, error, response } = await apiClient.GET("/api/remote", { signal });
  if (apiRequestFailed(response, error)) {
    throwRemoteError("Failed to fetch remote access status", response, error);
  }
  return requireResponseData(data, response, "Failed to fetch remote access status");
}

export async function patchRemoteStatus(
  body: RemotePatchRequest,
  signal?: AbortSignal
): Promise<RemoteStatusPayload> {
  const { data, error, response } = await apiClient.PATCH("/api/remote", { body, signal });
  if (apiRequestFailed(response, error)) {
    throwRemoteError("Failed to update remote access settings", response, error);
  }
  return requireResponseData(data, response, "Failed to update remote access settings");
}

export async function linkRemoteHost(
  body: RemoteLinkRequest = {},
  signal?: AbortSignal
): Promise<RemoteLinkResponse> {
  const { data, error, response } = await apiClient.POST("/api/remote/link", { body, signal });
  if (apiRequestFailed(response, error)) {
    throwRemoteError("Failed to link this computer", response, error);
  }
  return requireResponseData(data, response, "Failed to link this computer");
}

export async function unlinkRemoteHost(signal?: AbortSignal): Promise<void> {
  const { error, response } = await apiClient.DELETE("/api/remote/link", { signal });
  if (apiRequestFailed(response, error)) {
    throwRemoteError("Failed to unlink this computer", response, error);
  }
}

export async function fetchRemoteDevices(signal?: AbortSignal): Promise<RemoteDevicesResponse> {
  const { data, error, response } = await apiClient.GET("/api/remote/devices", { signal });
  if (apiRequestFailed(response, error)) {
    throwRemoteError("Failed to fetch devices allowed on this computer", response, error);
  }
  return requireResponseData(data, response, "Failed to fetch devices allowed on this computer");
}

export async function patchRemoteDevice(
  id: string,
  body: RemoteDevicePatchRequest,
  signal?: AbortSignal
): Promise<RemoteDevicePayload> {
  const { data, error, response } = await apiClient.PATCH("/api/remote/devices/{id}", {
    params: { path: { id } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwRemoteError(`Failed to update device "${id}"`, response, error);
  }
  return requireResponseData(data, response, `Failed to update device "${id}"`);
}

export async function revokeRemoteDevice(id: string, signal?: AbortSignal): Promise<void> {
  const { error, response } = await apiClient.DELETE("/api/remote/devices/{id}", {
    params: { path: { id } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwRemoteError(`Failed to revoke device "${id}"`, response, error);
  }
}
