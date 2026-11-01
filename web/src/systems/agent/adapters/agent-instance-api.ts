import { apiBaseUrl, clientID, runtimeFetch } from "@/lib/api-client";

import type {
  AgentInstanceCheckInsResponse,
  AgentInstancePayload,
  AgentInstancesResponse,
  CreateAgentInstanceParams,
  DraftAgentInstructionsParams,
  DraftAgentInstructionsResponse,
} from "../types";

const LIST_PAGE_LIMIT = 200;

/** A failed instance call with the daemon's stable code, e.g. `agent_instance_limit`. */
export class AgentInstanceApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(message: string, status: number, code = "") {
    super(message);
    this.name = "AgentInstanceApiError";
    this.status = status;
    this.code = code;
  }
}

interface RequestOptions {
  method?: "GET" | "POST" | "DELETE";
  body?: unknown;
  signal?: AbortSignal;
  fallback: string;
}

async function failure(response: Response, fallback: string) {
  const body = (await response.json().catch(() => null)) as {
    code?: string;
    message?: string;
    error?: string;
  } | null;
  return new AgentInstanceApiError(
    body?.message ?? body?.error ?? `${fallback}: ${response.status}`,
    response.status,
    body?.code
  );
}

/**
 * These routes aren't in the generated client yet, so the path and body types come from
 * `../types`. Same headers and base as `apiClient`.
 */
async function request<T>(path: string, options: RequestOptions): Promise<T> {
  const { method = "GET", body, signal, fallback } = options;
  const response = await runtimeFetch(new URL(path, apiBaseUrl), {
    method,
    signal,
    headers: {
      "X-Compozy-Client-ID": clientID,
      ...(body === undefined ? {} : { "Content-Type": "application/json" }),
    },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  });
  if (!response.ok) throw await failure(response, fallback);
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

/** Every non-archived instance the operator can reach; the set is small and read as one list. */
export async function fetchAgentInstances(signal?: AbortSignal): Promise<AgentInstancePayload[]> {
  const instances: AgentInstancePayload[] = [];
  let cursor = "";
  do {
    const query = new URLSearchParams({ all_workspaces: "true", limit: String(LIST_PAGE_LIMIT) });
    if (cursor) query.set("cursor", cursor);
    const page = await request<AgentInstancesResponse>(`/api/agent-instances?${query}`, {
      signal,
      fallback: "Failed to fetch stays-on agents",
    });
    instances.push(...page.instances);
    cursor = page.next_cursor;
  } while (cursor);
  return instances;
}

async function instanceAction(path: string, fallback: string, body?: unknown) {
  const { instance } = await request<{ instance: AgentInstancePayload }>(path, {
    method: "POST",
    body,
    fallback,
  });
  return instance;
}

export function createAgentInstance(params: CreateAgentInstanceParams) {
  return instanceAction("/api/agent-instances", "Failed to start this agent", params);
}

const instancePath = (id: string) => `/api/agent-instances/${encodeURIComponent(id)}`;

export function pauseAgentInstance(id: string) {
  return instanceAction(`${instancePath(id)}/pause`, "Failed to pause this agent");
}

export function resumeAgentInstance(id: string) {
  return instanceAction(`${instancePath(id)}/resume`, "Failed to resume this agent");
}

export function ensureAgentInstanceMain(id: string) {
  return instanceAction(`${instancePath(id)}/ensure-main`, "Failed to open this agent");
}

export async function archiveAgentInstance(id: string): Promise<void> {
  await request<void>(instancePath(id), {
    method: "DELETE",
    fallback: "Failed to archive this agent",
  });
}

export function fetchAgentInstanceCheckIns(id: string, limit: number, signal?: AbortSignal) {
  return request<AgentInstanceCheckInsResponse>(`${instancePath(id)}/check-ins?limit=${limit}`, {
    signal,
    fallback: "Failed to fetch check-ins",
  });
}

export async function draftAgentInstructions(
  params: DraftAgentInstructionsParams,
  signal?: AbortSignal
) {
  const { instructions } = await request<DraftAgentInstructionsResponse>(
    "/api/agents/draft-instructions",
    { method: "POST", body: params, signal, fallback: "Failed to write the instructions" }
  );
  return instructions;
}
