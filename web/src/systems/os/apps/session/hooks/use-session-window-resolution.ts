import { useQuery } from "@tanstack/react-query";

import { useProfileReadScope } from "@/systems/profiles";
import {
  sessionScopedDetailOptions,
  useForeignProfileSession,
  type ForeignProfileSessionState,
  type SessionPayload,
} from "@/systems/session";

export interface SessionWindowResolution {
  session: SessionPayload | undefined;
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
  runtimeWorkspaceId,
  liveTailEnabled,
}: {
  sessionId: string | null;
  runtimeWorkspaceId: string | null;
  liveTailEnabled: boolean;
}): SessionWindowResolution {
  const { key: profileKey } = useProfileReadScope();
  const query = useQuery(
    sessionScopedDetailOptions(sessionId ?? "", profileKey, {
      enabled: sessionId !== null,
      liveTail: liveTailEnabled,
    })
  );
  const scopedMiss = query.isError;
  const foreign = useForeignProfileSession(sessionId, scopedMiss);
  const sessionWorkspaceId = query.data?.workspace_id?.trim();
  return {
    session: query.data,
    isLoading: query.isLoading,
    error: query.error,
    scopedMiss,
    foreign,
    crossesWorkspace:
      sessionWorkspaceId !== undefined &&
      runtimeWorkspaceId !== null &&
      sessionWorkspaceId !== runtimeWorkspaceId,
  };
}
