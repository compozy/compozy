import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type { SessionPayload } from "../types";
import type { components } from "@compozy/client";
import { throwSessionRequestError } from "./session-api-errors";
import { sessionWorkspacePath } from "./session-workspace-path";

export type MoveSessionToLaterRequest = components["schemas"]["MoveSessionToLaterRequest"];
export type ResumeSessionFromLaterRequest = components["schemas"]["ResumeSessionFromLaterRequest"];

/**
 * Parks a session in Later using its explicit owner profile. The daemon stops
 * the whole session tree first (turn, children, monitors, processes,
 * terminals), so the request can take a few seconds.
 */
export async function moveSessionToLater(
  workspaceId: string,
  id: string,
  profile: string,
  body: MoveSessionToLaterRequest,
  signal?: AbortSignal
): Promise<SessionPayload> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/later",
    {
      params: {
        path: { workspace_id: sessionWorkspacePath(workspaceId), session_id: id },
        query: { profile },
      },
      body,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to move session "${id}" to Later`, id);
  }
  return requireResponseData(data, response, `Failed to move session "${id}" to Later`).session;
}

/** Brings a parked session back to the normal flow; the runtime is not started. */
export async function resumeSessionFromLater(
  workspaceId: string,
  id: string,
  profile: string,
  body: ResumeSessionFromLaterRequest = {},
  signal?: AbortSignal
): Promise<SessionPayload> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/resume-later",
    {
      params: {
        path: { workspace_id: sessionWorkspacePath(workspaceId), session_id: id },
        query: { profile },
      },
      body,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to bring session "${id}" back`, id);
  }
  return requireResponseData(data, response, `Failed to bring session "${id}" back`).session;
}
