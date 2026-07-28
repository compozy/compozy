import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type { ProviderAuthProbeResponse, ProviderListResponse, ProviderSummary } from "../types";

export class ProvidersApiError extends Error {
  constructor(
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = "ProvidersApiError";
  }
}

export async function listProviders(signal?: AbortSignal): Promise<ProviderListResponse> {
  const { data, error, response } = await apiClient.GET("/api/providers", { signal });
  if (apiRequestFailed(response, error)) {
    throw new ProvidersApiError(
      defaultApiErrorMessage("Failed to load providers", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load providers");
}

export async function getProvider(
  providerId: string,
  signal?: AbortSignal
): Promise<ProviderSummary> {
  const trimmed = providerId.trim();
  if (trimmed.length === 0) {
    throw new ProvidersApiError("provider_id is required", 400);
  }
  const { data, error, response } = await apiClient.GET("/api/providers/{provider_id}", {
    params: { path: { provider_id: trimmed } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new ProvidersApiError(
      defaultApiErrorMessage(`Failed to load provider "${trimmed}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to load provider "${trimmed}"`);
}

export async function probeProviderAuth(providerId: string): Promise<ProviderAuthProbeResponse> {
  const trimmed = providerId.trim();
  if (trimmed.length === 0) {
    throw new ProvidersApiError("provider_id is required", 400);
  }
  const { data, error, response } = await apiClient.POST(
    "/api/providers/{provider_id}/auth/probe",
    {
      params: { path: { provider_id: trimmed } },
    }
  );
  if (apiRequestFailed(response, error)) {
    throw new ProvidersApiError(
      defaultApiErrorMessage(`Failed to probe provider "${trimmed}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to probe provider "${trimmed}"`);
}
