import { useQuery } from "@tanstack/react-query";
import { useEffect } from "react";
import { toast } from "sonner";

import type { OsShellHandle } from "../../contexts/os-shell-context";
import { useOsShell } from "../../hooks/use-os-shell";
import { useSessionWindowRuntime } from "./hooks/use-session-window-runtime";
import { preloadSessionWindowModules } from "./session-window-module-loader";
import { SessionWindowNotice, SessionWindowView } from "./session-window-view";
import { sessionDetailOptions, SessionNotFoundError } from "@/systems/session";

// This controller is itself route-local and lazy. Once it is requested, warm
// the chat surface chunks together so nested lazy boundaries do not waterfall.
void Promise.allSettled([preloadSessionWindowModules()]);

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
  const { runtimeWorkspaceId, liveTailEnabled, sessionId, agentName } =
    useSessionWindowRuntime(windowId);

  const detailQueryOptions = sessionDetailOptions(runtimeWorkspaceId ?? "", sessionId ?? "");
  const sessionQuery = useQuery({
    ...detailQueryOptions,
    enabled: runtimeWorkspaceId !== null && sessionId !== null,
    refetchInterval: liveTailEnabled ? detailQueryOptions.refetchInterval : false,
  });
  const sessionWorkspaceId = sessionQuery.data?.workspace_id?.trim();
  const crossesWorkspace =
    sessionWorkspaceId !== undefined &&
    runtimeWorkspaceId !== null &&
    sessionWorkspaceId !== runtimeWorkspaceId;

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
  if (runtimeWorkspaceId === null || crossesWorkspace) {
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
        workspaceId={runtimeWorkspaceId}
        session={sessionQuery.data}
        liveTailEnabled={liveTailEnabled}
        isLoading={sessionQuery.isLoading}
        error={sessionQuery.error}
        onDeleteSuccess={() => returnToAgent(coordinator, windowId, agentName)}
      />
    </div>
  );
}
