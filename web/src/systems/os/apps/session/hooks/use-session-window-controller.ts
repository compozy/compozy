import { useEffect, useState } from "react";
import { toast } from "sonner";

import { useProfileReadScope } from "@/systems/profiles";
import { useSessionDeletionPending, useSessionPresence } from "@/systems/session";
import { useWindowDataWorkspace } from "@/systems/workspace";

import { useOsShell } from "../../../hooks/use-os-shell";
import { useSessionWindowResolution } from "./use-session-window-resolution";
import { useSessionWindowRuntime } from "./use-session-window-runtime";

/** Resolves the session window identity and owns its redirect/delete lifecycle. */
export function useSessionWindowController(windowId: string) {
  const { coordinator } = useOsShell();
  const { aggregate } = useProfileReadScope();
  // The window's own scope (its binding, else its partition): a session of
  // another scope is hidden here; a catalog window on All projects hides none.
  const windowScope = useWindowDataWorkspace();
  const focusedScope = windowScope === null || windowScope.kind === "all" ? null : windowScope;
  const { liveTailEnabled, presenceEnabled, sessionId, agentName } =
    useSessionWindowRuntime(windowId);
  const deletionPending = useSessionDeletionPending(sessionId);
  const [deletedSessionId, setDeletedSessionId] = useState<string | null>(null);
  const deletedByOperator = sessionId !== null && deletedSessionId === sessionId;
  const resolution = useSessionWindowResolution({
    sessionId,
    focusedScope,
    liveTailEnabled: liveTailEnabled && !deletedByOperator,
  });
  const remotelyGone =
    sessionId !== null &&
    resolution.scopedMiss &&
    (aggregate || resolution.foreign.status === "missing");
  const deletedLocally = deletedByOperator || remotelyGone;
  const effectiveLiveTailEnabled =
    liveTailEnabled && !deletedLocally && !resolution.crossesWorkspace;
  useSessionPresence(
    resolution.workspaceId,
    sessionId,
    presenceEnabled &&
      resolution.session !== undefined &&
      !resolution.scopedMiss &&
      !deletedLocally &&
      !resolution.crossesWorkspace &&
      resolution.foreign.status !== "loading"
  );

  useEffect(() => {
    // A document's project lens cannot remove a valid shared window. Only a
    // confirmed missing session retires it; scope mismatches render a local notice.
    if (sessionId === null || deletionPending || deletedByOperator || !remotelyGone) return;
    toast.error("Session not found");
    void coordinator.userRetireSession(windowId);
  }, [coordinator, deletedByOperator, deletionPending, remotelyGone, sessionId, windowId]);

  return {
    ...resolution,
    agentName,
    deletedLocally,
    effectiveLiveTailEnabled,
    liveTailEnabled,
    sessionId,
    handleDeleteSuccess: () => {
      if (sessionId === null) return;
      setDeletedSessionId(sessionId);
      void coordinator.userRetireSession(windowId);
    },
  };
}
