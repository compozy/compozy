import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  AgentHeartbeatHistoryResponse,
  AgentHeartbeatPayload,
  AgentHeartbeatStatusPayload,
  DeleteAgentHeartbeatParams,
  DeleteAgentHeartbeatResponse,
  PutAgentHeartbeatParams,
  PutAgentHeartbeatResponse,
  RollbackAgentHeartbeatParams,
  RollbackAgentHeartbeatResponse,
  ValidateAgentHeartbeatParams,
  ValidateAgentHeartbeatResponse,
  WakeAgentHeartbeatParams,
  WakeAgentHeartbeatResponse,
} from "../types";
import { AgentApiError, AgentDigestConflictError } from "./agent-api";

function workspaceIdQuery(workspaceId?: string | null): { workspace_id: string } | undefined {
  const trimmed = workspaceId?.trim();
  return trimmed ? { workspace_id: trimmed } : undefined;
}

export async function fetchAgentHeartbeat(
  name: string,
  workspaceId?: string | null,
  signal?: AbortSignal
): Promise<AgentHeartbeatPayload> {
  const { data, error, response } = await apiClient.GET("/api/agents/{name}/heartbeat", {
    params: { path: { name }, query: workspaceIdQuery(workspaceId) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new AgentApiError(
      defaultApiErrorMessage(`Failed to fetch heartbeat for "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to fetch heartbeat for "${name}"`);
}

export async function validateAgentHeartbeat(
  name: string,
  params: ValidateAgentHeartbeatParams,
  signal?: AbortSignal
): Promise<ValidateAgentHeartbeatResponse> {
  const { data, error, response } = await apiClient.POST("/api/agents/{name}/heartbeat/validate", {
    params: { path: { name } },
    body: params,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new AgentApiError(
      defaultApiErrorMessage(`Failed to validate heartbeat for "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to validate heartbeat for "${name}"`);
}

export async function putAgentHeartbeat(
  name: string,
  params: PutAgentHeartbeatParams,
  signal?: AbortSignal
): Promise<PutAgentHeartbeatResponse> {
  const { data, error, response } = await apiClient.PUT("/api/agents/{name}/heartbeat", {
    params: { path: { name } },
    body: params,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 409) {
      throw new AgentDigestConflictError(
        defaultApiErrorMessage(`Failed to update heartbeat for "${name}"`, response, error)
      );
    }
    throw new AgentApiError(
      defaultApiErrorMessage(`Failed to update heartbeat for "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to update heartbeat for "${name}"`);
}

export async function deleteAgentHeartbeat(
  name: string,
  params: DeleteAgentHeartbeatParams,
  signal?: AbortSignal
): Promise<DeleteAgentHeartbeatResponse> {
  const { data, error, response } = await apiClient.DELETE("/api/agents/{name}/heartbeat", {
    params: { path: { name } },
    body: params,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 409) {
      throw new AgentDigestConflictError(
        defaultApiErrorMessage(`Failed to delete heartbeat for "${name}"`, response, error)
      );
    }
    throw new AgentApiError(
      defaultApiErrorMessage(`Failed to delete heartbeat for "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to delete heartbeat for "${name}"`);
}

export async function fetchAgentHeartbeatHistory(
  name: string,
  workspaceId?: string | null,
  signal?: AbortSignal
): Promise<AgentHeartbeatHistoryResponse> {
  const { data, error, response } = await apiClient.GET("/api/agents/{name}/heartbeat/history", {
    params: { path: { name }, query: workspaceIdQuery(workspaceId) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new AgentApiError(
      defaultApiErrorMessage(`Failed to fetch heartbeat history for "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to fetch heartbeat history for "${name}"`);
}

export async function rollbackAgentHeartbeat(
  name: string,
  params: RollbackAgentHeartbeatParams,
  signal?: AbortSignal
): Promise<RollbackAgentHeartbeatResponse> {
  const { data, error, response } = await apiClient.POST("/api/agents/{name}/heartbeat/rollback", {
    params: { path: { name } },
    body: params,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 409) {
      throw new AgentDigestConflictError(
        defaultApiErrorMessage(`Failed to rollback heartbeat for "${name}"`, response, error)
      );
    }
    throw new AgentApiError(
      defaultApiErrorMessage(`Failed to rollback heartbeat for "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to rollback heartbeat for "${name}"`);
}

export interface FetchAgentHeartbeatStatusParams {
  workspaceId?: string | null;
  sessionId?: string;
  includeSessionHealth?: boolean;
  includeRecentWakeEvents?: boolean;
}

export async function fetchAgentHeartbeatStatus(
  name: string,
  options: FetchAgentHeartbeatStatusParams = {},
  signal?: AbortSignal
): Promise<AgentHeartbeatStatusPayload> {
  const workspace = options.workspaceId?.trim();
  const { data, error, response } = await apiClient.GET("/api/agents/{name}/heartbeat/status", {
    params: {
      path: { name },
      query: {
        ...(workspace ? { workspace_id: workspace } : {}),
        ...(options.sessionId ? { session_id: options.sessionId } : {}),
        ...(options.includeSessionHealth !== undefined
          ? { include_session_health: options.includeSessionHealth }
          : {}),
        ...(options.includeRecentWakeEvents !== undefined
          ? { include_recent_wake_events: options.includeRecentWakeEvents }
          : {}),
      },
    },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new AgentApiError(
      defaultApiErrorMessage(`Failed to fetch heartbeat status for "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to fetch heartbeat status for "${name}"`);
}

export async function wakeAgentHeartbeat(
  name: string,
  params: WakeAgentHeartbeatParams,
  signal?: AbortSignal
): Promise<WakeAgentHeartbeatResponse> {
  const { data, error, response } = await apiClient.POST("/api/agents/{name}/heartbeat/wake", {
    params: { path: { name } },
    body: params,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new AgentApiError(
      defaultApiErrorMessage(`Failed to wake heartbeat for "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to wake heartbeat for "${name}"`);
}
