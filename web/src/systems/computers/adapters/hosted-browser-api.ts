import type { components } from "@compozy/client";

import {
  apiClient,
  apiErrorCode,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

export type BrowserStatusPayload = components["schemas"]["BrowserStatus"];
export type BrowserSessionRequestPayload = components["schemas"]["BrowserSessionRequest"];
export type BrowserSessionResponsePayload = components["schemas"]["BrowserSessionResponse"];
export type BrowserApprovalStatusPayload = components["schemas"]["BrowserApprovalStatus"];
export type BrowserPeersResponsePayload = components["schemas"]["BrowserPeersResponse"];
export type HostedPeerPayload = components["schemas"]["HostedPeer"];

export class HostedBrowserApiError extends Error {
  readonly status: number;
  readonly code: string | undefined;
  readonly body: unknown;

  constructor(message: string, status: number, code: string | undefined, body: unknown) {
    super(message);
    this.name = "HostedBrowserApiError";
    this.status = status;
    this.code = code;
    this.body = body;
  }
}

function throwHostedBrowserError(fallback: string, response: Response, error: unknown): never {
  throw new HostedBrowserApiError(
    defaultApiErrorMessage(fallback, response, error),
    response.status,
    apiErrorCode(error),
    error
  );
}

/** Reads `GET /remote/v1/browser/status` (unauthenticated; reports `active` or `plan_required`). */
export async function fetchHostedBrowserStatus(
  signal?: AbortSignal
): Promise<BrowserStatusPayload> {
  const { data, error, response } = await apiClient.GET("/remote/v1/browser/status", { signal });
  if (apiRequestFailed(response, error)) {
    throwHostedBrowserError("Failed to read hosted computer status", response, error);
  }
  return requireResponseData(data, response, "Failed to read hosted computer status");
}

/** Redeems the one-time browser sign-in code (`POST /remote/v1/browser/session`). */
export async function startHostedBrowserSession(
  body: BrowserSessionRequestPayload,
  signal?: AbortSignal
): Promise<BrowserSessionResponsePayload> {
  const { data, error, response } = await apiClient.POST("/remote/v1/browser/session", {
    params: { header: { DPoP: "" } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwHostedBrowserError("Failed to start browser session", response, error);
  }
  return requireResponseData(data, response, "Failed to start browser session");
}

/** Ends the current browser session without revoking the device (`DELETE /remote/v1/browser/session`). */
export async function endHostedBrowserSession(signal?: AbortSignal): Promise<void> {
  const { error, response } = await apiClient.DELETE("/remote/v1/browser/session", {
    params: { header: { DPoP: "" } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwHostedBrowserError("Failed to sign out browser session", response, error);
  }
}

/** Polls a pending browser device approval (`GET /remote/v1/browser/approvals/{id}`). */
export async function fetchHostedBrowserApproval(
  approvalId: string,
  signal?: AbortSignal
): Promise<BrowserApprovalStatusPayload> {
  const { data, error, response } = await apiClient.GET("/remote/v1/browser/approvals/{id}", {
    params: { path: { id: approvalId }, header: { DPoP: "" } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwHostedBrowserError("Failed to check device approval", response, error);
  }
  return requireResponseData(data, response, "Failed to check device approval");
}

/** Lists other hosted computers on this account (`GET /remote/v1/browser/peers`). */
export async function fetchHostedBrowserPeers(
  signal?: AbortSignal
): Promise<BrowserPeersResponsePayload> {
  const { data, error, response } = await apiClient.GET("/remote/v1/browser/peers", {
    params: { header: { DPoP: "" } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwHostedBrowserError("Failed to list hosted computers", response, error);
  }
  return requireResponseData(data, response, "Failed to list hosted computers");
}
