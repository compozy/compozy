import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  ComputerChangePayload,
  ComputerListResponse,
  ComputerPayload,
  PatchComputerRequest,
} from "../types";

export class ComputersApiError extends Error {
  constructor(
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = "ComputersApiError";
  }
}

export async function fetchComputers(signal?: AbortSignal): Promise<ComputerListResponse> {
  const { data, error, response } = await apiClient.GET("/api/computers", { signal });
  if (apiRequestFailed(response, error)) {
    throw new ComputersApiError(
      defaultApiErrorMessage("Failed to fetch computers", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to fetch computers");
}

export async function fetchComputer(id: string, signal?: AbortSignal): Promise<ComputerPayload> {
  const { data, error, response } = await apiClient.GET("/api/computers/{id}", {
    params: { path: { id } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new ComputersApiError(
      defaultApiErrorMessage(`Failed to fetch computer "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to fetch computer "${id}"`);
}

export async function patchComputer(
  id: string,
  body: PatchComputerRequest,
  signal?: AbortSignal
): Promise<ComputerPayload> {
  const { data, error, response } = await apiClient.PATCH("/api/computers/{id}", {
    params: { path: { id } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new ComputersApiError(
      defaultApiErrorMessage(`Failed to update computer "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to update computer "${id}"`);
}

export interface ComputersStreamHandlers {
  onComputerChanged: (change: ComputerChangePayload) => void;
  onReset?: () => void;
  onError?: () => void;
}

export function openComputersStream(
  handlers: ComputersStreamHandlers,
  options: { url?: string } = {}
): () => void {
  if (typeof EventSource === "undefined") {
    return () => undefined;
  }
  const source = new EventSource(options.url ?? "/api/computers/stream");
  const onChanged = (event: MessageEvent<string>) => {
    try {
      handlers.onComputerChanged(JSON.parse(event.data) as ComputerChangePayload);
    } catch {
      handlers.onComputerChanged({});
    }
  };
  const onReset = () => {
    handlers.onReset?.();
  };
  const onError = () => {
    handlers.onError?.();
  };
  source.addEventListener("computer.changed", onChanged);
  source.addEventListener("stream_reset", onReset);
  source.addEventListener("error", onError);
  return () => {
    source.removeEventListener("computer.changed", onChanged);
    source.removeEventListener("stream_reset", onReset);
    source.removeEventListener("error", onError);
    source.close();
  };
}
