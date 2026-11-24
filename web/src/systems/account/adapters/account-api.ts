import type { components } from "@compozy/client";

import {
  apiClient,
  apiErrorCode,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

export type AccountStatusPayload = components["schemas"]["AccountStatusResponse"];
export type AccountLoginRequest = components["schemas"]["AccountLoginRequest"];
export type AccountLoginResponse = components["schemas"]["AccountLoginResponse"];
export type AccountLogoutRequest = components["schemas"]["AccountLogoutRequest"];
export type AccountLogoutResponse = components["schemas"]["AccountLogoutResponse"];
export type AccountDevicePayload = components["schemas"]["AccountDevice"];
export type AccountDevicesResponse = components["schemas"]["AccountDevicesResponse"];
export type AccountDevicePatch = components["schemas"]["AccountDevicePatch"];
export type AccountApprovalPayload = components["schemas"]["AccountApproval"];
export type AccountApprovalsResponse = components["schemas"]["AccountApprovalsResponse"];
export type AccountApprovalDecisionRequest =
  components["schemas"]["AccountApprovalDecisionRequest"];
export type ApprovalDeliveryPayload = components["schemas"]["ApprovalDelivery"];
export type AccountSignInProgressPayload = components["schemas"]["AccountSignInProgress"];
export type AccountStateEventPayload = components["schemas"]["AccountStatePayload"];

export type AccountChangedEventData = Omit<components["schemas"]["Change"], "approval"> & {
  state?: string;
  approval?: Partial<components["schemas"]["ApprovalResult"]>;
};

export class AccountApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly code?: string
  ) {
    super(message);
    this.name = "AccountApiError";
  }
}

function throwAccountError(fallback: string, response: Response, error: unknown): never {
  throw new AccountApiError(
    defaultApiErrorMessage(fallback, response, error),
    response.status,
    apiErrorCode(error)
  );
}

export async function fetchAccountStatus(signal?: AbortSignal): Promise<AccountStatusPayload> {
  const { data, error, response } = await apiClient.GET("/api/account", { signal });
  if (apiRequestFailed(response, error)) {
    throwAccountError("Failed to fetch account status", response, error);
  }
  return requireResponseData(data, response, "Failed to fetch account status");
}

export async function startAccountLogin(
  body: AccountLoginRequest,
  signal?: AbortSignal
): Promise<AccountLoginResponse> {
  const { data, error, response } = await apiClient.POST("/api/account/login", { body, signal });
  if (apiRequestFailed(response, error)) {
    throwAccountError("Failed to start sign-in", response, error);
  }
  return requireResponseData(data, response, "Failed to start sign-in");
}

export async function cancelAccountLogin(attemptId?: string, signal?: AbortSignal): Promise<void> {
  const { error, response } = await apiClient.DELETE("/api/account/login", {
    params: attemptId ? { query: { attempt_id: attemptId } } : undefined,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwAccountError("Failed to cancel sign-in", response, error);
  }
}

export async function logoutAccount(
  body: AccountLogoutRequest,
  signal?: AbortSignal
): Promise<AccountLogoutResponse> {
  const { data, error, response } = await apiClient.POST("/api/account/logout", { body, signal });
  if (apiRequestFailed(response, error)) {
    throwAccountError("Failed to sign out", response, error);
  }
  return requireResponseData(data, response, "Failed to sign out");
}

export async function fetchAccountDevices(signal?: AbortSignal): Promise<AccountDevicesResponse> {
  const { data, error, response } = await apiClient.GET("/api/account/devices", { signal });
  if (apiRequestFailed(response, error)) {
    throwAccountError("Failed to fetch account devices", response, error);
  }
  return requireResponseData(data, response, "Failed to fetch account devices");
}

export async function patchAccountDevice(
  id: string,
  body: AccountDevicePatch,
  signal?: AbortSignal
): Promise<AccountDevicePayload> {
  const { data, error, response } = await apiClient.PATCH("/api/account/devices/{id}", {
    params: { path: { id } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwAccountError(`Failed to update device "${id}"`, response, error);
  }
  return requireResponseData(data, response, `Failed to update device "${id}"`);
}

export async function revokeAccountDevice(id: string, signal?: AbortSignal): Promise<void> {
  const { error, response } = await apiClient.DELETE("/api/account/devices/{id}", {
    params: { path: { id } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwAccountError(`Failed to revoke device "${id}"`, response, error);
  }
}

export async function fetchAccountApprovals(
  signal?: AbortSignal
): Promise<AccountApprovalsResponse> {
  const { data, error, response } = await apiClient.GET("/api/account/approvals", { signal });
  if (apiRequestFailed(response, error)) {
    throwAccountError("Failed to fetch pending approvals", response, error);
  }
  return requireResponseData(data, response, "Failed to fetch pending approvals");
}

export async function decideAccountApproval(
  id: string,
  approve: boolean,
  signal?: AbortSignal
): Promise<ApprovalDeliveryPayload> {
  const { data, error, response } = await apiClient.POST("/api/account/approvals/{id}/decision", {
    params: { path: { id } },
    body: { approve },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwAccountError("Failed to send approval decision", response, error);
  }
  return requireResponseData(data, response, "Failed to send approval decision");
}

export interface AccountStreamHandlers {
  onAccountChanged: (data: AccountChangedEventData) => void;
  onReset?: () => void;
  onError?: () => void;
}

/**
 * Opens the local controller's `/api/account/stream` SSE subscription.
 * Emits `account.changed` events (including `evict_remote: true` on sign-out/switch).
 */
export function openAccountStream(
  handlers: AccountStreamHandlers,
  options: { url?: string } = {}
): () => void {
  if (typeof EventSource === "undefined") {
    return () => undefined;
  }
  const source = new EventSource(options.url ?? "/api/account/stream");
  const onChanged = (event: MessageEvent<string>) => {
    try {
      const parsed = JSON.parse(event.data) as
        | AccountChangedEventData
        | { data?: AccountChangedEventData };
      const data =
        parsed && typeof parsed === "object" && "data" in parsed && parsed.data
          ? parsed.data
          : (parsed as AccountChangedEventData);
      handlers.onAccountChanged(data);
    } catch {
      handlers.onAccountChanged({});
    }
  };
  const onReset = () => {
    handlers.onReset?.();
  };
  const onError = () => {
    handlers.onError?.();
  };
  source.addEventListener("account.changed", onChanged);
  source.addEventListener("stream_reset", onReset);
  source.addEventListener("error", onError);
  return () => {
    source.removeEventListener("account.changed", onChanged);
    source.removeEventListener("stream_reset", onReset);
    source.removeEventListener("error", onError);
    source.close();
  };
}

export interface AccountSignInStreamHandlers {
  onProgress?: (data: AccountSignInProgressPayload) => void;
  onState?: (data: AccountStateEventPayload) => void;
  onError?: () => void;
}

/**
 * Opens `/api/account/sign-in/events` to track an interactive device-code sign-in attempt.
 */
export function openAccountSignInStream(
  handlers: AccountSignInStreamHandlers,
  options: { url?: string } = {}
): () => void {
  if (typeof EventSource === "undefined") {
    return () => undefined;
  }
  const source = new EventSource(options.url ?? "/api/account/sign-in/events");
  const onProgress = (event: MessageEvent<string>) => {
    try {
      const parsed = JSON.parse(event.data) as AccountSignInProgressPayload;
      handlers.onProgress?.(parsed);
    } catch {
      // Ignore malformed SSE frames.
    }
  };
  const onState = (event: MessageEvent<string>) => {
    try {
      const parsed = JSON.parse(event.data) as AccountStateEventPayload;
      handlers.onState?.(parsed);
    } catch {
      // Ignore malformed SSE frames.
    }
  };
  const onError = () => {
    handlers.onError?.();
  };
  source.addEventListener("sign_in.progress", onProgress);
  source.addEventListener("account.state", onState);
  source.addEventListener("error", onError);
  return () => {
    source.removeEventListener("sign_in.progress", onProgress);
    source.removeEventListener("account.state", onState);
    source.removeEventListener("error", onError);
    source.close();
  };
}
