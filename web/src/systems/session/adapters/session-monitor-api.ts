import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";
import type { OperationResponse } from "@/lib/api-contract";

import { throwSessionRequestError } from "./session-api-errors";
import { sessionWorkspacePath } from "./session-workspace-path";

export type SessionMonitorsResponse = OperationResponse<"listSessionMonitors", 200>;
export type SessionMonitorPayload = SessionMonitorsResponse["monitors"][number];
export type SessionMonitorCancelResponse = OperationResponse<"cancelSessionMonitor", 200>;

/** Reads the session's monitors across authorized profiles, active first, newest first. */
export async function fetchSessionMonitors(
  workspaceId: string,
  sessionId: string,
  signal?: AbortSignal
): Promise<SessionMonitorsResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/monitors",
    {
      params: {
        path: { workspace_id: sessionWorkspacePath(workspaceId), session_id: sessionId },
        query: { all_profiles: true },
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to fetch monitors", sessionId);
  }
  return requireResponseData(data, response, "Failed to fetch monitors");
}

/** Cancels one monitor in its session's owner profile; the session is not woken. */
export async function cancelSessionMonitor(
  workspaceId: string,
  sessionId: string,
  profile: string,
  monitorId: string
): Promise<SessionMonitorCancelResponse> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/monitors/{monitor_id}/cancel",
    {
      params: {
        path: {
          workspace_id: sessionWorkspacePath(workspaceId),
          session_id: sessionId,
          monitor_id: monitorId,
        },
        query: { profile },
      },
    }
  );
  if (apiRequestFailed(response, error)) {
    // A 404 here is a settled or unknown monitor, not a missing session.
    throwSessionRequestError(response, error, `Failed to cancel monitor "${monitorId}"`);
  }
  return requireResponseData(data, response, `Failed to cancel monitor "${monitorId}"`);
}
