import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type { SessionPresenceRequest } from "../types";
import { throwSessionRequestError } from "./session-api-errors";

/**
 * The operator presence lease: how the daemon knows a session is being watched.
 *
 * Acquiring returns an opaque lease id; renewing and releasing require it, so
 * one client can never release another's lease and a second open window keeps
 * the session seen. While any lease is live, a turn that settles is marked seen
 * in the same step and never becomes `done`.
 *
 * Operator scope: a request carrying agent identity is denied by the daemon.
 */
async function postPresence(
  workspaceId: string,
  sessionId: string,
  body: SessionPresenceRequest,
  signal?: AbortSignal
) {
  return apiClient.POST("/api/workspaces/{workspace_id}/sessions/{session_id}/presence", {
    params: { path: { workspace_id: workspaceId, session_id: sessionId } },
    body,
    signal,
  });
}

/** Acquire returns the lease id every later call must present. */
export async function acquireSessionPresence(
  workspaceId: string,
  sessionId: string,
  signal?: AbortSignal
): Promise<string> {
  const { data, error, response } = await postPresence(
    workspaceId,
    sessionId,
    { visible: true },
    signal
  );
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to report session presence");
  }
  return requireResponseData(data, response, "Failed to report session presence").lease_id;
}

/** Renew keeps the lease alive well inside its TTL. Returns false if it is gone. */
export async function renewSessionPresence(
  workspaceId: string,
  sessionId: string,
  leaseId: string,
  signal?: AbortSignal
): Promise<boolean> {
  const { error, response } = await postPresence(
    workspaceId,
    sessionId,
    { visible: true, lease_id: leaseId },
    signal
  );
  return !apiRequestFailed(response, error);
}

/** Release on blur so a backgrounded session can go `done` again. */
export async function releaseSessionPresence(
  workspaceId: string,
  sessionId: string,
  leaseId: string
): Promise<void> {
  await postPresence(workspaceId, sessionId, { visible: false, lease_id: leaseId });
}
