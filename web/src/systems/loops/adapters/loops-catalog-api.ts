import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import { LoopsApiError, LoopValidationError } from "./loops-api-errors";
import type {
  CreateLoopRequest,
  LoopCatalogFilter,
  LoopDetail,
  LoopsListResponse,
  PatchLoopRequest,
  ValidateLoopRequest,
  ValidateLoopResult,
} from "../types";

function normalizeOptionalText(value?: string | null): string | undefined {
  if (typeof value !== "string") return undefined;
  const normalized = value.trim();
  return normalized === "" ? undefined : normalized;
}

function normalizeLoopCatalogFilter(filters: LoopCatalogFilter = {}): LoopCatalogFilter {
  return {
    q: normalizeOptionalText(filters.q),
    kind: filters.kind,
    category: normalizeOptionalText(filters.category),
    status: filters.status,
    sort: filters.sort,
    cursor: normalizeOptionalText(filters.cursor),
    limit: filters.limit,
  };
}

export async function listLoops(
  workspaceId: string,
  filters: LoopCatalogFilter = {},
  signal?: AbortSignal
): Promise<LoopsListResponse> {
  const { data, error, response } = await apiClient.GET("/api/workspaces/{workspace_id}/loops", {
    params: {
      path: { workspace_id: workspaceId },
      query: normalizeLoopCatalogFilter(filters),
    },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new LoopsApiError(
      defaultApiErrorMessage("Failed to fetch loops", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to fetch loops");
}

export async function getLoop(
  workspaceId: string,
  name: string,
  signal?: AbortSignal
): Promise<LoopDetail> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/loops/{name}",
    {
      params: { path: { workspace_id: workspaceId, name } },
      signal,
    }
  );

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new LoopsApiError(`Loop not found: ${name}`, 404);
    throw new LoopsApiError(
      defaultApiErrorMessage(`Failed to fetch loop "${name}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to fetch loop "${name}"`).loop;
}

export async function createLoop(
  workspaceId: string,
  body: CreateLoopRequest,
  signal?: AbortSignal
): Promise<LoopDetail> {
  const { data, error, response } = await apiClient.POST("/api/workspaces/{workspace_id}/loops", {
    params: { path: { workspace_id: workspaceId } },
    body,
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 409) {
      throw new LoopsApiError(defaultApiErrorMessage("Loop already exists", response, error), 409);
    }
    throw new LoopsApiError(
      defaultApiErrorMessage("Failed to create loop", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to create loop").loop;
}

function isValidationResult(value: unknown): value is ValidateLoopResult {
  return typeof value === "object" && value !== null && "valid" in value;
}

export async function patchLoop(
  workspaceId: string,
  name: string,
  body: PatchLoopRequest,
  signal?: AbortSignal
): Promise<LoopDetail> {
  const { data, error, response } = await apiClient.PATCH(
    "/api/workspaces/{workspace_id}/loops/{name}",
    {
      params: { path: { workspace_id: workspaceId, name } },
      body,
      signal,
    }
  );

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new LoopsApiError(`Loop not found: ${name}`, 404);
    if (response.status === 409) {
      throw new LoopsApiError(
        defaultApiErrorMessage(`Loop "${name}" was modified by another editor`, response, error),
        409
      );
    }
    if (response.status === 422 && isValidationResult(error)) {
      const count = error.errors?.length ?? 0;
      throw new LoopValidationError(
        `Publish rejected: ${name} has ${count} lint issue${count === 1 ? "" : "s"}`,
        error
      );
    }
    throw new LoopsApiError(
      defaultApiErrorMessage(`Failed to publish loop "${name}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to publish loop "${name}"`).loop;
}

export async function deleteLoop(
  workspaceId: string,
  name: string,
  signal?: AbortSignal
): Promise<void> {
  const { error, response } = await apiClient.DELETE(
    "/api/workspaces/{workspace_id}/loops/{name}",
    {
      params: { path: { workspace_id: workspaceId, name } },
      signal,
    }
  );

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new LoopsApiError(`Loop not found: ${name}`, 404);
    throw new LoopsApiError(
      defaultApiErrorMessage(`Failed to delete loop "${name}"`, response, error),
      response.status
    );
  }
}

export async function validateLoop(
  workspaceId: string,
  name: string,
  body: ValidateLoopRequest,
  signal?: AbortSignal
): Promise<ValidateLoopResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/loops/{name}/validate",
    {
      params: { path: { workspace_id: workspaceId, name } },
      body,
      signal,
    }
  );

  if (response.status === 422 && isValidationResult(error)) return error;
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new LoopsApiError(`Loop not found: ${name}`, 404);
    throw new LoopsApiError(
      defaultApiErrorMessage(`Failed to validate loop "${name}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to validate loop "${name}"`);
}
