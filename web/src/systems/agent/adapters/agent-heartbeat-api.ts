import {
  apiClient,
  apiErrorCode,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  AgentHeartbeatHistoryResponse,
  AgentHeartbeatScriptApproval,
  AgentHeartbeatSourceCatalog,
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
  signal?: AbortSignal,
  profile?: string
): Promise<AgentHeartbeatPayload> {
  const { data, error, response } = await apiClient.GET("/api/agents/{name}/heartbeat", {
    params: {
      path: { name },
      query: { ...workspaceIdQuery(workspaceId), ...(profile ? { profile } : {}) },
    },
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
  signal?: AbortSignal,
  profile?: string
): Promise<ValidateAgentHeartbeatResponse> {
  const { data, error, response } = await apiClient.POST("/api/agents/{name}/heartbeat/validate", {
    params: { path: { name }, query: profile ? { profile } : undefined },
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
  signal?: AbortSignal,
  profile?: string
): Promise<PutAgentHeartbeatResponse> {
  const { data, error, response } = await apiClient.PUT("/api/agents/{name}/heartbeat", {
    params: { path: { name }, query: profile ? { profile } : undefined },
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
  signal?: AbortSignal,
  profile?: string
): Promise<DeleteAgentHeartbeatResponse> {
  const { data, error, response } = await apiClient.DELETE("/api/agents/{name}/heartbeat", {
    params: { path: { name }, query: profile ? { profile } : undefined },
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
  signal?: AbortSignal,
  profile?: string
): Promise<AgentHeartbeatHistoryResponse> {
  const { data, error, response } = await apiClient.GET("/api/agents/{name}/heartbeat/history", {
    params: {
      path: { name },
      query: { ...workspaceIdQuery(workspaceId), ...(profile ? { profile } : {}) },
    },
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
  signal?: AbortSignal,
  profile?: string
): Promise<RollbackAgentHeartbeatResponse> {
  const { data, error, response } = await apiClient.POST("/api/agents/{name}/heartbeat/rollback", {
    params: { path: { name }, query: profile ? { profile } : undefined },
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
  profile?: string;
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
        ...(options.profile ? { profile: options.profile } : {}),
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
  signal?: AbortSignal,
  profile?: string
): Promise<WakeAgentHeartbeatResponse> {
  const { data, error, response } = await apiClient.POST("/api/agents/{name}/heartbeat/wake", {
    params: { path: { name }, query: profile ? { profile } : undefined },
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

/** The source kinds and scripts the agent's check-ins can read, and the daemon's limits. */
export async function fetchAgentHeartbeatSources(
  name: string,
  workspaceId?: string | null,
  signal?: AbortSignal,
  profile?: string
): Promise<AgentHeartbeatSourceCatalog> {
  const { data, error, response } = await apiClient.GET("/api/agents/{name}/heartbeat/sources", {
    params: {
      path: { name },
      query: { ...workspaceIdQuery(workspaceId), ...(profile ? { profile } : {}) },
    },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new AgentApiError(
      defaultApiErrorMessage(`Failed to fetch check-in sources for "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to fetch check-in sources for "${name}"`);
}

/** A refused script approval with the daemon's code, e.g. `source_script_changed`. */
export class AgentHeartbeatScriptApprovalError extends AgentApiError {
  readonly code: string;

  constructor(message: string, status: number, code = "") {
    super(message, status);
    this.name = "AgentHeartbeatScriptApprovalError";
    this.code = code;
  }
}

/**
 * Approves the exact script bytes the operator saw, named by their sha256. The daemon refuses
 * with 409 `source_script_changed` when the file changed since, and 403 for agent callers.
 */
export async function approveAgentHeartbeatScript(
  name: string,
  script: string,
  sha256: string,
  workspaceId?: string | null,
  signal?: AbortSignal,
  profile?: string
): Promise<AgentHeartbeatScriptApproval> {
  const { data, error, response } = await apiClient.POST(
    "/api/agents/{name}/heartbeat/scripts/{script}/approve",
    {
      params: {
        path: { name, script },
        query: { ...workspaceIdQuery(workspaceId), ...(profile ? { profile } : {}) },
      },
      body: { sha256 },
      signal,
    }
  );
  const fallback = `Failed to approve script "${script}" for "${name}"`;
  if (apiRequestFailed(response, error)) {
    throw new AgentHeartbeatScriptApprovalError(
      defaultApiErrorMessage(fallback, response, error),
      response.status,
      apiErrorCode(error) ?? ""
    );
  }
  return requireResponseData(data, response, fallback);
}

/** Approves the exact check-in authority digest the operator was shown (operator only). */
export async function approveAgentHeartbeatAuthority(
  name: string,
  sha256: string,
  workspaceId?: string | null,
  profile?: string
) {
  const { data, error, response } = await apiClient.POST(
    "/api/agents/{name}/heartbeat/authority/approve",
    {
      params: {
        path: { name },
        query: { ...workspaceIdQuery(workspaceId), ...(profile ? { profile } : {}) },
      },
      body: { sha256 },
    }
  );
  const fallback = `Failed to approve check-in changes for "${name}"`;
  if (apiRequestFailed(response, error)) {
    throw new AgentApiError(defaultApiErrorMessage(fallback, response, error), response.status);
  }
  return requireResponseData(data, response, fallback);
}

/** One tool the agent can see, reduced to what the check-in write list needs. */
export interface AgentToolChoice {
  tool_id: string;
  title: string;
  access: string;
  backend: string;
  server?: string;
}

/** The tools the agent sees in this scope, as the daemon's registry resolves them. */
export async function fetchAgentTools(
  name: string,
  workspaceId?: string | null,
  signal?: AbortSignal,
  profile?: string
): Promise<AgentToolChoice[]> {
  const { data, error, response } = await apiClient.GET("/api/tools", {
    params: {
      query: {
        agent_name: name,
        ...workspaceIdQuery(workspaceId),
        ...(profile ? { profile } : {}),
      },
    },
    signal,
  });
  const fallback = `Failed to list the tools of "${name}"`;
  if (apiRequestFailed(response, error)) {
    throw new AgentApiError(defaultApiErrorMessage(fallback, response, error), response.status);
  }
  return requireResponseData(data, response, fallback).tools.map(({ descriptor }) => ({
    tool_id: descriptor.tool_id,
    title: descriptor.display_title || descriptor.tool_id,
    access: descriptor.access,
    backend: descriptor.backend.kind,
    server: descriptor.backend.mcp_server,
  }));
}
