import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  BridgeDetailResponse,
  BridgeListFilter,
  BridgeResolveTargetRequest,
  BridgeResolveTargetResponse,
  BridgeRoute,
  BridgeSecretBinding,
  BridgeProvider,
  BridgeSecretBindingsResponse,
  BridgeTargetsQuery,
  BridgeTargetsResponse,
  DisableBridgeResponse,
  EnableBridgeResponse,
  BridgesListResponse,
  CreateBridgeRequest,
  CreateBridgeResponse,
  PutBridgeSecretBindingRequest,
  RestartBridgeResponse,
  TestBridgeDeliveryRequest,
  TestBridgeDeliveryResponse,
  UpdateBridgeRequest,
  UpdateBridgeResponse,
} from "../types";

export class BridgesApiError extends Error {
  constructor(
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = "BridgesApiError";
  }
}

function normalizeOptionalText(value?: string | null): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }

  const normalized = value.trim();
  return normalized === "" ? undefined : normalized;
}

function normalizeBridgeListFilter(filters: BridgeListFilter = {}): BridgeListFilter {
  return {
    scope: filters.scope,
    workspace_id: normalizeOptionalText(filters.workspace_id),
    workspace: normalizeOptionalText(filters.workspace),
    q: normalizeOptionalText(filters.q),
    platform: normalizeOptionalText(filters.platform),
    status: filters.status,
    sort: filters.sort,
    cursor: normalizeOptionalText(filters.cursor),
    limit: filters.limit,
  };
}

export async function listBridges(
  filters: BridgeListFilter = {},
  signal?: AbortSignal
): Promise<BridgesListResponse> {
  const { data, error, response } = await apiClient.GET("/api/bridges", {
    params: { query: normalizeBridgeListFilter(filters) },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new BridgesApiError(
      defaultApiErrorMessage("Failed to fetch bridges", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to fetch bridges");
}

export async function listBridgeProviders(signal?: AbortSignal): Promise<BridgeProvider[]> {
  const { data, error, response } = await apiClient.GET("/api/bridges/providers", { signal });

  if (apiRequestFailed(response, error)) {
    throw new BridgesApiError(
      defaultApiErrorMessage("Failed to fetch bridge providers", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to fetch bridge providers").providers;
}

export async function getBridge(id: string, signal?: AbortSignal): Promise<BridgeDetailResponse> {
  const { data, error, response } = await apiClient.GET("/api/bridges/{id}", {
    params: { path: { id } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new BridgesApiError(`Bridge not found: ${id}`, 404);
    }

    throw new BridgesApiError(
      defaultApiErrorMessage(`Failed to load bridge "${id}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to load bridge "${id}"`);
}

export async function listBridgeRoutes(id: string, signal?: AbortSignal): Promise<BridgeRoute[]> {
  const { data, error, response } = await apiClient.GET("/api/bridges/{id}/routes", {
    params: { path: { id } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new BridgesApiError(`Bridge not found: ${id}`, 404);
    }

    throw new BridgesApiError(
      defaultApiErrorMessage(`Failed to load routes for bridge "${id}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to load routes for bridge "${id}"`).routes;
}

function normalizeBridgeTargetsQuery(query: BridgeTargetsQuery = {}): BridgeTargetsQuery {
  return {
    q: normalizeOptionalText(query.q),
    limit: query.limit,
  };
}

export async function listBridgeTargets(
  id: string,
  query: BridgeTargetsQuery = {},
  signal?: AbortSignal
): Promise<BridgeTargetsResponse> {
  const { data, error, response } = await apiClient.GET("/api/bridges/{id}/targets", {
    params: { path: { id }, query: normalizeBridgeTargetsQuery(query) },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new BridgesApiError(`Bridge not found: ${id}`, 404);
    }

    throw new BridgesApiError(
      defaultApiErrorMessage(`Failed to load targets for bridge "${id}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to load targets for bridge "${id}"`);
}

function isBridgeResolveTargetPayload(value: unknown): value is BridgeResolveTargetResponse {
  return Boolean(value && typeof value === "object" && "result" in value);
}

export async function resolveBridgeTarget(
  id: string,
  body: BridgeResolveTargetRequest,
  signal?: AbortSignal
): Promise<BridgeResolveTargetResponse> {
  const { data, error, response } = await apiClient.POST("/api/bridges/{id}/resolve", {
    body,
    params: { path: { id } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (
      (response.status === 404 || response.status === 422) &&
      isBridgeResolveTargetPayload(error)
    ) {
      return error;
    }
    if (response.status === 404) {
      throw new BridgesApiError(`Bridge not found: ${id}`, 404);
    }

    throw new BridgesApiError(
      defaultApiErrorMessage(`Failed to resolve target for bridge "${id}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to resolve target for bridge "${id}"`);
}

export async function listBridgeSecretBindings(
  id: string,
  signal?: AbortSignal
): Promise<BridgeSecretBindingsResponse["bindings"]> {
  const { data, error, response } = await apiClient.GET("/api/bridges/{id}/secret-bindings", {
    params: { path: { id } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new BridgesApiError(`Bridge not found: ${id}`, 404);
    }

    throw new BridgesApiError(
      defaultApiErrorMessage(`Failed to load secret bindings for bridge "${id}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to load secret bindings for bridge "${id}"`)
    .bindings;
}

export async function createBridge(
  data: CreateBridgeRequest,
  signal?: AbortSignal
): Promise<CreateBridgeResponse> {
  const {
    data: responseData,
    error,
    response,
  } = await apiClient.POST("/api/bridges", {
    body: data,
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new BridgesApiError(
      defaultApiErrorMessage("Failed to create bridge", response, error),
      response.status
    );
  }

  return requireResponseData(responseData, response, "Failed to create bridge");
}

export async function updateBridge(
  id: string,
  data: UpdateBridgeRequest,
  signal?: AbortSignal
): Promise<UpdateBridgeResponse> {
  const {
    data: responseData,
    error,
    response,
  } = await apiClient.PATCH("/api/bridges/{id}", {
    body: data,
    params: { path: { id } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new BridgesApiError(`Bridge not found: ${id}`, 404);
    }

    throw new BridgesApiError(
      defaultApiErrorMessage(`Failed to update bridge "${id}"`, response, error),
      response.status
    );
  }

  return requireResponseData(responseData, response, `Failed to update bridge "${id}"`);
}

export async function putBridgeSecretBinding(
  id: string,
  bindingName: string,
  data: PutBridgeSecretBindingRequest,
  signal?: AbortSignal
): Promise<BridgeSecretBinding> {
  const {
    data: responseData,
    error,
    response,
  } = await apiClient.PUT("/api/bridges/{id}/secret-bindings/{binding_name}", {
    body: data,
    params: { path: { binding_name: bindingName, id } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new BridgesApiError(`Bridge not found: ${id}`, 404);
    }

    throw new BridgesApiError(
      defaultApiErrorMessage(
        `Failed to update secret binding "${bindingName}" for bridge "${id}"`,
        response,
        error
      ),
      response.status
    );
  }

  return requireResponseData(
    responseData,
    response,
    `Failed to update secret binding "${bindingName}" for bridge "${id}"`
  ).binding;
}

export async function deleteBridgeSecretBinding(
  id: string,
  bindingName: string,
  signal?: AbortSignal
): Promise<void> {
  const { error, response } = await apiClient.DELETE(
    "/api/bridges/{id}/secret-bindings/{binding_name}",
    {
      params: { path: { binding_name: bindingName, id } },
      signal,
    }
  );

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new BridgesApiError(`Bridge not found: ${id}`, 404);
    }

    throw new BridgesApiError(
      defaultApiErrorMessage(
        `Failed to delete secret binding "${bindingName}" for bridge "${id}"`,
        response,
        error
      ),
      response.status
    );
  }
}

async function postBridgeLifecycle(
  path: "/api/bridges/{id}/disable" | "/api/bridges/{id}/enable" | "/api/bridges/{id}/restart",
  actionLabel: "disable" | "enable" | "restart",
  id: string,
  signal?: AbortSignal
): Promise<BridgeDetailResponse> {
  const {
    data: responseData,
    error,
    response,
  } = await apiClient.POST(path, {
    params: { path: { id } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new BridgesApiError(`Bridge not found: ${id}`, 404);
    }

    throw new BridgesApiError(
      defaultApiErrorMessage(`Failed to ${actionLabel} bridge "${id}"`, response, error),
      response.status
    );
  }

  return requireResponseData(responseData, response, `Failed to ${actionLabel} bridge "${id}"`);
}

export async function enableBridge(
  id: string,
  signal?: AbortSignal
): Promise<EnableBridgeResponse> {
  return postBridgeLifecycle("/api/bridges/{id}/enable", "enable", id, signal);
}

export async function disableBridge(
  id: string,
  signal?: AbortSignal
): Promise<DisableBridgeResponse> {
  return postBridgeLifecycle("/api/bridges/{id}/disable", "disable", id, signal);
}

export async function restartBridge(
  id: string,
  signal?: AbortSignal
): Promise<RestartBridgeResponse> {
  return postBridgeLifecycle("/api/bridges/{id}/restart", "restart", id, signal);
}

export async function testBridgeDelivery(
  id: string,
  data: TestBridgeDeliveryRequest,
  signal?: AbortSignal
): Promise<TestBridgeDeliveryResponse> {
  const {
    data: responseData,
    error,
    response,
  } = await apiClient.POST("/api/bridges/{id}/test-delivery", {
    body: data,
    params: { path: { id } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new BridgesApiError(`Bridge not found: ${id}`, 404);
    }
    if (response.status === 409) {
      throw new BridgesApiError(
        defaultApiErrorMessage(`Bridge "${id}" is unavailable`, response, error),
        409
      );
    }

    throw new BridgesApiError(
      defaultApiErrorMessage(`Failed to test delivery for bridge "${id}"`, response, error),
      response.status
    );
  }

  return requireResponseData(responseData, response, `Failed to test delivery for bridge "${id}"`);
}
