import type { components } from "@compozy/client";

import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

export type AccountStatusPayload = components["schemas"]["AccountStatusResponse"];

export interface AccountChangedEventData {
  evict_remote?: boolean;
  plan?: string;
  state?: string;
  approval?: { id?: string; result?: string };
}

export class AccountApiError extends Error {
  constructor(
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = "AccountApiError";
  }
}

export async function fetchAccountStatus(signal?: AbortSignal): Promise<AccountStatusPayload> {
  const { data, error, response } = await apiClient.GET("/api/account", { signal });
  if (apiRequestFailed(response, error)) {
    throw new AccountApiError(
      defaultApiErrorMessage("Failed to fetch account status", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to fetch account status");
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
      const parsed = JSON.parse(event.data) as AccountChangedEventData;
      handlers.onAccountChanged(parsed);
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
