import {
  apiClientFor,
  apiErrorCode,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
  type ComputerId,
} from "@/lib/api-client";
import { apiErrorDetails, remoteRejectionMessage } from "@/lib/remote-error-copy";

import type { ProviderAuthProbeResponse, ProviderListResponse, ProviderSummary } from "../types";

export class ProvidersApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly code: string | null = null,
    public readonly details?: Readonly<Record<string, string>>
  ) {
    super(message);
    this.name = "ProvidersApiError";
  }
}

function throwProvidersError(response: Response, error: unknown, fallback: string): never {
  const code = apiErrorCode(error) ?? null;
  const details = apiErrorDetails(error);
  const message = remoteRejectionMessage(
    code,
    defaultApiErrorMessage(fallback, response, error),
    details
  );
  throw new ProvidersApiError(message, response.status, code, details);
}

export async function listProviders(
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<ProviderListResponse> {
  const { data, error, response } = await apiClientFor(computerId).GET("/api/providers", {
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwProvidersError(response, error, "Failed to load providers");
  }
  return requireResponseData(data, response, "Failed to load providers");
}

export async function getProvider(
  providerId: string,
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<ProviderSummary> {
  const trimmed = providerId.trim();
  if (trimmed.length === 0) {
    throw new ProvidersApiError("provider_id is required", 400);
  }
  const { data, error, response } = await apiClientFor(computerId).GET(
    "/api/providers/{provider_id}",
    {
      params: { path: { provider_id: trimmed } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwProvidersError(response, error, `Failed to load provider "${trimmed}"`);
  }
  return requireResponseData(data, response, `Failed to load provider "${trimmed}"`);
}

/** Runs the provider's live sign-in check; the daemon keeps the verdict for later reads. */
export async function probeProviderAuth(
  providerId: string,
  computerId?: ComputerId
): Promise<ProviderAuthProbeResponse> {
  const trimmed = providerId.trim();
  if (trimmed.length === 0) {
    throw new ProvidersApiError("provider_id is required", 400);
  }
  const { data, error, response } = await apiClientFor(computerId).POST(
    "/api/providers/{provider_id}/auth/probe",
    {
      params: { path: { provider_id: trimmed } },
    }
  );
  if (apiRequestFailed(response, error)) {
    throwProvidersError(response, error, `Failed to check sign-in for "${trimmed}"`);
  }
  return requireResponseData(data, response, `Failed to check sign-in for "${trimmed}"`);
}
