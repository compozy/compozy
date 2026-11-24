import {
  apiClient,
  apiErrorCode,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  AddComputerRequest,
  AvailableComputersResponse,
  ComputerChangePayload,
  ComputerListResponse,
  ComputerPayload,
  ComputerSetupAnswerRequest,
  ComputerSetupCanceledPayload,
  ComputerSetupStartedPayload,
  ComputerSetupStatusPayload,
  ComputerSSHHostsResponse,
  ComputerUpdateStartedPayload,
  PatchComputerRequest,
  RemoveComputerResponse,
} from "../types";

function extractErrorDetails(error: unknown): Record<string, string> | undefined {
  if (error == null || typeof error !== "object") return undefined;
  const candidate = Reflect.get(error, "details");
  if (candidate == null || typeof candidate !== "object") return undefined;
  const result: Record<string, string> = {};
  for (const [key, value] of Object.entries(candidate as Record<string, unknown>)) {
    if (typeof value === "string") {
      result[key] = value;
    }
  }
  return Object.keys(result).length > 0 ? result : undefined;
}

export class ComputersApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly code?: string,
    public readonly details?: Record<string, string>
  ) {
    super(message);
    this.name = "ComputersApiError";
  }
}

function throwComputersError(fallback: string, response: Response, error: unknown): never {
  throw new ComputersApiError(
    defaultApiErrorMessage(fallback, response, error),
    response.status,
    apiErrorCode(error),
    extractErrorDetails(error)
  );
}

export async function fetchComputers(signal?: AbortSignal): Promise<ComputerListResponse> {
  const { data, error, response } = await apiClient.GET("/api/computers", { signal });
  if (apiRequestFailed(response, error)) {
    throwComputersError("Failed to fetch computers", response, error);
  }
  return requireResponseData(data, response, "Failed to fetch computers");
}

export async function fetchComputer(id: string, signal?: AbortSignal): Promise<ComputerPayload> {
  const { data, error, response } = await apiClient.GET("/api/computers/{id}", {
    params: { path: { id } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwComputersError(`Failed to fetch computer "${id}"`, response, error);
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
    throwComputersError(`Failed to update computer "${id}"`, response, error);
  }
  return requireResponseData(data, response, `Failed to update computer "${id}"`);
}

export async function fetchAvailableComputers(
  signal?: AbortSignal
): Promise<AvailableComputersResponse> {
  const { data, error, response } = await apiClient.GET("/api/computers/available", { signal });
  if (apiRequestFailed(response, error)) {
    throwComputersError("Failed to fetch available computers", response, error);
  }
  return requireResponseData(data, response, "Failed to fetch available computers");
}

export async function addAccountComputer(
  computerId: string,
  name?: string,
  signal?: AbortSignal
): Promise<ComputerPayload> {
  const body: AddComputerRequest = {
    kind: "account",
    computer_id: computerId,
    ...(name ? { name } : {}),
  };
  const { data, error, response } = await apiClient.POST("/api/computers", { body, signal });
  if (apiRequestFailed(response, error)) {
    throwComputersError("Failed to add computer", response, error);
  }
  const resolved = requireResponseData(data, response, "Failed to add computer");
  if (!("id" in resolved)) {
    throw new ComputersApiError("Unexpected setup response when adding account computer", 500);
  }
  return resolved;
}

export async function startSSHComputerSetup(
  input: {
    host: string;
    user?: string;
    port?: number;
    name?: string;
    trust_host_fingerprint?: string;
  },
  signal?: AbortSignal
): Promise<ComputerSetupStartedPayload> {
  const body: AddComputerRequest = {
    kind: "ssh",
    host: input.host,
    ...(input.user ? { user: input.user } : {}),
    ...(input.port ? { port: input.port } : {}),
    ...(input.name ? { name: input.name } : {}),
    ...(input.trust_host_fingerprint
      ? { trust_host_fingerprint: input.trust_host_fingerprint }
      : {}),
  };
  const { data, error, response } = await apiClient.POST("/api/computers", { body, signal });
  if (apiRequestFailed(response, error)) {
    throwComputersError("Failed to start SSH setup", response, error);
  }
  const resolved = requireResponseData(data, response, "Failed to start SSH setup");
  if (!("setup_id" in resolved)) {
    throw new ComputersApiError("Unexpected computer response when starting SSH setup", 500);
  }
  return resolved;
}

export async function removeComputer(
  id: string,
  options: { unlink?: boolean; uninstall?: boolean } = {},
  signal?: AbortSignal
): Promise<RemoveComputerResponse> {
  const { data, error, response } = await apiClient.DELETE("/api/computers/{id}", {
    params: {
      path: { id },
      query: {
        ...(options.unlink !== undefined ? { unlink: options.unlink } : {}),
        ...(options.uninstall !== undefined ? { uninstall: options.uninstall } : {}),
      },
    },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwComputersError(`Failed to remove computer "${id}"`, response, error);
  }
  return requireResponseData(data, response, `Failed to remove computer "${id}"`);
}

export async function removeComputerRoute(
  id: string,
  routeId: string,
  signal?: AbortSignal
): Promise<void> {
  const { error, response } = await apiClient.DELETE("/api/computers/{id}/routes/{route_id}", {
    params: { path: { id, route_id: routeId } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwComputersError(`Failed to remove route "${routeId}"`, response, error);
  }
}

export async function fetchComputerSSHHosts(
  signal?: AbortSignal
): Promise<ComputerSSHHostsResponse> {
  const { data, error, response } = await apiClient.GET("/api/computers/ssh-hosts", { signal });
  if (apiRequestFailed(response, error)) {
    throwComputersError("Failed to discover SSH hosts", response, error);
  }
  return requireResponseData(data, response, "Failed to discover SSH hosts");
}

export async function fetchComputerSetupStatus(
  setupId: string,
  waitMs?: number,
  signal?: AbortSignal
): Promise<ComputerSetupStatusPayload> {
  const { data, error, response } = await apiClient.GET("/api/computers/setups/{setup_id}", {
    params: {
      path: { setup_id: setupId },
      ...(waitMs !== undefined ? { query: { wait_ms: waitMs } } : {}),
    },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwComputersError("Failed to read SSH setup status", response, error);
  }
  return requireResponseData(data, response, "Failed to read SSH setup status");
}

export async function answerComputerSetup(
  setupId: string,
  body: ComputerSetupAnswerRequest,
  signal?: AbortSignal
): Promise<ComputerSetupStatusPayload> {
  const { data, error, response } = await apiClient.POST(
    "/api/computers/setups/{setup_id}/answer",
    {
      params: { path: { setup_id: setupId } },
      body,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwComputersError("Failed to answer SSH setup prompt", response, error);
  }
  return requireResponseData(data, response, "Failed to answer SSH setup prompt");
}

export async function cancelComputerSetup(
  setupId: string,
  signal?: AbortSignal
): Promise<ComputerSetupCanceledPayload> {
  const { data, error, response } = await apiClient.DELETE("/api/computers/setups/{setup_id}", {
    params: { path: { setup_id: setupId } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwComputersError("Failed to cancel SSH setup", response, error);
  }
  return requireResponseData(data, response, "Failed to cancel SSH setup");
}

export async function startComputerUpdate(
  id: string,
  signal?: AbortSignal
): Promise<ComputerUpdateStartedPayload> {
  const { data, error, response } = await apiClient.POST("/api/computers/{id}/update", {
    params: { path: { id } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwComputersError(`Failed to update computer "${id}"`, response, error);
  }
  return requireResponseData(data, response, `Failed to update computer "${id}"`);
}

export interface ComputersStreamHandlers {
  onComputerChanged?: (change: ComputerChangePayload) => void;
  onComputerSetup?: (change: ComputerChangePayload) => void;
  onReset?: () => void;
  onError?: () => void;
}

function unwrapComputerEventData(raw: string): ComputerChangePayload {
  try {
    const parsed = JSON.parse(raw) as ComputerChangePayload | { data?: ComputerChangePayload };
    if (parsed && typeof parsed === "object" && "data" in parsed && parsed.data) {
      return parsed.data;
    }
    return parsed as ComputerChangePayload;
  } catch {
    return {};
  }
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
    handlers.onComputerChanged?.(unwrapComputerEventData(event.data));
  };
  const onSetup = (event: MessageEvent<string>) => {
    handlers.onComputerSetup?.(unwrapComputerEventData(event.data));
  };
  const onReset = () => {
    handlers.onReset?.();
  };
  const onError = () => {
    handlers.onError?.();
  };
  source.addEventListener("computer.changed", onChanged);
  source.addEventListener("computer.setup", onSetup);
  source.addEventListener("stream_reset", onReset);
  source.addEventListener("error", onError);
  return () => {
    source.removeEventListener("computer.changed", onChanged);
    source.removeEventListener("computer.setup", onSetup);
    source.removeEventListener("stream_reset", onReset);
    source.removeEventListener("error", onError);
    source.close();
  };
}
