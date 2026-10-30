import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";
import type { OperationRequestBody, OperationResponse } from "@/lib/api-contract";

import { throwSessionRequestError } from "./session-api-errors";
import { sessionWorkspacePath } from "./session-workspace-path";

export type SessionProcessesResponse = OperationResponse<"listSessionProcesses", 200>;
export type SessionProcessPayload = SessionProcessesResponse["items"][number];
export type SessionProcessKind = SessionProcessPayload["kind"];
export type SessionProcessTarget = Pick<SessionProcessPayload, "pid" | "started_at">;
export type TerminateSessionProcessesRequest = OperationRequestBody<"terminateSessionProcesses">;
export type TerminateSessionProcessesResponse = OperationResponse<"terminateSessionProcesses", 200>;
export type SessionProcessResult = TerminateSessionProcessesResponse["results"][number];
export type SessionProcessOutcome = SessionProcessResult["outcome"];
export type ProcessesResponse = OperationResponse<"listProcesses", 200>;
export type SessionProcessGroupPayload = ProcessesResponse["groups"][number];
export type TerminateProcessesResponse = OperationResponse<"terminateProcesses", 200>;

/** What a stop, archive, or delete does with the session's running processes. */
export type SessionProcessPolicy = "keep" | "terminate";

/** Reads the processes this session started that are still running, newest scan first. */
export async function fetchSessionProcesses(
  workspaceId: string,
  sessionId: string,
  profile: string,
  signal?: AbortSignal
): Promise<SessionProcessesResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/processes",
    {
      params: {
        path: { workspace_id: sessionWorkspacePath(workspaceId), session_id: sessionId },
        query: { profile },
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to fetch processes", sessionId);
  }
  return requireResponseData(data, response, "Failed to fetch processes");
}

/**
 * Stops processes the session started. The daemon re-verifies each identity
 * before it signals and answers once per target.
 */
export async function terminateSessionProcesses(
  workspaceId: string,
  sessionId: string,
  profile: string,
  request: TerminateSessionProcessesRequest
): Promise<TerminateSessionProcessesResponse> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/processes/terminate",
    {
      params: {
        path: { workspace_id: sessionWorkspacePath(workspaceId), session_id: sessionId },
        query: { profile },
      },
      body: request,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to stop processes", sessionId);
  }
  return requireResponseData(data, response, "Failed to stop processes");
}

/** Every process left running by a stopped, archived, deleted, or unknown session. */
export async function fetchLeftRunningProcesses(signal?: AbortSignal): Promise<ProcessesResponse> {
  const { data, error, response } = await apiClient.GET("/api/processes", {
    params: { query: { all_workspaces: true, all_profiles: true, orphaned: true } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to fetch processes left running");
  }
  return requireResponseData(data, response, "Failed to fetch processes left running");
}

/** The most targets the operator route accepts in one request. */
export const TERMINATE_PROCESSES_MAX_TARGETS = 100;

async function terminateProcessesBatch(
  targets: readonly SessionProcessTarget[]
): Promise<TerminateProcessesResponse> {
  const { data, error, response } = await apiClient.POST("/api/processes/terminate", {
    body: { targets: targets.map(({ pid, started_at }) => ({ pid, started_at })) },
  });
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to stop processes");
  }
  return requireResponseData(data, response, "Failed to stop processes");
}

/**
 * Stops processes by identity whatever their session's state; the operator
 * aggregate's cleanup. Targets go in batches the route accepts, sent one after
 * another, and the per-target results come back as one answer. A batch the daemon refuses reads as `failed`
 * for each of its targets, so the rest still report; only when every batch is
 * refused does the first refusal reject.
 */
export async function terminateProcesses(
  targets: readonly SessionProcessTarget[]
): Promise<TerminateProcessesResponse> {
  const batches: SessionProcessTarget[][] = [];
  for (let start = 0; start < targets.length; start += TERMINATE_PROCESSES_MAX_TARGETS) {
    batches.push(targets.slice(start, start + TERMINATE_PROCESSES_MAX_TARGETS));
  }
  // One batch at a time: each stop signals, waits out a grace period, and
  // re-verifies identities on the daemon, so batches must not pile up there.
  const settled: PromiseSettledResult<TerminateProcessesResponse>[] = [];
  for (const batch of batches) {
    try {
      settled.push({ status: "fulfilled", value: await terminateProcessesBatch(batch) });
    } catch (reason) {
      settled.push({ status: "rejected", reason });
    }
  }
  const firstRefusal = settled.find(outcome => outcome.status === "rejected");
  if (firstRefusal && settled.every(outcome => outcome.status === "rejected")) {
    throw firstRefusal.reason;
  }
  return settled.reduce<TerminateProcessesResponse>(
    (combined, outcome, index) => {
      if (outcome.status === "fulfilled") {
        return {
          supported: combined.supported && outcome.value.supported,
          results: [...combined.results, ...outcome.value.results],
        };
      }
      const reason =
        outcome.reason instanceof Error && outcome.reason.message
          ? outcome.reason.message
          : "Failed to stop processes";
      const refused = batches[index]!.map(({ pid, started_at }) => ({
        pid,
        started_at,
        outcome: "failed" as const,
        error: reason,
      }));
      return { ...combined, results: [...combined.results, ...refused] };
    },
    { supported: true, results: [] }
  );
}
