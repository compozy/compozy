import { useEffect } from "react";
import { toast } from "sonner";

import type { OsShellHandle } from "../../contexts/os-shell-context";
import { useOsShell } from "../../hooks/use-os-shell";
import { useSessionWindowResolution } from "./hooks/use-session-window-resolution";
import { useSessionWindowRuntime } from "./hooks/use-session-window-runtime";
import { preloadSessionWindowModules } from "./session-window-module-loader";
import { SessionProfileOwnerNotice } from "./session-profile-owner-notice";
import { SessionWindowNotice, SessionWindowView } from "./session-window-view";

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
 * Session window controller: parses `agent + session` identity from the window's
 * WM location, then resolves the session under the active profile before
 * rehosting the session view.
 *
 * The primary read is the profile-enforced by-id route rather than the
 * workspace one, because only the former compares the session's owner against
 * the scope. Its 404 is what makes a foreign session distinguishable from a
 * deleted one — and until the aggregate lookup answers, this window commits to
 * neither: closing it while that request is in flight is what made a perfectly
 * visible session look deleted.
 */
export function SessionWindow({ windowId }: { windowId: string }) {
  const { coordinator } = useOsShell();
  const { runtimeWorkspaceId, liveTailEnabled, sessionId, agentName } =
    useSessionWindowRuntime(windowId);
  const { session, isLoading, error, scopedMiss, foreign, crossesWorkspace } =
    useSessionWindowResolution({ sessionId, runtimeWorkspaceId, liveTailEnabled });

  useEffect(() => {
    if (agentName === null) return;
    // Only a settled "nowhere" licenses the bounce. `loading` and `found` both
    // mean the operator is still owed an answer on this screen.
    const gone = scopedMiss && foreign.status === "missing";
    if (!gone && !crossesWorkspace) return;
    toast.error("Session not found");
    returnToAgent(coordinator, windowId, agentName);
  }, [agentName, coordinator, crossesWorkspace, foreign.status, scopedMiss, windowId]);

  if (sessionId === null || agentName === null) {
    return <SessionWindowNotice message="This window does not point at a session." />;
  }
  if (foreign.status === "found") {
    return (
      <SessionProfileOwnerNotice
        agentName={agentName}
        liveTailEnabled={liveTailEnabled}
        owner={foreign.owner}
        session={foreign.session}
        windowId={windowId}
      />
    );
  }
  if (foreign.status === "loading") {
    return (
      <SessionWindowView
        windowId={windowId}
        name={agentName}
        id={sessionId}
        workspaceId={runtimeWorkspaceId ?? ""}
        session={undefined}
        liveTailEnabled={liveTailEnabled}
        isLoading
        error={null}
        onDeleteSuccess={() => returnToAgent(coordinator, windowId, agentName)}
      />
    );
  }
  if (foreign.status === "error") {
    return <SessionWindowNotice message={foreign.error.message} />;
  }
  if (runtimeWorkspaceId === null || crossesWorkspace) {
    return <SessionWindowNotice message={error?.message ?? "Session workspace unavailable"} />;
  }

  return (
    <div className="flex min-h-full min-w-0 flex-col">
      <SessionWindowView
        windowId={windowId}
        name={agentName}
        id={sessionId}
        workspaceId={runtimeWorkspaceId}
        session={session}
        liveTailEnabled={liveTailEnabled}
        isLoading={isLoading}
        error={error}
        onDeleteSuccess={() => returnToAgent(coordinator, windowId, agentName)}
      />
    </div>
  );
}
