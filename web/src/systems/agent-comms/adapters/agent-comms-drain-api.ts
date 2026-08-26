/**
 * Stop subtree — the one control that acts on a whole delegation branch.
 *
 * It is the ordinary session stop route carrying `{subtree: true, reason}`, the
 * same operation `compozy session stop --subtree` and `compozy__session_stop`
 * invoke. The daemon stops every child under the root (depth ≤ 3, cycle-safe),
 * closes open calls with the parent-terminal reason, and preserves completed
 * results. Repeating it is a no-op.
 *
 * The receipt is the daemon's own three numbers. Nothing here derives or
 * estimates them — the tree renders exactly what came back.
 */
import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import { toAgentCommsApiError } from "./agent-comms-api-error";
import type { StopSessionDrainResponse } from "../types";

export async function drainCallSubtree(
  workspaceId: string,
  sessionId: string,
  reason: string,
  profile?: string,
  signal?: AbortSignal
): Promise<StopSessionDrainResponse> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/stop",
    {
      params: {
        path: { workspace_id: workspaceId, session_id: sessionId },
        query: profile ? { profile } : {},
      },
      body: { subtree: true, ...(reason ? { reason } : {}) },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw toAgentCommsApiError(`Failed to stop the subtree under ${sessionId}`, response, error);
  }
  return requireResponseData(
    data,
    response,
    `Failed to stop the subtree under ${sessionId}`
  ) as StopSessionDrainResponse;
}
