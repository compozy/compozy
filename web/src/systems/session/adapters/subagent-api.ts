import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";
import type { OperationQuery, OperationResponse } from "@/lib/api-contract";

import { throwSessionRequestError } from "./session-api-errors";

export type SubagentListQuery = OperationQuery<"listSessionSubagents">;
export type SubagentListResponse = OperationResponse<"listSessionSubagents", 200>;
export type SubagentPayload = SubagentListResponse["subagents"][number];

/**
 * `SubagentPayload.worktree` as `_dx.md` defines it. Declared here until the
 * generated contract carries `isolation` and `worktree`; then this shape and
 * `SubagentWirePayload` collapse into the generated `SubagentPayload`.
 */
export interface SubagentWorktreePayload {
  id: string;
  name: string;
  branch: string;
  base_ref: string;
  base_sha?: string;
  path: string;
  head_sha?: string;
  commits_ahead?: number;
  dirty_files?: number;
  observed_at?: string;
  pull_request_status?: string;
  pull_request?: { url: string; number: number; state: string };
}

export type SubagentWirePayload = SubagentPayload & {
  isolation?: string;
  worktree?: SubagentWorktreePayload | null;
};
export type SubagentCancelResponse = OperationResponse<"cancelSubagent", 202>;

/** One page of a parent session's direct subagents, newest first. */
export async function fetchSessionSubagents(
  workspaceId: string,
  sessionId: string,
  query: SubagentListQuery = {},
  signal?: AbortSignal
): Promise<SubagentListResponse> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/subagents",
    {
      params: { path: { workspace_id: workspaceId, session_id: sessionId }, query },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to fetch subagents", sessionId);
  }
  return requireResponseData(data, response, "Failed to fetch subagents");
}

/** Requests cancellation of one delegated subagent; the daemon settles it asynchronously. */
export async function cancelSubagent(
  workspaceId: string,
  subagentId: string,
  reason = ""
): Promise<SubagentCancelResponse> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/subagents/{subagent_id}/cancel",
    {
      params: { path: { workspace_id: workspaceId, subagent_id: subagentId } },
      body: { reason },
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to stop subagent "${subagentId}"`);
  }
  return requireResponseData(data, response, `Failed to stop subagent "${subagentId}"`);
}
