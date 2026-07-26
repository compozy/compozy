import createClient from "openapi-fetch";

import type { paths as aghPaths } from "@/generated/agh-openapi";
import type { paths as compozyPaths } from "@/generated/compozy-openapi";

export const apiBaseUrl =
  typeof window === "undefined" ? "http://localhost" : window.location.origin;

// openapi-fetch captures the fetch implementation at client creation time.
// Delegate through globalThis.fetch so tests can stub it after module import.
export const runtimeFetch: typeof globalThis.fetch = (input, init) => globalThis.fetch(input, init);

export const apiClient = createClient<aghPaths>({
  baseUrl: apiBaseUrl,
  fetch: runtimeFetch,
});

export const daemonApiClient = createClient<compozyPaths>({
  baseUrl: apiBaseUrl,
  fetch: runtimeFetch,
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
