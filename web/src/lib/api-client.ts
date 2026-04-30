import createClient, { type Middleware } from "openapi-fetch";

import type { paths as daemonPaths } from "@/generated/compozy-openapi";

import { csrfMiddleware } from "./csrf";

export const apiBaseUrl =
  typeof window === "undefined" ? "http://localhost" : window.location.origin;

const runtimeFetch: typeof globalThis.fetch = (input, init) => globalThis.fetch(input, init);

export const daemonApiClient = createClient<daemonPaths>({
  baseUrl: apiBaseUrl,
  fetch: runtimeFetch,
});

daemonApiClient.use(csrfMiddleware as Middleware);

export type DaemonApiClient = typeof daemonApiClient;

export interface TransportErrorShape {
  code: string;
  message: string;
  request_id?: string;
  details?: Record<string, unknown>;
}

export function toTransportError(error: unknown): TransportErrorShape | undefined {
  if (!error || typeof error !== "object") {
    return undefined;
  }
  const code = Reflect.get(error, "code");
  const message = Reflect.get(error, "message");
  if (typeof code !== "string" || typeof message !== "string") {
    const cause = Reflect.get(error, "cause");
    if (cause !== undefined && cause !== error) {
      return toTransportError(cause);
    }
    return undefined;
  }
  const request_id = Reflect.get(error, "request_id");
  const details = Reflect.get(error, "details");
  return {
    code,
    message,
    request_id: typeof request_id === "string" ? request_id : undefined,
    details:
      details && typeof details === "object" && !Array.isArray(details)
        ? (details as Record<string, unknown>)
        : undefined,
  };
}

export function isStaleWorkspaceError(error: unknown): boolean {
  const transport = toTransportError(error);
  return transport?.code === "workspace_context_stale";
}

export function apiErrorMessage(error: unknown, fallback: string): string {
  const transport = toTransportError(error);
  const candidate = transport?.message?.trim();
  if (candidate && candidate.length > 0) {
    return candidate;
  }
  if (error instanceof Error) {
    const message = error.message.trim();
    if (message.length > 0) {
      return message;
    }
  }
  return fallback;
}

export function requireData<T>(
  data: T | undefined,
  response: Response,
  fallback: string,
  error: unknown
): T {
  if (data !== undefined) {
    return data;
  }
  if (!response.ok || error !== undefined) {
    throw new ApiRequestError(apiErrorMessage(error, fallback), response.status, error);
  }
  throw new ApiRequestError(`${fallback}: empty response`, response.status, error);
}

export class ApiRequestError extends Error {
  readonly status: number;
  readonly cause: unknown;
  constructor(message: string, status: number, cause: unknown) {
    super(message);
    this.name = "ApiRequestError";
    this.status = status;
    this.cause = cause;
  }
}
