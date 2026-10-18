import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";
import type { OperationQuery, OperationResponse } from "@/lib/api-contract";

import { throwSessionRequestError } from "./session-api-errors";

export type SubagentListQuery = Omit<
  OperationQuery<"listSessionSubagents">,
  "profile" | "all_profiles"
>;
export type SubagentListResponse = OperationResponse<"listSessionSubagents", 200>;
export type SubagentPayload = SubagentListResponse["subagents"][number];

export type SubagentWorktreePayload = NonNullable<SubagentPayload["worktree"]>;
export type SubagentCancelResponse = OperationResponse<"cancelSubagent", 202>;

/** Reads one page of direct subagents across authorized profiles, newest first. */
export async function fetchSessionSubagents(
  workspaceId: string,
  sessionId: string,
  query: SubagentListQuery = {},
  signal?: AbortSignal
): Promise<SubagentListResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/subagents",
    {
      params: {
        path: { workspace_id: workspaceId, session_id: sessionId },
        query: { all_profiles: true, ...query },
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to fetch subagents", sessionId);
  }
  return requireResponseData(data, response, "Failed to fetch subagents");
}

/** Cancels a delegated subagent in its parent's profile; the daemon settles it asynchronously. */
export async function cancelSubagent(
  workspaceId: string,
  subagentId: string,
  profile: string,
  reason = ""
): Promise<SubagentCancelResponse> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/subagents/{subagent_id}/cancel",
    {
      params: { path: { workspace_id: workspaceId, subagent_id: subagentId }, query: { profile } },
      body: { reason },
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to stop subagent "${subagentId}"`);
  }
  return requireResponseData(data, response, `Failed to stop subagent "${subagentId}"`);
}
