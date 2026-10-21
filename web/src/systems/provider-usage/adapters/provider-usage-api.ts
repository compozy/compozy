import { apiClient, apiErrorCode, apiErrorMessage } from "@/lib/api-client";

import type { ProviderUsageEnvelope, ProviderUsageRefreshRequest } from "../types";

// Every request of this system lives in this file. Reads and refresh go
// through the typed client; the stream path feeds the ticketed EventSource.

export class ProviderUsageApiError extends Error {
  constructor(
    message: string,
    /** HTTP status; `0` when the daemon could not be reached at all. */
    public readonly status: number,
    /** The daemon's error code (`provider_usage_disabled`, `profile_not_found`, …). */
    public readonly code?: string
  ) {
    super(message);
    this.name = "ProviderUsageApiError";
  }

  /** The request never got an HTTP answer: the daemon is unreachable. */
  get unreachable(): boolean {
    return this.status === 0;
  }
}

export const PROVIDER_USAGE_PATH = "/api/provider-usage";

export function providerUsageStreamPath(profile: string): string {
  return `${PROVIDER_USAGE_PATH}/stream?${new URLSearchParams({ profile }).toString()}`;
}

/** The profile-scoped usage envelope. Reads the daemon cache; never contacts providers. */
export async function getProviderUsage(
  profile: string,
  signal?: AbortSignal
): Promise<ProviderUsageEnvelope> {
  return settle(
    apiClient.GET("/api/provider-usage", { params: { query: { profile } }, signal }),
    signal,
    "Failed to load provider usage"
  );
}

/** One provider's row (`tray` is `null` in a filtered read). */
export async function getProviderUsageProvider(
  profile: string,
  providerId: string,
  signal?: AbortSignal
): Promise<ProviderUsageEnvelope> {
  const id = providerId.trim();
  if (id === "") {
    throw new ProviderUsageApiError("provider_id is required", 400, "invalid_request");
  }
  return settle(
    apiClient.GET("/api/provider-usage/providers/{provider_id}", {
      params: { path: { provider_id: id }, query: { profile } },
      signal,
    }),
    signal,
    `Failed to load usage for "${id}"`
  );
}

/** Asks the daemon to refresh targets; returns the resulting envelope. */
export async function refreshProviderUsage(
  body: ProviderUsageRefreshRequest,
  signal?: AbortSignal
): Promise<ProviderUsageEnvelope> {
  return settle(
    apiClient.POST("/api/provider-usage/refresh", { body, signal }),
    signal,
    "Couldn't refresh provider usage"
  );
}

interface ClientResult {
  data?: unknown;
  error?: unknown;
  response: Response;
}

/** Maps a client call to the envelope or a typed error; no answer at all is "unreachable". */
async function settle(
  request: Promise<ClientResult>,
  signal: AbortSignal | undefined,
  fallback: string
): Promise<ProviderUsageEnvelope> {
  let result: ClientResult;
  try {
    result = await request;
  } catch (error) {
    if (signal?.aborted) throw error;
    throw new ProviderUsageApiError("Daemon unavailable", 0);
  }
  const { data, error, response } = result;
  if (!response.ok || error !== undefined) {
    throw new ProviderUsageApiError(
      errorMessage(error) ?? `${fallback}: ${response.status}`,
      response.status,
      errorCode(error)
    );
  }
  if (!isEnvelope(data)) {
    throw new ProviderUsageApiError(`${fallback}: unexpected response`, response.status);
  }
  return data;
}

/**
 * Usage errors are flat (`{error, code}`); profile errors nest them
 * (`{error: {code, message}}`).
 */
function errorMessage(payload: unknown): string | undefined {
  const nested = nestedError(payload);
  if (nested) return apiErrorStringField(nested, "message");
  return apiErrorMessage(payload);
}

function errorCode(payload: unknown): string | undefined {
  return apiErrorCode(nestedError(payload) ?? payload);
}

function nestedError(payload: unknown): object | undefined {
  if (payload == null || typeof payload !== "object") return undefined;
  const candidate = Reflect.get(payload, "error");
  return candidate != null && typeof candidate === "object" ? candidate : undefined;
}

function apiErrorStringField(source: object, key: string): string | undefined {
  const value = Reflect.get(source, key);
  return typeof value === "string" && value.trim() !== "" ? value.trim() : undefined;
}

function isEnvelope(payload: unknown): payload is ProviderUsageEnvelope {
  return (
    payload != null &&
    typeof payload === "object" &&
    Array.isArray(Reflect.get(payload, "targets")) &&
    typeof Reflect.get(payload, "revision") === "number"
  );
}
