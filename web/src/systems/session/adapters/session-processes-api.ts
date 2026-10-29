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

/** Stops processes by identity whatever their session's state; the operator aggregate's cleanup. */
export async function terminateProcesses(
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
