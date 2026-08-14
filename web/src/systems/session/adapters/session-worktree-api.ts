import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type { ForkSessionToWorktreeParams, SessionPayload } from "../types";
import { throwSessionRequestError } from "./session-api-errors";

/**
 * Creates a fresh session bound to the target worktree. The origin session is
 * never moved: `confirmed` is mandatory precisely because the caller must have
 * been told that, and its uncommitted changes stay where they are.
 *
 * Pass an already-ready `worktree` id. Materializing a new worktree through this
 * call is synchronous daemon-side and would hide the creation phases, so the
 * fork flow creates the worktree first and forks into it once it is ready.
 */
export async function forkSessionToWorktree(
  workspaceId: string,
  sessionId: string,
  params: ForkSessionToWorktreeParams,
  signal?: AbortSignal
): Promise<SessionPayload> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/worktree-fork",
    {
      params: { path: { workspace_id: workspaceId, session_id: sessionId } },
      body: params,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(
      response,
      error,
      `Failed to fork session "${sessionId}" into a worktree`,
      sessionId
    );
  }
  return requireResponseData(
    data,
    response,
    `Failed to fork session "${sessionId}" into a worktree`
  ).session;
}
