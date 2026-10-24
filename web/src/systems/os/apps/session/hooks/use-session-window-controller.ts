import { useEffect } from "react";
import { toast } from "sonner";

import { useProfileReadScope } from "@/systems/profiles";
import { useSessionDeletionPending, useSessionPresence } from "@/systems/session";
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
  const resolution = useSessionWindowResolution({
    sessionId,
    focusedScope,
    liveTailEnabled,
  });
  const remotelyGone = sessionRemotelyGone(sessionId, resolution, aggregate);
  const deletedLocally = remotelyGone;
  const effectiveLiveTailEnabled =
    liveTailEnabled && !deletedLocally && !resolution.crossesWorkspace;
  useSessionPresence(
    resolution.workspaceId,
    sessionId,
    presenceEnabled && sessionPresenceAllowed(resolution, deletedLocally)
  );
  // A document's project lens cannot remove a valid shared window. Only a
  // confirmed missing session retires it; scope mismatches render a local notice.
  useRetireMissingSessionWindow(windowId, sessionId !== null && !deletionPending && remotelyGone);

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

function useRetireMissingSessionWindow(windowId: string, retire: boolean) {
  const { coordinator } = useOsShell();
  useEffect(() => {
    if (!retire) return;
    toast.error("Session not found");
    void coordinator.userRetireSession(windowId);
  }, [coordinator, retire, windowId]);
}
