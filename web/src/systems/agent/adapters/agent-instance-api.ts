import { apiBaseUrl, clientID, runtimeFetch } from "@/lib/api-client";

import type {
  AgentActivityPayload,
  AgentInstanceAway,
  AgentInstanceCheckInsResponse,
  AgentInstanceCheckResponse,
  AgentInstanceOccurrence,
  AgentInstancePayload,
  AgentInstanceReport,
  AgentInstanceReportsResponse,
  AgentInstancesResponse,
  CreateAgentInstanceParams,
  DraftAgentInstructionsParams,
  DraftAgentInstructionsResponse,
} from "../types";

const LIST_PAGE_LIMIT = 100;

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
  method?: "GET" | "POST" | "PATCH" | "DELETE";
  body?: unknown;
  signal?: AbortSignal;
  fallback: string;
}

/** No route answers yet. A page says so plainly; it never shows made-up data in its place. */
export function isAgentInstanceRouteMissing(error: unknown) {
  return error instanceof AgentInstanceApiError && [404, 405, 501].includes(error.status);
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

export async function setAgentInstanceChecks(id: string, checksEnabled: boolean) {
  const { instance } = await request<{ instance: AgentInstancePayload }>(instancePath(id), {
    method: "PATCH",
    body: { checks_enabled: checksEnabled },
    fallback: "Failed to change check-ins",
  });
  return instance;
}

export function runAgentInstanceCheck(id: string, dryRun: boolean) {
  return request<AgentInstanceCheckResponse>(`${instancePath(id)}/check`, {
    method: "POST",
    body: { dry_run: dryRun },
    fallback: "Failed to run this check",
  });
}

export function fetchAgentInstanceOccurrence(id: string, occurrence: string, signal?: AbortSignal) {
  return request<AgentInstanceOccurrence>(
    `${instancePath(id)}/occurrences/${encodeURIComponent(occurrence)}`,
    { signal, fallback: "Failed to read this check" }
  );
}

const REPORTS_READ_LIMIT = 100;

export async function fetchAgentInstanceReports(
  id: string,
  limit = REPORTS_READ_LIMIT,
  signal?: AbortSignal
): Promise<AgentInstanceReport[]> {
  const { reports } = await request<AgentInstanceReportsResponse>(
    `${instancePath(id)}/reports?limit=${limit}`,
    { signal, fallback: "Failed to fetch this agent's reports" }
  );
  return reports;
}

export function fetchAgentInstanceCheckIns(id: string, limit: number, signal?: AbortSignal) {
  return request<AgentInstanceCheckInsResponse>(`${instancePath(id)}/check-ins?limit=${limit}`, {
    signal,
    fallback: "Failed to fetch check-ins",
  });
}

/** `null` means no route answers yet, which is different from no activity. */
export async function fetchAgentActivity(
  name: string,
  params: { days: number; workspace?: string | null; profile?: string },
  signal?: AbortSignal
): Promise<AgentActivityPayload | null> {
  const query = new URLSearchParams({ days: String(params.days) });
  if (params.workspace) query.set("workspace", params.workspace);
  if (params.profile) query.set("profile", params.profile);
  try {
    return await request<AgentActivityPayload>(
      `/api/agents/${encodeURIComponent(name)}/activity?${query}`,
      { signal, fallback: "Failed to fetch this agent's activity" }
    );
  } catch (error) {
    if (error instanceof AgentInstanceApiError && [404, 501].includes(error.status)) return null;
    throw error;
  }
}

/** `null` until the daemon serves away reads: no route means no digest, never a made-up one. */
export async function fetchAgentInstanceAway(
  id: string,
  signal?: AbortSignal
): Promise<AgentInstanceAway | null> {
  try {
    return await request<AgentInstanceAway>(`/api/agent-instances/${encodeURIComponent(id)}/away`, {
      signal,
      fallback: "Failed to fetch what happened while you were away",
    });
  } catch (error) {
    if (error instanceof AgentInstanceApiError && error.status === 404) return null;
    throw error;
  }
}

export async function markAgentInstanceSeen(id: string): Promise<void> {
  try {
    await request<void>(`/api/agent-instances/${encodeURIComponent(id)}/away`, {
      method: "POST",
      fallback: "Failed to mark this agent as seen",
    });
  } catch (error) {
    if (error instanceof AgentInstanceApiError && error.status === 404) return;
    throw error;
  }
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
