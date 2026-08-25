import { useEffect, useState } from "react";
import { toast } from "sonner";

import { useSessionPresence } from "@/systems/session";
import { useActiveWorkspace } from "@/systems/workspace";

import type { OsShellHandle } from "../../../contexts/os-shell-context";
import { useOsShell } from "../../../hooks/use-os-shell";
import { useSessionWindowResolution } from "./use-session-window-resolution";
import { useSessionWindowRuntime } from "./use-session-window-runtime";

function returnToAgent(
  coordinator: OsShellHandle["coordinator"],
  windowId: string,
  agentName: string
): void {
  void coordinator.userClose(windowId).then(closed => {
    if (!closed) return;
    void coordinator.userOpen({
      app: "agents",
      route: {
        pathname: `/agents/${encodeURIComponent(agentName)}`,
        search: {},
      },
    });
  });
}

/** Resolves the session window identity and owns its redirect/delete lifecycle. */
export function useSessionWindowController(windowId: string) {
  const { coordinator } = useOsShell();
  const activeWorkspace = useActiveWorkspace();
  const runtimeWorkspaceId = activeWorkspace.runtimeWorkspaceId?.trim() || null;
  const { liveTailEnabled, presenceEnabled, sessionId, agentName } =
    useSessionWindowRuntime(windowId);
  const [deletedSessionId, setDeletedSessionId] = useState<string | null>(null);
  const deletedLocally = sessionId !== null && deletedSessionId === sessionId;
  const effectiveLiveTailEnabled = liveTailEnabled && !deletedLocally;
  const resolution = useSessionWindowResolution({
    sessionId,
    runtimeWorkspaceId,
    liveTailEnabled: effectiveLiveTailEnabled,
  });
  useSessionPresence(resolution.workspaceId, sessionId, presenceEnabled && !deletedLocally);

  useEffect(() => {
    if (agentName === null || deletedLocally) return;
    const gone = resolution.scopedMiss && resolution.foreign.status === "missing";
    if (!gone && !resolution.crossesWorkspace) return;
    toast.error("Session not found");
    returnToAgent(coordinator, windowId, agentName);
  }, [
    agentName,
    coordinator,
    deletedLocally,
    resolution.crossesWorkspace,
    resolution.foreign.status,
    resolution.scopedMiss,
    windowId,
  ]);

  useEffect(() => {
    if (!deletedLocally || agentName === null) return;
    returnToAgent(coordinator, windowId, agentName);
  }, [agentName, coordinator, deletedLocally, windowId]);

  return {
    ...resolution,
    agentName,
    deletedLocally,
    effectiveLiveTailEnabled,
    liveTailEnabled,
    sessionId,
    handleDeleteSuccess: () => {
      if (sessionId !== null) setDeletedSessionId(sessionId);
    },
  };
}
