import { useEffect, useState } from "react";
import { toast } from "sonner";

import { useSessionPresence } from "@/systems/session";
import { useActiveWorkspace } from "@/systems/workspace";

import { useOsShell } from "../../../hooks/use-os-shell";
import { useSessionWindowResolution } from "./use-session-window-resolution";
import { useSessionWindowRuntime } from "./use-session-window-runtime";

/** Resolves the session window identity and owns its redirect/delete lifecycle. */
export function useSessionWindowController(windowId: string) {
  const { coordinator } = useOsShell();
  const activeWorkspace = useActiveWorkspace();
  const runtimeWorkspaceId = activeWorkspace.runtimeWorkspaceId?.trim() || null;
  const { liveTailEnabled, presenceEnabled, sessionId, agentName } =
    useSessionWindowRuntime(windowId);
  const [deletedSessionId, setDeletedSessionId] = useState<string | null>(null);
  const deletedByOperator = sessionId !== null && deletedSessionId === sessionId;
  const resolution = useSessionWindowResolution({
    sessionId,
    runtimeWorkspaceId,
    liveTailEnabled: liveTailEnabled && !deletedByOperator,
  });
  const remotelyGone =
    sessionId !== null && resolution.scopedMiss && resolution.foreign.status === "missing";
  const deletedLocally = deletedByOperator || remotelyGone;
  const effectiveLiveTailEnabled =
    liveTailEnabled && !deletedLocally && !resolution.crossesWorkspace;
  useSessionPresence(
    resolution.workspaceId,
    sessionId,
    presenceEnabled &&
      !deletedLocally &&
      !resolution.crossesWorkspace &&
      resolution.foreign.status !== "loading"
  );

  useEffect(() => {
    // A document's project lens cannot remove a valid shared window. Only a
    // confirmed missing session retires it; scope mismatches render a local notice.
    if (sessionId === null || deletedByOperator || !remotelyGone) return;
    toast.error("Session not found");
    void coordinator.userRetireSession(windowId);
  }, [coordinator, deletedByOperator, remotelyGone, sessionId, windowId]);

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
