import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type { StatusPayload } from "../types";

export async function fetchStatus(signal?: AbortSignal): Promise<StatusPayload> {
  const { data, error, response } = await apiClient.GET("/api/status", { signal });
  if (apiRequestFailed(response, error)) {
    throw new Error(defaultApiErrorMessage("Runtime status check failed", response, error));
  }
  return requireResponseData(data, response, "Runtime status check failed");
}
