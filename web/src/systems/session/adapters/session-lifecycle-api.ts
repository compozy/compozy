import {
  apiClientFor,
  apiRequestFailed,
  requireResponseData,
  type ComputerId,
} from "@/lib/api-client";

import type { RenameSessionRequest, SessionPayload, SessionStopResult } from "../types";
import { throwSessionRequestError } from "./session-api-errors";
import type { SessionProcessPolicy } from "./session-processes-api";
import { sessionWorkspacePath } from "./session-workspace-path";

/** Deletes a session in its owning workspace and profile; rejects failed API responses. */
export async function deleteSession(
  workspaceId: string,
  id: string,
  profile: string,
  signal?: AbortSignal,
  processes?: SessionProcessPolicy,
  computerId?: ComputerId
): Promise<void> {
  const { error, response } = await apiClientFor(computerId).DELETE(
    "/api/workspaces/{workspace_id}/sessions/{session_id}",
    {
      params: {
        path: { workspace_id: sessionWorkspacePath(workspaceId), session_id: id },
        query: processes ? { profile, processes } : { profile },
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to delete session "${id}"`, id);
  }
}

/** Renames a session within its captured workspace and owning profile. */
export async function renameSession(
  workspaceId: string,
  id: string,
  profile: string,
  request: RenameSessionRequest,
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<SessionPayload> {
  const { data, error, response } = await apiClientFor(computerId).PATCH(
    "/api/workspaces/{workspace_id}/sessions/{session_id}",
    {
      params: {
        path: { workspace_id: sessionWorkspacePath(workspaceId), session_id: id },
        query: { profile },
      },
      body: request,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to rename session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to rename session "${id}"`).session;
}

/** Nests a top-level session under `parentId` in the same project and profile. */
export async function setSessionParent(
  workspaceId: string,
  id: string,
  profile: string,
  parentId: string,
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<SessionPayload> {
  const { data, error, response } = await apiClientFor(computerId).PUT(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/parent",
    {
      params: {
        path: { workspace_id: sessionWorkspacePath(workspaceId), session_id: id },
        query: { profile },
      },
      body: { parent_session_id: parentId },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to move session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to move session "${id}"`).session;
}

/** Returns a nested (provenance) child to the top level. */
export async function clearSessionParent(
  workspaceId: string,
  id: string,
  profile: string,
  signal?: AbortSignal,
  computerId?: ComputerId
): Promise<SessionPayload> {
  const { data, error, response } = await apiClientFor(computerId).DELETE(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/parent",
    {
      params: {
        path: { workspace_id: sessionWorkspacePath(workspaceId), session_id: id },
        query: { profile },
      },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to move session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to move session "${id}"`).session;
}

export interface StopSessionOptions {
  signal?: AbortSignal;
  /**
   * Block on the shared stop manager until the ladder settles (verified or
   * unverified). Off by default: an ordinary stop is accepted with 202 and the
   * session read model owns the settled state.
   */
  wait?: boolean;
  /** Stop or keep the processes the session left running; omitted keeps them. */
  processes?: SessionProcessPolicy;
  /** Computer owning the session; omitted targets this computer. */
  computerId?: ComputerId;
}

/**
 * Requests a session stop. Without `wait` the daemon answers 202 once the stop
 * is accepted (or 200 when the session is already stopped) and settles the
 * session through the shared stop manager; with `wait` it answers 200 with the
 * settled outcome, including an unverified one. The answer is evidence that
 * this request settled — session state stays with the read model.
 */
export async function stopSession(
  workspaceId: string,
  id: string,
  profile: string,
  options: StopSessionOptions = {},
  computerId?: ComputerId
): Promise<SessionStopResult> {
  const { data, error, response } = await apiClientFor(computerId ?? options.computerId).POST(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/stop",
    {
      params: {
        path: { workspace_id: sessionWorkspacePath(workspaceId), session_id: id },
        query: { profile },
      },
      body: {
        wait: options.wait === true,
        ...(options.processes ? { processes: options.processes } : {}),
      },
      signal: options.signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, `Failed to stop session "${id}"`, id);
  }
  return requireResponseData(data, response, `Failed to stop session "${id}"`);
}
