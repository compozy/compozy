import { useSessionWindowController } from "./hooks/use-session-window-controller";
import { preloadSessionWindowModules } from "./session-window-module-loader";
import { SessionProfileOwnerNotice } from "./session-profile-owner-notice";
import { SessionWindowNotice, SessionWindowView } from "./session-window-view";

// This controller is itself route-local and lazy. Once it is requested, warm
// the chat surface chunks together so nested lazy boundaries do not waterfall.
void Promise.allSettled([preloadSessionWindowModules()]);

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
  const {
    agentName,
    crossesWorkspace,
    deletedLocally,
    effectiveLiveTailEnabled,
    error,
    foreign,
    handleDeleteSuccess,
    isLoading,
    liveTailEnabled,
    session,
    sessionId,
    workspaceId,
  } = useSessionWindowController(windowId);

  if (deletedLocally) return null;

  if (sessionId === null || agentName === null) {
    return <SessionWindowNotice message="This window does not point at a session." />;
  }
  if (foreign.status === "found") {
    return (
      <SessionProfileOwnerNotice
        agentName={agentName}
        liveTailEnabled={effectiveLiveTailEnabled}
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
        workspaceId={workspaceId ?? ""}
        session={undefined}
        liveTailEnabled={liveTailEnabled}
        isLoading
        error={null}
        onDeleteSuccess={handleDeleteSuccess}
      />
    );
  }
  if (foreign.status === "error") {
    return <SessionWindowNotice message={foreign.error.message} />;
  }
  if (workspaceId === null || crossesWorkspace) {
    return <SessionWindowNotice message={error?.message ?? "Session workspace unavailable"} />;
  }

  return (
    <div className="flex min-h-full min-w-0 flex-col">
      <SessionWindowView
        windowId={windowId}
        name={agentName}
        id={sessionId}
        workspaceId={workspaceId}
        session={session}
        liveTailEnabled={effectiveLiveTailEnabled}
        isLoading={isLoading}
        error={error}
        onDeleteSuccess={handleDeleteSuccess}
      />
    </div>
  );
}
