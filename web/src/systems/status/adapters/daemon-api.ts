import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type { RuntimeIdentityPayload, StatusPayload } from "../types";

export async function fetchStatus(signal?: AbortSignal): Promise<StatusPayload> {
  const { data, error, response } = await apiClient.GET("/api/status", { signal });
  if (apiRequestFailed(response, error)) {
    throw new Error(defaultApiErrorMessage("Runtime status check failed", response, error));
  }
  return requireResponseData(data, response, "Runtime status check failed");
}

export async function fetchRuntimeIdentity(signal?: AbortSignal): Promise<RuntimeIdentityPayload> {
  const { data, error, response } = await apiClient.GET("/api/status/identity", { signal });
  if (apiRequestFailed(response, error)) {
    throw new Error(defaultApiErrorMessage("Runtime identity check failed", response, error));
  }
  return requireResponseData(data, response, "Runtime identity check failed");
}
