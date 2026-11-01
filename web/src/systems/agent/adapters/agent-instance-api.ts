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
  AgentInstanceReportState,
  AgentInstancesResponse,
  AgentInstanceWirePayload,
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

/** Which instance a route acts on, and the profile that owns it: id routes refuse any other. */
export type AgentInstanceRef = Pick<AgentInstancePayload, "id" | "profile_name">;

/** The instances a view can see, with each read profile's own slots. */
export interface AgentInstancesRead {
  instances: AgentInstancePayload[];
  capacity: Readonly<Record<string, AgentInstanceCapacityCount>>;
}

export interface AgentInstanceCapacityCount {
  limit: number;
  active: number;
}

function owned(instance: AgentInstanceWirePayload, profile_name: string) {
  return { ...instance, profile_name };
}

async function fetchProfileInstances(profile: string, signal?: AbortSignal) {
  const instances: AgentInstancePayload[] = [];
  let cursor = "";
  let page: AgentInstancesResponse;
  do {
    const query = new URLSearchParams({
      all_workspaces: "true",
      limit: String(LIST_PAGE_LIMIT),
      profile,
    });
    if (cursor) query.set("cursor", cursor);
    page = await request<AgentInstancesResponse>(`/api/agent-instances?${query}`, {
      signal,
      fallback: "Failed to fetch stays-on agents",
    });
    instances.push(...page.instances.map(instance => owned(instance, profile)));
    cursor = page.next_cursor;
  } while (cursor);
  return {
    profile,
    instances,
    capacity: { limit: page.max_active_instances, active: page.active_count },
  };
}

/**
 * One read per profile: the list route serves a single profile, so a profile's slots stay its own
 * and the aggregate is every profile's read. One unavailable profile doesn't hide the others.
 */
export async function fetchAgentInstances(
  profiles: readonly string[],
  signal?: AbortSignal
): Promise<AgentInstancesRead> {
  const settled = await Promise.allSettled(
    profiles.map(profile => fetchProfileInstances(profile, signal))
  );
  const reads = settled.flatMap(result => (result.status === "fulfilled" ? [result.value] : []));
  if (reads.length === 0 && settled[0]?.status === "rejected") throw settled[0].reason;
  return {
    instances: reads.flatMap(read => read.instances),
    capacity: Object.fromEntries(reads.map(read => [read.profile, read.capacity])),
  };
}

function instancePath(
  { id, profile_name }: AgentInstanceRef,
  suffix = "",
  params: Record<string, string> = {}
) {
  const query = new URLSearchParams({ ...params, profile: profile_name });
  return `/api/agent-instances/${encodeURIComponent(id)}${suffix}?${query}`;
}

async function instanceAction(
  path: string,
  profile: string,
  fallback: string,
  body?: unknown
): Promise<AgentInstancePayload> {
  const { instance } = await request<{ instance: AgentInstanceWirePayload }>(path, {
    method: "POST",
    body,
    fallback,
  });
  return owned(instance, profile);
}

export function createAgentInstance(params: CreateAgentInstanceParams, profile: string) {
  const query = new URLSearchParams({ profile });
  return instanceAction(
    `/api/agent-instances?${query}`,
    profile,
    "Failed to start this agent",
    params
  );
}

export function pauseAgentInstance(ref: AgentInstanceRef) {
  return instanceAction(
    instancePath(ref, "/pause"),
    ref.profile_name,
    "Failed to pause this agent"
  );
}

export function resumeAgentInstance(ref: AgentInstanceRef) {
  return instanceAction(
    instancePath(ref, "/resume"),
    ref.profile_name,
    "Failed to resume this agent"
  );
}

export function ensureAgentInstanceMain(ref: AgentInstanceRef) {
  return instanceAction(
    instancePath(ref, "/ensure-main"),
    ref.profile_name,
    "Failed to open this agent"
  );
}

export async function archiveAgentInstance(ref: AgentInstanceRef): Promise<void> {
  await request<void>(instancePath(ref), {
    method: "DELETE",
    fallback: "Failed to archive this agent",
  });
}

export async function setAgentInstanceChecks(ref: AgentInstanceRef, checksEnabled: boolean) {
  const { instance } = await request<{ instance: AgentInstanceWirePayload }>(instancePath(ref), {
    method: "PATCH",
    body: { checks_enabled: checksEnabled },
    fallback: "Failed to change check-ins",
  });
  return owned(instance, ref.profile_name);
}

export function runAgentInstanceCheck(ref: AgentInstanceRef, dryRun: boolean) {
  return request<AgentInstanceCheckResponse>(instancePath(ref, "/check"), {
    method: "POST",
    body: { dry_run: dryRun },
    fallback: "Failed to run this check",
  });
}

export function fetchAgentInstanceOccurrence(
  ref: AgentInstanceRef,
  occurrence: string,
  signal?: AbortSignal
) {
  return request<AgentInstanceOccurrence>(
    instancePath(ref, `/occurrences/${encodeURIComponent(occurrence)}`),
    { signal, fallback: "Failed to read this check" }
  );
}

const REPORTS_READ_LIMIT = 100;

export async function fetchAgentInstanceReports(
  ref: AgentInstanceRef,
  limit = REPORTS_READ_LIMIT,
  signal?: AbortSignal,
  state?: AgentInstanceReportState
): Promise<AgentInstanceReport[]> {
  const { reports } = await request<AgentInstanceReportsResponse>(
    instancePath(ref, "/reports", { limit: String(limit), ...(state ? { state } : {}) }),
    { signal, fallback: "Failed to fetch this agent's reports" }
  );
  return reports;
}

/** Reads newest-first up to `limit`, following the daemon's page cap. */
export async function fetchAgentInstanceCheckIns(
  ref: AgentInstanceRef,
  limit: number,
  signal?: AbortSignal
): Promise<AgentInstanceCheckInsResponse> {
  const check_ins: AgentInstanceCheckInsResponse["check_ins"] = [];
  let cursor = "";
  do {
    const query: Record<string, string> = {
      limit: String(Math.min(limit - check_ins.length, LIST_PAGE_LIMIT)),
    };
    if (cursor) query.cursor = cursor;
    const page = await request<AgentInstanceCheckInsResponse>(
      instancePath(ref, "/check-ins", query),
      { signal, fallback: "Failed to fetch check-ins" }
    );
    check_ins.push(...page.check_ins);
    cursor = page.next_cursor;
  } while (cursor && check_ins.length < limit);
  return { check_ins, next_cursor: cursor };
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
  ref: AgentInstanceRef,
  signal?: AbortSignal
): Promise<AgentInstanceAway | null> {
  try {
    return await request<AgentInstanceAway>(instancePath(ref, "/away"), {
      signal,
      fallback: "Failed to fetch what happened while you were away",
    });
  } catch (error) {
    if (error instanceof AgentInstanceApiError && error.status === 404) return null;
    throw error;
  }
}

export async function markAgentInstanceSeen(ref: AgentInstanceRef): Promise<void> {
  try {
    await request<void>(instancePath(ref, "/away"), {
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
  profile: string,
  signal?: AbortSignal
) {
  const query = new URLSearchParams({ profile });
  const { instructions } = await request<DraftAgentInstructionsResponse>(
    `/api/agents/draft-instructions?${query}`,
    { method: "POST", body: params, signal, fallback: "Failed to write the instructions" }
  );
  return instructions;
}
