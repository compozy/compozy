import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  KnowledgeSelector,
  KnowledgeListFilter,
  ListMemoryDecisionsParams,
  MemoryDecisionRevertRequest,
  MemoryDecisionRevertResponse,
  MemoryDecisionsResponse,
  MemoryDeleteResponse,
  MemoryDreamTriggerResponse,
  MemoryEditRequest,
  MemoryEditResponse,
  MemoryHeader,
  MemoryListPage,
  MemoryListQuery,
  MemorySearchRequest,
  MemorySearchResponse,
  MemoryWriteRequest,
  MemoryWriteResponse,
} from "../types";

export class KnowledgeApiError extends Error {
  constructor(
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = "KnowledgeApiError";
  }
}

interface SelectorParams {
  scope?: KnowledgeSelector["scope"];
  workspace_id?: string;
  agent_name?: string;
  agent_tier?: KnowledgeSelector["agentTier"];
}

function selectorToQuery(selector: KnowledgeSelector | undefined): SelectorParams {
  if (!selector) return {};
  const params: SelectorParams = { scope: selector.scope };
  if (selector.workspaceId) {
    params.workspace_id = selector.workspaceId;
  }
  if (selector.agentName) {
    params.agent_name = selector.agentName;
  }
  if (selector.agentTier) {
    params.agent_tier = selector.agentTier;
  }
  return params;
}

function listFilterToQuery(filters: KnowledgeListFilter | undefined): MemoryListQuery {
  const selector = selectorToQuery(filters);
  return {
    ...selector,
    type: filters?.type,
    sort: filters?.sort,
    cursor: filters?.cursor?.trim() || undefined,
    limit: filters?.limit,
    include_system: filters?.includeSystem,
  };
}

export async function listMemories(
  filters?: KnowledgeListFilter,
  signal?: AbortSignal
): Promise<MemoryListPage> {
  const { data, error, response } = await apiClient.GET("/api/memory", {
    params: { query: listFilterToQuery(filters) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new KnowledgeApiError(
      defaultApiErrorMessage("Failed to fetch memories", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to fetch memories");
}

export async function readMemory(
  selector: KnowledgeSelector,
  filename: string,
  signal?: AbortSignal
): Promise<MemoryHeader & { content: string }> {
  const { data, error, response } = await apiClient.GET("/api/memory/{filename}", {
    params: {
      path: { filename },
      query: selectorToQuery(selector),
    },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new KnowledgeApiError(`Memory not found: ${filename}`, 404);
    }
    throw new KnowledgeApiError(
      defaultApiErrorMessage(`Failed to read memory "${filename}"`, response, error),
      response.status
    );
  }
  const payload = requireResponseData(data, response, `Failed to read memory "${filename}"`).memory;
  return { ...payload.summary, content: payload.content };
}

export async function writeMemory(
  body: MemoryWriteRequest,
  signal?: AbortSignal
): Promise<MemoryWriteResponse> {
  const { data, error, response } = await apiClient.POST("/api/memory", {
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new KnowledgeApiError(
      defaultApiErrorMessage("Failed to write memory", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to write memory");
}

export async function editMemory(
  filename: string,
  body: MemoryEditRequest,
  signal?: AbortSignal
): Promise<MemoryEditResponse> {
  const { data, error, response } = await apiClient.PATCH("/api/memory/{filename}", {
    params: { path: { filename } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new KnowledgeApiError(`Memory not found: ${filename}`, 404);
    }
    throw new KnowledgeApiError(
      defaultApiErrorMessage(`Failed to edit memory "${filename}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to edit memory "${filename}"`);
}

export async function deleteMemory(
  selector: KnowledgeSelector,
  filename: string,
  signal?: AbortSignal
): Promise<MemoryDeleteResponse> {
  const { data, error, response } = await apiClient.DELETE("/api/memory/{filename}", {
    params: {
      path: { filename },
      query: selectorToQuery(selector),
    },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new KnowledgeApiError(`Memory not found: ${filename}`, 404);
    }
    throw new KnowledgeApiError(
      defaultApiErrorMessage(`Failed to delete memory "${filename}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to delete memory "${filename}"`);
}

export async function searchMemory(
  body: MemorySearchRequest,
  signal?: AbortSignal
): Promise<MemorySearchResponse> {
  const { data, error, response } = await apiClient.POST("/api/memory/search", {
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new KnowledgeApiError(
      defaultApiErrorMessage("Failed to search memory", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to search memory");
}

export async function listMemoryDecisions(
  params: ListMemoryDecisionsParams,
  signal?: AbortSignal
): Promise<MemoryDecisionsResponse> {
  const query = selectorToQuery(params);
  const { data, error, response } = await apiClient.GET("/api/memory/decisions", {
    params: {
      query: {
        ...query,
        ...(params.op ? { op: params.op } : {}),
        ...(params.filename ? { filename: params.filename } : {}),
        ...(params.since ? { since: params.since } : {}),
        ...(typeof params.limit === "number" ? { limit: params.limit } : {}),
      },
    },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new KnowledgeApiError(
      defaultApiErrorMessage("Failed to load memory decisions", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load memory decisions");
}

export async function revertMemoryDecision(
  decisionID: string,
  body: MemoryDecisionRevertRequest = {},
  signal?: AbortSignal
): Promise<MemoryDecisionRevertResponse> {
  const { data, error, response } = await apiClient.POST(
    "/api/memory/decisions/{decision_id}/revert",
    {
      params: { path: { decision_id: decisionID } },
      body,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new KnowledgeApiError(`Memory decision not found: ${decisionID}`, 404);
    }
    throw new KnowledgeApiError(
      defaultApiErrorMessage(`Failed to revert memory decision "${decisionID}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to revert memory decision "${decisionID}"`);
}

export async function triggerMemoryDream(
  workspaceID?: string,
  signal?: AbortSignal
): Promise<MemoryDreamTriggerResponse> {
  const { data, error, response } = await apiClient.POST("/api/memory/dreams/trigger", {
    body: { workspace_id: workspaceID },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new KnowledgeApiError(
      defaultApiErrorMessage("Failed to trigger memory dreaming", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to trigger memory dreaming");
}
