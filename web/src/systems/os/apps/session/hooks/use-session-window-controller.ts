import { toast } from "@compozy/ui";
import { useEffect, useState } from "react";

import { useProfileReadScope } from "@/systems/profiles";
import {
  useSessionArchivePending,
  useSessionDeletedByOperator,
  useSessionDeletionPending,
  useSessionPresence,
  type SessionPayload,
} from "@/systems/session";
import { useWindowDataWorkspace } from "@/systems/workspace";

import { useOsShell } from "../../../hooks/use-os-shell";
import { useSessionWindowResolution } from "./use-session-window-resolution";
import { useSessionWindowRuntime } from "./use-session-window-runtime";

/**
 * Resolves the session window identity and retires it once its session is confirmed gone.
 *
 * While a delete this window confirmed is in flight, its target (`deletingSessions`) stands in
 * for the resolution once the delete clears the by-id cache: the window keeps its loaded view
 * instead of swapping to skeleton/notice/empty, which is what unmounted the confirmation
 * mid-flight. Once the request settles the stand-in drops and the window follows the normal
 * gone/retire path.
 */
export function useSessionWindowController(
  windowId: string,
  options: { deletingSessions?: readonly SessionPayload[] | null } = {}
) {
  const { aggregate } = useProfileReadScope();
  // The window's own scope (its binding, else its partition): a session of
  // another scope is hidden here; a catalog window on All projects hides none.
  const focusedScope = focusedWindowScope(useWindowDataWorkspace());
  const { focused, liveTailEnabled, presenceEnabled, sessionId, agentName } =
    useSessionWindowRuntime(windowId);
  const deletionPending = useSessionDeletionPending(sessionId);
  const archivePending = useSessionArchivePending(windowId);
  const deletedByOperator = useSessionDeletedByOperator(sessionId);
  const resolution = useSessionWindowResolution({
    sessionId,
    focusedScope,
    enabled: sessionId !== null && !deletedByOperator && !deletionPending,
    liveTailEnabled: liveTailEnabled && !deletedByOperator && !deletionPending,
  });
  const remotelyGone = sessionRemotelyGone(sessionId, resolution, aggregate);
  const archivedWhileOpen = useSessionArchivedWhileOpen(sessionId, resolution, archivePending);
  const retention = sessionDeletionRetention({
    deletedByOperator,
    remotelyGone,
    shownSession: resolution.shownSession,
    resolvedSession: resolution.session,
    sessionId,
    deletingSessions: options.deletingSessions,
    workspaceId: resolution.workspaceId,
    isLoading: resolution.isLoading,
    error: resolution.error,
  });
  const { operatorDeleted, retainingSession, deletedLocally, session } = retention;
  const effectiveLiveTailEnabled =
    liveTailEnabled && !deletionPending && !operatorDeleted && !resolution.crossesWorkspace;
  useSessionPresence(
    resolution.workspaceId,
    sessionId,
    presenceEnabled &&
      !retainingSession &&
      !deletionPending &&
      sessionPresenceAllowed(resolution, operatorDeleted)
  );
  // A document's project lens cannot remove a valid shared window. Only a confirmed
  // missing session, one this client deleted, or one archived while open retires it;
  // scope mismatches render a local notice. A delete still in flight never retires it.
  useRetireSessionWindow(
    windowId,
    sessionId,
    sessionId !== null && !deletionPending && (deletedLocally || archivedWhileOpen),
    // The list's delete already retired this window; if that retarget was refused, retire it
    // again here without announcing an operator-deleted session as missing. An archived
    // session is not missing either.
    !deletedByOperator && !archivedWhileOpen
  );

  return {
    ...resolution,
    workspaceId: retention.workspaceId,
    isLoading: retention.isLoading,
    error: retention.error,
    agentName,
    deletedLocally,
    effectiveLiveTailEnabled,
    focused,
    liveTailEnabled,
    retainingSession,
    session,
    sessionId,
  };
}

/**
 * What the window shows while a delete it confirmed is in flight. The delete's own target
 * stands in only once the by-id read can no longer answer: before that the resolution is
 * fresher, and after the request settles the target is gone and the normal gone/retire path
 * takes over.
 */
function sessionDeletionRetention({
  deletedByOperator,
  remotelyGone,
  shownSession,
  resolvedSession,
  sessionId,
  deletingSessions,
  workspaceId,
  isLoading,
  error,
}: {
  deletedByOperator: boolean;
  remotelyGone: boolean;
  shownSession: SessionPayload | undefined;
  resolvedSession: SessionPayload | undefined;
  sessionId: string | null;
  deletingSessions: readonly SessionPayload[] | null | undefined;
  workspaceId: string | null;
  isLoading: boolean;
  error: Error | null;
}) {
  const operatorDeleted = deletedByOperator || remotelyGone;
  const retained =
    shownSession === undefined ? retainedDeletionTarget(sessionId, deletingSessions) : undefined;
  const retainingSession = retained !== undefined;
  return {
    operatorDeleted,
    retainingSession,
    deletedLocally: operatorDeleted && !retainingSession,
    session: retained ?? resolvedSession,
    // A retained target keeps its own project: the by-id read that supplied the workspace
    // is gone by the time the delete clears the cache.
    workspaceId: retained ? (retained.workspace_id?.trim() ?? "") : workspaceId,
    isLoading: retainingSession ? false : isLoading,
    error: retainingSession ? null : error,
  };
}

/** The session a delete this window confirmed is removing, while it matches the window. */
function retainedDeletionTarget(
  sessionId: string | null,
  deletingSessions: readonly SessionPayload[] | null | undefined
): SessionPayload | undefined {
  if (sessionId === null) return undefined;
  return deletingSessions?.find(session => session.id === sessionId);
}

type SessionWindowResolution = ReturnType<typeof useSessionWindowResolution>;

function focusedWindowScope(windowScope: ReturnType<typeof useWindowDataWorkspace>) {
  return windowScope === null || windowScope.kind === "all" ? null : windowScope;
}

function sessionRemotelyGone(
  sessionId: string | null,
  resolution: SessionWindowResolution,
  aggregate: boolean
): boolean {
  if (sessionId === null || !resolution.scopedMiss) return false;
  return aggregate || resolution.foreign.status === "missing";
}

function sessionPresenceAllowed(resolution: SessionWindowResolution, deletedLocally: boolean) {
  if (resolution.session === undefined || resolution.scopedMiss || deletedLocally) return false;
  return !resolution.crossesWorkspace && resolution.foreign.status !== "loading";
}

/**
 * True once the session this window showed unarchived turns archived, from any surface: the
 * catalog stream rereads the detail for a CLI or native-tool archive too. Only the transition
 * counts, so an archived session opened on purpose stays. A read in flight proves nothing: the
 * cache of a session archived earlier still says unarchived until its refetch lands.
 */
function useSessionArchivedWhileOpen(
  sessionId: string | null,
  { shownSession, isFetching }: Pick<SessionWindowResolution, "shownSession" | "isFetching">,
  archivePending: boolean
): boolean {
  const [seenUnarchivedId, setSeenUnarchivedId] = useState<string | null>(null);
  const shown = shownSession?.id === sessionId ? shownSession : undefined;
  const seenUnarchived = shown?.archived_at === null && !isFetching;
  const next = seenUnarchived || seenUnarchivedId === sessionId ? sessionId : null;
  if (next !== seenUnarchivedId) setSeenUnarchivedId(next);
  return !archivePending && next !== null && shown !== undefined && shown.archived_at !== null;
}

function useRetireSessionWindow(
  windowId: string,
  sessionId: string | null,
  retire: boolean,
  announce: boolean
) {
  const { coordinator } = useOsShell();
  useEffect(() => {
    if (!retire || sessionId === null) return;
    if (announce) toast.error("Session not found");
    // The shell may retarget before this effect runs; retire only the session we resolved.
    void coordinator.userRetireSession(windowId, sessionId);
  }, [announce, coordinator, retire, sessionId, windowId]);
}
