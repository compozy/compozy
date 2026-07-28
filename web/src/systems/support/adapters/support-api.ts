import {
  apiBaseUrl,
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
  runtimeFetch,
} from "@/lib/api-client";

import type { CreateSupportBundleRequest, SupportBundleOperation } from "../types";

export class SupportApiError extends Error {
  constructor(
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = "SupportApiError";
  }
}

export async function createSupportBundle(
  body: CreateSupportBundleRequest,
  signal?: AbortSignal
): Promise<SupportBundleOperation> {
  const { data, error, response } = await apiClient.POST("/api/support/bundles", {
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SupportApiError(
      defaultApiErrorMessage("Failed to create support bundle", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to create support bundle").operation;
}

export async function getSupportBundle(
  operationId: string,
  signal?: AbortSignal
): Promise<SupportBundleOperation> {
  const { data, error, response } = await apiClient.GET("/api/support/bundles/{operation_id}", {
    params: { path: { operation_id: operationId } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SupportApiError(
      defaultApiErrorMessage("Failed to load support bundle status", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load support bundle status").operation;
}

export async function downloadSupportBundle(
  operationId: string,
  signal?: AbortSignal
): Promise<Blob> {
  const response = await runtimeFetch(
    `${apiBaseUrl}/api/support/bundles/${encodeURIComponent(operationId)}/download`,
    { signal }
  );
  if (!response.ok) {
    let message = `Failed to download support bundle: ${response.status}`;
    try {
      const payload = (await response.json()) as unknown;
      if (payload && typeof payload === "object") {
        const candidate = Reflect.get(payload, "error");
        if (typeof candidate === "string" && candidate.trim() !== "") {
          message = candidate;
        }
      }
    } catch {
      // Non-JSON download errors fall back to status text.
    }
    throw new SupportApiError(message, response.status);
  }
  return response.blob();
}

export const supportApi = {
  create: createSupportBundle,
  get: getSupportBundle,
  download: downloadSupportBundle,
};
