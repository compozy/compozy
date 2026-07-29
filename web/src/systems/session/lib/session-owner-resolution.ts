import type { QueryClient } from "@tanstack/react-query";

import { SessionNotFoundError } from "../adapters/session-api-errors";
import type { SessionOwnerResponse } from "../types";
import {
  SESSION_OWNER_STALE_TIME_MS,
  sessionOwnerKeys,
  sessionOwnerOptions,
  type SessionOwnerDialogState,
} from "./query-options";

export function sessionOwnerDialogState(owner: SessionOwnerResponse): SessionOwnerDialogState {
  return {
    sessionId: owner.session_id,
    workspaceId: owner.workspace_id,
    workspaceName: owner.workspace_name,
  };
}

/**
 * A fresh owner settles the confirmation round trip without putting foreign detail data into a
 * workspace-scoped cache. The bounded stale window keeps rename and deletion truth refreshable.
 */
export function cachedForeignSessionOwner(
  queryClient: QueryClient,
  sessionId: string,
  activeWorkspaceId: string
): SessionOwnerDialogState | null {
  const state = queryClient.getQueryState<SessionOwnerResponse>(sessionOwnerKeys.detail(sessionId));
  const owner = state?.data;
  const isFresh =
    state !== undefined && Date.now() - state.dataUpdatedAt < SESSION_OWNER_STALE_TIME_MS;
  if (!owner || !isFresh || owner.workspace_id === activeWorkspaceId) {
    return null;
  }
  return sessionOwnerDialogState(owner);
}

export async function resolveSessionOwner(
  queryClient: QueryClient,
  sessionId: string
): Promise<SessionOwnerDialogState | null> {
  try {
    return sessionOwnerDialogState(
      await queryClient.ensureQueryData(sessionOwnerOptions(sessionId))
    );
  } catch (error) {
    if (error instanceof SessionNotFoundError) {
      return null;
    }
    throw error;
  }
}

/** Resolves only a genuinely foreign owner after an active-workspace detail miss. */
export async function resolveForeignSessionOwner(
  queryClient: QueryClient,
  sessionId: string,
  activeWorkspaceId: string
): Promise<SessionOwnerDialogState | null> {
  const owner = await resolveSessionOwner(queryClient, sessionId);
  return owner && owner.workspaceId !== activeWorkspaceId ? owner : null;
}
