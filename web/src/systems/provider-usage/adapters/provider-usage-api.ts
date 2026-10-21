import { apiBaseUrl, apiErrorCode, apiErrorMessage, runtimeFetch } from "@/lib/api-client";
import { reportGatewayResponse } from "@/lib/gateway-access-signal";

import type { ProviderUsageEnvelope, ProviderUsageRefreshRequest } from "../types";

// The provider-usage routes are not in the generated client yet, so these calls
// go through the same low-level fetch the window manager uses. Every request of
// this system lives in this file so the switch to `apiClient` stays local.

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
  return send(
    `${PROVIDER_USAGE_PATH}?${new URLSearchParams({ profile }).toString()}`,
    { signal },
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
  return send(
    `${PROVIDER_USAGE_PATH}/providers/${encodeURIComponent(id)}?${new URLSearchParams({ profile }).toString()}`,
    { signal },
    `Failed to load usage for "${id}"`
  );
}

/** Asks the daemon to refresh targets; returns the resulting envelope. */
export async function refreshProviderUsage(
  body: ProviderUsageRefreshRequest,
  signal?: AbortSignal
): Promise<ProviderUsageEnvelope> {
  return send(
    `${PROVIDER_USAGE_PATH}/refresh`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
      signal,
    },
    "Couldn't refresh provider usage"
  );
}

async function send(
  path: string,
  init: RequestInit,
  fallback: string
): Promise<ProviderUsageEnvelope> {
  let response: Response;
  try {
    response = await runtimeFetch(new Request(`${apiBaseUrl}${path}`, init));
  } catch (error) {
    if (init.signal?.aborted) throw error;
    throw new ProviderUsageApiError("Daemon unavailable", 0);
  }
  await reportGatewayResponse(response.clone());
  const payload = await readJson(response);
  if (!response.ok) {
    throw new ProviderUsageApiError(
      errorMessage(payload) ?? `${fallback}: ${response.status}`,
      response.status,
      errorCode(payload)
    );
  }
  if (!isEnvelope(payload)) {
    throw new ProviderUsageApiError(`${fallback}: unexpected response`, response.status);
  }
  return payload;
}

async function readJson(response: Response): Promise<unknown> {
  try {
    return (await response.json()) as unknown;
  } catch {
    return undefined;
  }
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
