import { useQuery } from "@tanstack/react-query";
import { useEffect } from "react";
import { toast } from "sonner";

import { SessionNotFoundError, sessionDetailOptions } from "@/systems/session";
import { useActiveWorkspace } from "@/systems/workspace";

import type { OsShellHandle } from "../../contexts/os-shell-context";
import { useOsShell } from "../../hooks/use-os-shell";
import { matchSessionInstance } from "../../lib/app-registry";
import { useSessionWindowDesktopState } from "./hooks/use-session-window-desktop-state";
import { SessionWindowNotice, SessionWindowView } from "./session-window-view";

const SESSION_AGENT_PATTERN = /^\/agents\/([^/]+)\/sessions\//;

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

/**
 * Session window controller: parses `agent + session` identity from
 * the window's WM location, then resolves the session inside the active
 * workspace before rehosting the existing session view.
 */
export function SessionWindow({ windowId }: { windowId: string }) {
  const { coordinator } = useOsShell();
  const { activeWorkspaceId } = useActiveWorkspace();
  const { liveTailEnabled, pathname } = useSessionWindowDesktopState(windowId);
  const sessionId = matchSessionInstance(pathname);
  const agentMatch = SESSION_AGENT_PATTERN.exec(pathname);
  const agentName = agentMatch ? decodeURIComponent(agentMatch[1]) : null;

  const sessionQuery = useQuery({
    ...sessionDetailOptions(activeWorkspaceId ?? "", sessionId ?? ""),
    enabled: activeWorkspaceId !== null && sessionId !== null,
  });
  const sessionWorkspaceId = sessionQuery.data?.workspace_id?.trim();
  const crossesWorkspace =
    sessionWorkspaceId !== undefined &&
    activeWorkspaceId !== null &&
    sessionWorkspaceId !== activeWorkspaceId;

  useEffect(() => {
    if (
      (!(sessionQuery.error instanceof SessionNotFoundError) && !crossesWorkspace) ||
      agentName === null
    ) {
      return;
    }
    toast.error("Session not found");
    returnToAgent(coordinator, windowId, agentName);
  }, [agentName, coordinator, crossesWorkspace, sessionQuery.error, windowId]);

  if (sessionId === null || agentName === null) {
    return <SessionWindowNotice message="This window does not point at a session." />;
  }
  if (activeWorkspaceId === null || crossesWorkspace) {
    return (
      <SessionWindowNotice
        message={sessionQuery.error?.message ?? "Session workspace unavailable"}
      />
    );
  }

  return (
    <div className="flex min-h-full min-w-0 flex-col">
      <SessionWindowView
        windowId={windowId}
        name={agentName}
        id={sessionId}
        workspaceId={activeWorkspaceId}
        session={sessionQuery.data}
        liveTailEnabled={liveTailEnabled}
        isLoading={sessionQuery.isLoading}
        error={sessionQuery.error}
        onDeleteSuccess={() => returnToAgent(coordinator, windowId, agentName)}
      />
    </div>
  );
}
