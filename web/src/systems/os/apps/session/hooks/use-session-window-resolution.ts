import { useQuery } from "@tanstack/react-query";

import { useProfileReadScope } from "@/systems/profiles";
import { isSessionUnavailable } from "@/systems/session/adapters/session-api-errors";
import {
  sessionScopedDetailOptions,
  useForeignProfileSession,
  type ForeignProfileSessionState,
  type SessionPayload,
} from "@/systems/session";
import { ownerDataScope, sameDataScope, type FocusedDataScope } from "@/systems/workspace";

function focusedWorkspaceId(focusedScope: FocusedDataScope | null): string | null {
  if (focusedScope === null) return null;
  return focusedScope.kind === "user" ? "" : focusedScope.workspaceId;
}

export interface SessionWindowResolution {
  session: SessionPayload | undefined;
  /** What the window shows: the scoped read, else another profile's session read-only. */
  shownSession: SessionPayload | undefined;
  /** The session's own workspace id (`""` for User), or the focused one before it resolves. */
  workspaceId: string | null;
  isLoading: boolean;
  /** A read is in flight, so `shownSession` may predate what the daemon holds now. */
  isFetching: boolean;
  error: Error | null;
  /** The scoped read answered "not in this profile" — not yet "nowhere". */
  scopedMiss: boolean;
  foreign: ForeignProfileSessionState;
  crossesWorkspace: boolean;
}

/**
 * Resolves a session on both ownership axes before the window commits to a view.
 *
 * The primary read is the profile-enforced by-id route rather than the workspace
 * one, because only the former compares the session's owner against the scope.
 * Its 404 is what makes a foreign session distinguishable from a deleted one,
 * and the aggregate lookup that follows is what turns that distinction into
 * something the operator can act on.
 */
export function useSessionWindowResolution({
  sessionId,
  focusedScope,
  liveTailEnabled,
  enabled = true,
}: {
  sessionId: string | null;
  /** The window's data scope when bounded (a project or User); `null` for the aggregate or unsettled. */
  focusedScope: FocusedDataScope | null;
  liveTailEnabled: boolean;
  enabled?: boolean;
}): SessionWindowResolution {
  const { params } = useProfileReadScope();
  const query = useQuery(
    sessionScopedDetailOptions(sessionId ?? "", params, {
      enabled: sessionId !== null && enabled,
      liveTail: liveTailEnabled,
    })
  );
  const scopedMiss = query.isError && isSessionUnavailable(query.error);
  const foreign = useForeignProfileSession(sessionId, scopedMiss && enabled);
  const foreignSession = foreign.status === "found" ? foreign.session : undefined;
  const resolvedSession = query.data ?? foreignSession;
  const sessionWorkspaceId = resolvedSession ? (resolvedSession.workspace_id?.trim() ?? "") : null;
  const sessionScope = sessionWorkspaceId === null ? null : ownerDataScope(sessionWorkspaceId);
  return {
    session: query.data,
    shownSession: resolvedSession,
    workspaceId: sessionWorkspaceId ?? focusedWorkspaceId(focusedScope),
    isLoading: query.isLoading,
    isFetching: query.isFetching || (foreign.status === "found" && foreign.isFetching),
    error: query.error,
    scopedMiss,
    foreign,
    // A project's session on the User desktop crosses too, and vice versa.
    crossesWorkspace:
      sessionScope !== null && focusedScope !== null && !sameDataScope(sessionScope, focusedScope),
  };
}
