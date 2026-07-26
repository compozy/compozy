import {
  apiClient,
  apiErrorMessage,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  AgentCatalogFilter,
  AgentCatalogResponse,
  AgentPayload,
  CreateAgentParams,
  DeleteAgentResponse,
  DuplicateAgentParams,
  UpdateAgentParams,
} from "../types";

export async function fetchAgentCatalog(
  params: AgentCatalogFilter,
  signal?: AbortSignal
): Promise<AgentCatalogResponse> {
  const { data, error, response } = await apiClient.GET("/api/agents/catalog", {
    params: { query: params },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new AgentApiError(
      defaultApiErrorMessage("Failed to fetch agent catalog", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to fetch agent catalog");
}

export type AgentApiErrorKind = "digest_conflict" | "target_exists" | "generic";

export class AgentApiError extends Error {
  status: number;
  readonly kind: AgentApiErrorKind;

  constructor(message: string, status: number, kind: AgentApiErrorKind = "generic") {
    super(message);
    this.name = "AgentApiError";
    this.status = status;
    this.kind = kind;
  }
}

export class AgentDigestConflictError extends AgentApiError {
  override readonly kind = "digest_conflict" as const;

  constructor(message: string) {
    super(message, 409, "digest_conflict");
    this.name = "AgentDigestConflictError";
  }
}

export class AgentTargetExistsError extends AgentApiError {
  override readonly kind = "target_exists" as const;

  constructor(message: string) {
    super(message, 409, "target_exists");
    this.name = "AgentTargetExistsError";
  }
}

export function isAgentDigestConflict(error: unknown): error is AgentDigestConflictError {
  return (
    error instanceof AgentDigestConflictError ||
    (error instanceof AgentApiError && error.kind === "digest_conflict" && error.status === 409)
  );
}

export function isAgentTargetExists(error: unknown): error is AgentTargetExistsError {
  return (
    error instanceof AgentTargetExistsError ||
    (error instanceof AgentApiError && error.kind === "target_exists" && error.status === 409)
  );
}

/** Compose fallback context with backend detail and status — never drop the agent name. */
function composeAgentApiErrorMessage(fallback: string, response: Response, error: unknown): string {
  const detail = apiErrorMessage(error);
  if (detail) {
    return `${fallback}: ${detail} (${response.status})`;
  }
  return `${fallback}: ${response.status}`;
}

export async function fetchAgents(
  workspace?: string | null,
  signal?: AbortSignal
): Promise<AgentPayload[]> {
  const { data, error, response } = await apiClient.GET("/api/agents", {
    params: { query: agentWorkspaceQuery(workspace) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new AgentApiError(
      defaultApiErrorMessage("Failed to fetch agents", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to fetch agents").agents;
}

export async function fetchAgent(
  name: string,
  workspace?: string | null,
  signal?: AbortSignal
): Promise<AgentPayload> {
  const { data, error, response } = await apiClient.GET("/api/agents/{name}", {
    params: { path: { name }, query: agentWorkspaceQuery(workspace) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new AgentApiError(`Agent not found: ${name}`, 404);
    }
    throw new AgentApiError(
      defaultApiErrorMessage(`Failed to fetch agent "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to fetch agent "${name}"`).agent;
}

export async function createAgent(
  params: CreateAgentParams,
  signal?: AbortSignal
): Promise<AgentPayload> {
  const { data, error, response } = await apiClient.POST("/api/agents", {
    body: params,
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 409) {
      throw new AgentTargetExistsError(
        defaultApiErrorMessage("Failed to create agent", response, error)
      );
    }
    throw new AgentApiError(
      defaultApiErrorMessage("Failed to create agent", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to create agent").agent;
}

export async function updateAgent(
  name: string,
  params: UpdateAgentParams,
  signal?: AbortSignal
): Promise<AgentPayload> {
  const { data, error, response } = await apiClient.PUT("/api/agents/{name}", {
    params: { path: { name } },
    body: params,
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 409) {
      throw new AgentDigestConflictError(
        composeAgentApiErrorMessage(`Failed to update agent "${name}"`, response, error)
      );
    }
    throw new AgentApiError(
      composeAgentApiErrorMessage(`Failed to update agent "${name}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to update agent "${name}"`).agent;
}

export async function deleteAgent(
  name: string,
  workspace?: string | null,
  signal?: AbortSignal
): Promise<DeleteAgentResponse> {
  const { data, error, response } = await apiClient.DELETE("/api/agents/{name}", {
    params: { path: { name }, query: agentWorkspaceQuery(workspace) },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new AgentApiError(`Agent not found: ${name}`, 404);
    }
    throw new AgentApiError(
      defaultApiErrorMessage(`Failed to delete agent "${name}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to delete agent "${name}"`);
}

export async function duplicateAgent(
  sourceName: string,
  params: DuplicateAgentParams,
  signal?: AbortSignal
): Promise<AgentPayload> {
  const { data, error, response } = await apiClient.POST("/api/agents/{name}/duplicate", {
    params: { path: { name: sourceName } },
    body: params,
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 409) {
      throw new AgentTargetExistsError(
        composeAgentApiErrorMessage(`Failed to duplicate agent "${sourceName}"`, response, error)
      );
    }
    throw new AgentApiError(
      composeAgentApiErrorMessage(`Failed to duplicate agent "${sourceName}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to duplicate agent "${sourceName}"`).agent;
}

export function agentWorkspaceQuery(workspace?: string | null): { workspace: string } | undefined {
  const trimmed = workspace?.trim();
  return trimmed ? { workspace: trimmed } : undefined;
}
