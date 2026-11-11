import createClient from "openapi-fetch";

import type { paths as compozyPaths } from "@compozy/client";
import { createClientId } from "./client-id";
import { reportGatewayResponse } from "./gateway-access-signal";

export const apiBaseUrl =
  typeof window === "undefined" ? "http://localhost" : window.location.origin;

export const clientID = `web-${createClientId()}`;

// Trace ids are correlation values, not secrets: without browser crypto (some embedded or
// insecure contexts) fall back to Math.random rather than failing the request.
function randomHex(bytes: number): string {
  const values = new Uint8Array(bytes);
  if (typeof globalThis.crypto?.getRandomValues === "function") {
    globalThis.crypto.getRandomValues(values);
  } else {
    for (let index = 0; index < bytes; index++) values[index] = Math.floor(Math.random() * 256);
  }
  return Array.from(values, value => value.toString(16).padStart(2, "0")).join("");
}

/**
 * A W3C `traceparent` with the sampled flag off (`-00`), so the daemon's sample ratio stays
 * authoritative for traces the web client starts.
 */
export function formatTraceparent(traceId: string = randomHex(16)): string {
  return `00-${traceId}-${randomHex(8)}-00`;
}

/**
 * Scopes one trace id over the daemon requests a single user action issues: pass
 * `headers: action()` on each call. Requests without one get a fresh trace id.
 */
export function startTraceAction(): () => { traceparent: string } {
  const traceId = randomHex(16);
  return () => ({ traceparent: formatTraceparent(traceId) });
}

// Daemon-bound requests carry an unsampled traceparent; any other origin never receives one.
function applyTraceparent(url: string, headers: Headers): void {
  if (new URL(url, apiBaseUrl).origin !== apiBaseUrl) {
    headers.delete("traceparent");
  } else if (!headers.has("traceparent")) {
    headers.set("traceparent", formatTraceparent());
  }
}

/**
 * The web client's only `fetch` for daemon requests: `apiClient`, adapters that call the daemon
 * directly, prompt streaming, and raw `Request`s. It applies the traceparent rule, keeps the
 * caller's `(input, init)` shape, and delegates through `globalThis.fetch` at call time so tests
 * can stub it after module import (openapi-fetch captures its fetch at client creation).
 */
export const daemonFetch: typeof globalThis.fetch = (input, init) => {
  if (input instanceof Request && init === undefined) {
    applyTraceparent(input.url, input.headers);
    return globalThis.fetch(input);
  }
  const url = input instanceof Request ? input.url : String(input);
  const headers = new Headers(
    init?.headers ?? (input instanceof Request ? input.headers : undefined)
  );
  applyTraceparent(url, headers);
  return globalThis.fetch(input, { ...init, headers });
};

export const apiClient = createClient<compozyPaths>({
  baseUrl: apiBaseUrl,
  fetch: daemonFetch,
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
  onRequest({ request }) {
    request.headers.set("X-Compozy-Client-ID", clientID);
    return request;
  },
  async onResponse({ response }) {
    await reportGatewayResponse(response);
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

function apiErrorStringField(error: unknown, key: string): string | undefined {
  if (error == null || typeof error !== "object") {
    return undefined;
  }
  const candidate = Reflect.get(error, key);
  if (typeof candidate !== "string") {
    return undefined;
  }
  const normalized = candidate.trim();
  return normalized === "" ? undefined : normalized;
}

/** The daemon's deterministic error code (`ErrorPayload.code`), when the body carries one. */
export function apiErrorCode(error: unknown): string | undefined {
  return apiErrorStringField(error, "code");
}

/** The turn the daemon reports as active on a fence refusal (`ErrorPayload.current_turn_id`). */
export function apiErrorCurrentTurnId(error: unknown): string | undefined {
  return apiErrorStringField(error, "current_turn_id");
}

/**
 * The queue cap a `queue_full` refusal names (`diagnostic.evidence.queue_cap`),
 * so the composer can say "N of N" and drop the queue affordance while full.
 */
export function apiErrorQueueCap(error: unknown): number | undefined {
  if (error == null || typeof error !== "object") {
    return undefined;
  }
  const diagnostic = Reflect.get(error, "diagnostic");
  if (diagnostic == null || typeof diagnostic !== "object") {
    return undefined;
  }
  const evidence = Reflect.get(diagnostic, "evidence");
  if (evidence == null || typeof evidence !== "object") {
    return undefined;
  }
  const cap = Reflect.get(evidence, "queue_cap");
  return typeof cap === "number" && Number.isInteger(cap) && cap > 0 ? cap : undefined;
}

/**
 * The human message of the structured diagnostic an error body carries
 * (`diagnostic.title` and `diagnostic.message`), rendered as the daemon renders it.
 */
export function apiErrorDiagnosticMessage(error: unknown): string | undefined {
  if (error == null || typeof error !== "object") {
    return undefined;
  }
  const diagnostic = Reflect.get(error, "diagnostic");
  const title = apiErrorStringField(diagnostic, "title");
  const message = apiErrorStringField(diagnostic, "message");
  if (title && message && message !== title) {
    return `${title}: ${message}`;
  }
  return title ?? message;
}
