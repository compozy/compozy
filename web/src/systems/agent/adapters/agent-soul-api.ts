import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  AgentSoulHistoryResponse,
  AgentSoulPayload,
  DeleteAgentSoulParams,
  DeleteAgentSoulResponse,
  PutAgentSoulParams,
  PutAgentSoulResponse,
  RollbackAgentSoulParams,
  RollbackAgentSoulResponse,
  ValidateAgentSoulParams,
  ValidateAgentSoulResponse,
} from "../types";
import { AgentApiError, AgentDigestConflictError } from "./agent-api";

function workspaceIdQuery(workspaceId?: string | null): { workspace_id: string } | undefined {
  const trimmed = workspaceId?.trim();
  return trimmed ? { workspace_id: trimmed } : undefined;
}

export async function fetchAgentSoul(
  name: string,
  workspaceId?: string | null,
  signal?: AbortSignal
): Promise<AgentSoulPayload> {
  const { data, error, response } = await apiClient.GET("/api/agents/{name}/soul", {
    params: { path: { name }, query: workspaceIdQuery(workspaceId) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new AgentApiError(
      defaultApiErrorMessage(`Failed to fetch soul for "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to fetch soul for "${name}"`);
}

export async function validateAgentSoul(
  name: string,
  params: ValidateAgentSoulParams,
  signal?: AbortSignal
): Promise<ValidateAgentSoulResponse> {
  const { data, error, response } = await apiClient.POST("/api/agents/{name}/soul/validate", {
    params: { path: { name } },
    body: params,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new AgentApiError(
      defaultApiErrorMessage(`Failed to validate soul for "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to validate soul for "${name}"`);
}

export async function putAgentSoul(
  name: string,
  params: PutAgentSoulParams,
  signal?: AbortSignal
): Promise<PutAgentSoulResponse> {
  const { data, error, response } = await apiClient.PUT("/api/agents/{name}/soul", {
    params: { path: { name } },
    body: params,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 409) {
      throw new AgentDigestConflictError(
        defaultApiErrorMessage(`Failed to update soul for "${name}"`, response, error)
      );
    }
    throw new AgentApiError(
      defaultApiErrorMessage(`Failed to update soul for "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to update soul for "${name}"`);
}

export async function deleteAgentSoul(
  name: string,
  params: DeleteAgentSoulParams,
  signal?: AbortSignal
): Promise<DeleteAgentSoulResponse> {
  const { data, error, response } = await apiClient.DELETE("/api/agents/{name}/soul", {
    params: { path: { name } },
    body: params,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 409) {
      throw new AgentDigestConflictError(
        defaultApiErrorMessage(`Failed to delete soul for "${name}"`, response, error)
      );
    }
    throw new AgentApiError(
      defaultApiErrorMessage(`Failed to delete soul for "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to delete soul for "${name}"`);
}

export async function fetchAgentSoulHistory(
  name: string,
  workspaceId?: string | null,
  signal?: AbortSignal
): Promise<AgentSoulHistoryResponse> {
  const { data, error, response } = await apiClient.GET("/api/agents/{name}/soul/history", {
    params: { path: { name }, query: workspaceIdQuery(workspaceId) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new AgentApiError(
      defaultApiErrorMessage(`Failed to fetch soul history for "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to fetch soul history for "${name}"`);
}

export async function rollbackAgentSoul(
  name: string,
  params: RollbackAgentSoulParams,
  signal?: AbortSignal
): Promise<RollbackAgentSoulResponse> {
  const { data, error, response } = await apiClient.POST("/api/agents/{name}/soul/rollback", {
    params: { path: { name } },
    body: params,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 409) {
      throw new AgentDigestConflictError(
        defaultApiErrorMessage(`Failed to rollback soul for "${name}"`, response, error)
      );
    }
    throw new AgentApiError(
      defaultApiErrorMessage(`Failed to rollback soul for "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to rollback soul for "${name}"`);
}
