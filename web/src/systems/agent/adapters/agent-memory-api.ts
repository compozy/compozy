import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  AddAgentMemoryParams,
  AgentMemoryDocument,
  AgentMemoryHistoryResponse,
  PutAgentMemoryParams,
  RemoveAgentMemoryParams,
  RemoveAgentMemoryResponse,
  ReplaceAgentMemoryParams,
  RollbackAgentMemoryParams,
  SearchAgentMemoryParams,
  SearchAgentMemoryResponse,
  ValidateAgentMemoryParams,
  ValidateAgentMemoryResponse,
} from "../types";
import { AgentApiError, AgentDigestConflictError } from "./agent-api";

function memoryParams(name: string, workspace?: string | null, profile?: string) {
  const selector = workspace?.trim();
  return {
    path: { name },
    query: { ...(selector ? { workspace: selector } : {}), ...(profile ? { profile } : {}) },
  };
}

function memoryFailure(fallback: string, response: Response, error: unknown) {
  const message = defaultApiErrorMessage(fallback, response, error);
  // `agent_memory_conflict`: the file moved on, and the caller rereads it.
  return response.status === 409
    ? new AgentDigestConflictError(message)
    : new AgentApiError(message, response.status);
}

export async function fetchAgentMemory(
  name: string,
  workspace?: string | null,
  signal?: AbortSignal,
  profile?: string
): Promise<AgentMemoryDocument> {
  const fallback = `Failed to fetch memory for "${name}"`;
  const { data, error, response } = await apiClient.GET("/api/agents/{name}/memory", {
    params: memoryParams(name, workspace, profile),
    signal,
  });
  if (apiRequestFailed(response, error)) throw memoryFailure(fallback, response, error);
  return requireResponseData(data, response, fallback);
}

export async function putAgentMemory(
  name: string,
  params: PutAgentMemoryParams,
  workspace?: string | null,
  signal?: AbortSignal,
  profile?: string
): Promise<AgentMemoryDocument> {
  const fallback = `Failed to update memory for "${name}"`;
  const { data, error, response } = await apiClient.PUT("/api/agents/{name}/memory", {
    params: memoryParams(name, workspace, profile),
    body: params,
    signal,
  });
  if (apiRequestFailed(response, error)) throw memoryFailure(fallback, response, error);
  return requireResponseData(data, response, fallback);
}

export async function addAgentMemory(
  name: string,
  params: AddAgentMemoryParams,
  workspace?: string | null,
  signal?: AbortSignal,
  profile?: string
): Promise<AgentMemoryDocument> {
  const fallback = `Failed to add a note for "${name}"`;
  const { data, error, response } = await apiClient.POST("/api/agents/{name}/memory/entries", {
    params: memoryParams(name, workspace, profile),
    body: params,
    signal,
  });
  if (apiRequestFailed(response, error)) throw memoryFailure(fallback, response, error);
  return requireResponseData(data, response, fallback);
}

export async function replaceAgentMemory(
  name: string,
  params: ReplaceAgentMemoryParams,
  workspace?: string | null,
  signal?: AbortSignal,
  profile?: string
): Promise<AgentMemoryDocument> {
  const fallback = `Failed to change a note for "${name}"`;
  const { data, error, response } = await apiClient.PATCH("/api/agents/{name}/memory/entries", {
    params: memoryParams(name, workspace, profile),
    body: params,
    signal,
  });
  if (apiRequestFailed(response, error)) throw memoryFailure(fallback, response, error);
  return requireResponseData(data, response, fallback);
}

export async function removeAgentMemory(
  name: string,
  params: RemoveAgentMemoryParams,
  workspace?: string | null,
  signal?: AbortSignal,
  profile?: string
): Promise<RemoveAgentMemoryResponse> {
  const fallback = `Failed to remove a note for "${name}"`;
  const { data, error, response } = await apiClient.DELETE("/api/agents/{name}/memory/entries", {
    params: memoryParams(name, workspace, profile),
    body: params,
    signal,
  });
  if (apiRequestFailed(response, error)) throw memoryFailure(fallback, response, error);
  return requireResponseData(data, response, fallback);
}

export async function searchAgentMemory(
  name: string,
  params: SearchAgentMemoryParams,
  workspace?: string | null,
  signal?: AbortSignal,
  profile?: string
): Promise<SearchAgentMemoryResponse> {
  const fallback = `Failed to search memory for "${name}"`;
  const { data, error, response } = await apiClient.POST("/api/agents/{name}/memory/search", {
    params: memoryParams(name, workspace, profile),
    body: params,
    signal,
  });
  if (apiRequestFailed(response, error)) throw memoryFailure(fallback, response, error);
  return requireResponseData(data, response, fallback);
}

export async function validateAgentMemory(
  name: string,
  params: ValidateAgentMemoryParams,
  workspace?: string | null,
  signal?: AbortSignal,
  profile?: string
): Promise<ValidateAgentMemoryResponse> {
  const fallback = `Failed to validate memory for "${name}"`;
  const { data, error, response } = await apiClient.POST("/api/agents/{name}/memory/validate", {
    params: memoryParams(name, workspace, profile),
    body: params,
    signal,
  });
  if (apiRequestFailed(response, error)) throw memoryFailure(fallback, response, error);
  return requireResponseData(data, response, fallback);
}

export async function fetchAgentMemoryHistory(
  name: string,
  workspace?: string | null,
  signal?: AbortSignal,
  profile?: string
): Promise<AgentMemoryHistoryResponse> {
  const fallback = `Failed to fetch memory history for "${name}"`;
  const { data, error, response } = await apiClient.GET("/api/agents/{name}/memory/history", {
    params: memoryParams(name, workspace, profile),
    signal,
  });
  if (apiRequestFailed(response, error)) throw memoryFailure(fallback, response, error);
  return requireResponseData(data, response, fallback);
}

export async function rollbackAgentMemory(
  name: string,
  params: RollbackAgentMemoryParams,
  workspace?: string | null,
  signal?: AbortSignal,
  profile?: string
): Promise<AgentMemoryDocument> {
  const fallback = `Failed to go back to that version for "${name}"`;
  const { data, error, response } = await apiClient.POST("/api/agents/{name}/memory/rollback", {
    params: memoryParams(name, workspace, profile),
    body: params,
    signal,
  });
  if (apiRequestFailed(response, error)) throw memoryFailure(fallback, response, error);
  return requireResponseData(data, response, fallback);
}
