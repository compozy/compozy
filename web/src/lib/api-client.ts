import createClient from "openapi-fetch";

import type { paths as compozyPaths } from "@/generated/compozy-openapi";
import { reportGatewayAccess } from "./gateway-access-signal";

export const apiBaseUrl =
  typeof window === "undefined" ? "http://localhost" : window.location.origin;

// openapi-fetch captures the fetch implementation at client creation time.
// Delegate through globalThis.fetch so tests can stub it after module import.
export const runtimeFetch: typeof globalThis.fetch = (input, init) => globalThis.fetch(input, init);

export const apiClient = createClient<compozyPaths>({
  baseUrl: apiBaseUrl,
  fetch: runtimeFetch,
});

/**
 * Single chokepoint for "this device's access ended". Only an explicit daemon
 * code on a 401 counts (see `gateway-access-signal`), so ordinary failures and
 * unrelated 401s pass through untouched.
 *
 * The parse is awaited rather than fired and forgotten: `openapi-fetch` awaits
 * `onResponse`, so resolving here guarantees the terminal signal has already
 * landed by the time the failing caller regains control. Otherwise that caller
 * could render — or reuse — protected cached data in the window before the
 * boundary learned the session was over. The body is read from a clone so the
 * calling adapter still receives an unconsumed response.
 */
apiClient.use({
  async onResponse({ response }) {
    if (response.status !== 401) return undefined;
    try {
      reportGatewayAccess(response.status, await response.clone().json());
    } catch {
      // A 401 without a JSON envelope carries no access decision.
    }
    return undefined;
  },
});

export function apiErrorMessage(error: unknown): string | undefined {
  if (typeof error === "string") {
    const normalized = error.trim();
    return normalized === "" ? undefined : normalized;
  }

  if (error == null || typeof error !== "object") {
    return undefined;
  }

  const candidate = Reflect.get(error, "error");
  if (typeof candidate !== "string") {
    return undefined;
  }

  const normalized = candidate.trim();
  return normalized === "" ? undefined : normalized;
}

export function defaultApiErrorMessage(
  fallback: string,
  response: Response,
  error: unknown
): string {
  return apiErrorMessage(error) ?? `${fallback}: ${response.status}`;
}

export function apiRequestFailed(response: Response, error: unknown): boolean {
  return !response.ok || error !== undefined;
}

export function requireResponseData<T>(
  data: T | undefined,
  response: Response,
  fallback: string
): T {
  if (data === undefined) {
    throw new Error(`${fallback}: empty response (${response.status})`);
  }
  return data;
}
