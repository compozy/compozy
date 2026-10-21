import { useQuery } from "@tanstack/react-query";

import { useProfileReadScope } from "@/systems/profiles";
import { isSessionUnavailable } from "@/systems/session/adapters/session-api-errors";
import {
  sessionScopedDetailOptions,
  useForeignProfileSession,
  type ForeignProfileSessionState,
  type SessionPayload,
} from "@/systems/session";
import {
  ownerDataScope,
  sameDataScope,
  type DataScope,
  type FocusedDataScope,
} from "@/systems/workspace";

export interface SessionWindowResolution {
  session: SessionPayload | undefined;
  /** The session's own workspace id (`""` for User), or the focused one before it resolves. */
  workspaceId: string | null;
  /** What the window's session list shows: the session's scope, else the focused one. */
  listScope: DataScope | null;
  isLoading: boolean;
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
}: {
  sessionId: string | null;
  /** The focused destination (a project or User); `null` under All projects or unsettled. */
  focusedScope: FocusedDataScope | null;
  liveTailEnabled: boolean;
}): SessionWindowResolution {
  const { params } = useProfileReadScope();
  const query = useQuery(
    sessionScopedDetailOptions(sessionId ?? "", params, {
      enabled: sessionId !== null,
      liveTail: liveTailEnabled,
    })
  );
  const scopedMiss = query.isError && isSessionUnavailable(query.error);
  const foreign = useForeignProfileSession(sessionId, scopedMiss);
  const foreignSession = foreign.status === "found" ? foreign.session : undefined;
  const resolvedSession = query.data ?? foreignSession;
  const sessionWorkspaceId = resolvedSession ? (resolvedSession.workspace_id?.trim() ?? "") : null;
  const sessionScope = sessionWorkspaceId === null ? null : ownerDataScope(sessionWorkspaceId);
  const focusedWorkspaceId =
    focusedScope === null ? null : focusedScope.kind === "user" ? "" : focusedScope.workspaceId;
  return {
    session: query.data,
    workspaceId: sessionWorkspaceId ?? focusedWorkspaceId,
    listScope: sessionScope ?? focusedScope,
    isLoading: query.isLoading,
    error: query.error,
    scopedMiss,
    foreign,
    // A project's session on the User desktop crosses too, and vice versa.
    crossesWorkspace:
      sessionScope !== null && focusedScope !== null && !sameDataScope(sessionScope, focusedScope),
  };
}
