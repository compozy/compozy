import { useEffect, useState } from "react";
import { toast } from "sonner";

import { useProfileReadScope } from "@/systems/profiles";
import {
  useSessionDeletedByOperator,
  useSessionDeletionPending,
  useSessionPresence,
} from "@/systems/session";
import { useWindowDataWorkspace } from "@/systems/workspace";

import { useOsShell } from "../../../hooks/use-os-shell";
import { useSessionWindowResolution } from "./use-session-window-resolution";
import { useSessionWindowRuntime } from "./use-session-window-runtime";

/** Resolves the session window identity and retires it once its session is confirmed gone. */
export function useSessionWindowController(windowId: string) {
  const { aggregate } = useProfileReadScope();
  // The window's own scope (its binding, else its partition): a session of
  // another scope is hidden here; a catalog window on All projects hides none.
  const focusedScope = focusedWindowScope(useWindowDataWorkspace());
  const { liveTailEnabled, presenceEnabled, sessionId, agentName } =
    useSessionWindowRuntime(windowId);
  const deletionPending = useSessionDeletionPending(sessionId);
  const deletedByOperator = useSessionDeletedByOperator(sessionId);
  const resolution = useSessionWindowResolution({
    sessionId,
    focusedScope,
    enabled: sessionId !== null && !deletedByOperator && !deletionPending,
    liveTailEnabled: liveTailEnabled && !deletedByOperator && !deletionPending,
  });
  const remotelyGone = sessionRemotelyGone(sessionId, resolution, aggregate);
  const archivedWhileOpen = useSessionArchivedWhileOpen(sessionId, resolution);
  const deletedLocally = deletedByOperator || remotelyGone;
  const effectiveLiveTailEnabled =
    liveTailEnabled && !deletionPending && !deletedLocally && !resolution.crossesWorkspace;
  useSessionPresence(
    resolution.workspaceId,
    sessionId,
    presenceEnabled && !deletionPending && sessionPresenceAllowed(resolution, deletedLocally)
  );
  // A document's project lens cannot remove a valid shared window. Only a confirmed
  // missing session, one this client deleted, or one archived while open retires it;
  // scope mismatches render a local notice.
  useRetireSessionWindow(
    windowId,
    sessionId !== null && !deletionPending && (deletedLocally || archivedWhileOpen),
    // The list's delete already retired this window; if that retarget was refused, retire it
    // again here without announcing an operator-deleted session as missing. An archived
    // session is not missing either.
    !deletedByOperator && !archivedWhileOpen
  );

  return {
    ...resolution,
    agentName,
    deletedLocally,
    effectiveLiveTailEnabled,
    liveTailEnabled,
    sessionId,
  };
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
  { session, isFetching }: Pick<SessionWindowResolution, "session" | "isFetching">
): boolean {
  const [seenUnarchivedId, setSeenUnarchivedId] = useState<string | null>(null);
  const shown = session?.id === sessionId ? session : undefined;
  const seenUnarchived = shown?.archived_at === null && !isFetching;
  const next = seenUnarchived || seenUnarchivedId === sessionId ? sessionId : null;
  if (next !== seenUnarchivedId) setSeenUnarchivedId(next);
  return next !== null && shown !== undefined && shown.archived_at !== null;
}

function useRetireSessionWindow(windowId: string, retire: boolean, announce: boolean) {
  const { coordinator } = useOsShell();
  useEffect(() => {
    if (!retire) return;
    if (announce) toast.error("Session not found");
    void coordinator.userRetireSession(windowId);
  }, [announce, coordinator, retire, windowId]);
}
