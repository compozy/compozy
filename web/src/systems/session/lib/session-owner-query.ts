import type { QueryClient } from "@tanstack/react-query";
import { queryOptions } from "@tanstack/react-query";

import { SessionNotFoundError } from "../adapters/session-api-errors";
import { fetchSessionOwner } from "../adapters/session-owner-api";
import type { SessionOwnerResponse } from "../types";

/**
 * The only workspace-agnostic session key in the app. It stays out of `sessionKeys` on purpose:
 * every catalog/detail/transcript key is workspace-prefixed and owns rendered session data, while
 * the owner projection answers "which workspace owns this id" before any workspace is chosen.
 * Merging the two would let a foreign payload reach a workspace-scoped cache (ADR-004).
 */
export const sessionOwnerKeys = {
  all: ["session-owner"] as const,
  detail: (sessionId: string) => [...sessionOwnerKeys.all, sessionId] as const,
};

/** Dialog state for a foreign-workspace deep link — the whole payload the confirm step may carry. */
export interface SessionOwnerDialogState {
  sessionId: string;
  workspaceId: string;
  workspaceName: string;
}

const SESSION_OWNER_STALE_TIME_MS = 30_000;

export function sessionOwnerOptions(sessionId: string) {
  return queryOptions({
    queryKey: sessionOwnerKeys.detail(sessionId),
    queryFn: ({ signal }) => fetchSessionOwner(sessionId, signal),
    staleTime: SESSION_OWNER_STALE_TIME_MS,
    enabled: !!sessionId,
  });
}

export function sessionOwnerDialogState(owner: SessionOwnerResponse): SessionOwnerDialogState {
  return {
    sessionId: owner.session_id,
    workspaceId: owner.workspace_id,
    workspaceName: owner.workspace_name,
  };
}

/**
 * A fresh owner settles the question for the confirmation round trip: the deep link stops
 * re-asking the active workspace for a session it already knows lives elsewhere. Respecting the
 * query's stale window keeps workspace rename/deletion truth refreshable.
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

/**
 * Resolves the owning workspace of a session that missed in the active workspace.
 * `null` means the session exists nowhere — the caller keeps its not-found outcome.
 * Every other failure propagates so the route stays truthful and recoverable.
 */
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
