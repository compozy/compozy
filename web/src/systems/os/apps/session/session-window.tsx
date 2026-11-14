import { useSessionWindowViewState } from "@/systems/session";

import { useSessionWindowController } from "./hooks/use-session-window-controller";
import { SessionBoardWindow } from "./session-board-window";
import { preloadSessionWindowModules } from "./session-window-module-loader";
import { SessionWindowLifecycleHost } from "./session-window-lifecycle";
import { useSessionWindowLifecycle } from "./hooks/use-session-window-lifecycle";
import { SessionProfileOwnerNotice } from "./session-profile-owner-notice";
import { SessionWindowEmpty } from "./session-window-empty";
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

const PROJECT_UNAVAILABLE = "This session's project isn't available right now";

export function SessionWindow({ windowId }: { windowId: string }) {
  const { view } = useSessionWindowViewState();
  // On Kanban the board owns the window body; it shows the window's session in its drawer
  // (#1052) through `SessionWindowSession`, so the session resolves only while it is shown.
  if (view === "kanban") {
    return (
      <SessionBoardWindow
        windowId={windowId}
        session={<SessionWindowSession windowId={windowId} />}
      />
    );
  }
  return <SessionWindowSession windowId={windowId} />;
}

/**
 * The window's session at full size, or inside the board drawer: the one session view both
 * hosts mount, resolution and notices included. The lifecycle host sits above this branch so
 * a delete confirmation outlives the session it deletes.
 */
export function SessionWindowSession({ windowId }: { windowId: string }) {
  return (
    <SessionWindowLifecycleHost windowId={windowId}>
      <SessionWindowSessionBody windowId={windowId} />
    </SessionWindowLifecycleHost>
  );
}

function SessionWindowSessionBody({ windowId }: { windowId: string }) {
  const lifecycle = useSessionWindowLifecycle();
  const {
    agentName,
    crossesWorkspace,
    deletedLocally,
    effectiveLiveTailEnabled,
    error,
    foreign,
    isLoading,
    focused,
    liveTailEnabled,
    retainingSession,
    session,
    sessionId,
    workspaceId,
  } = useSessionWindowController(windowId, {
    // The delete this window confirmed is still running: its targets stand in for the
    // resolution while the request clears the session out of the cache.
    deletingSessions: lifecycle?.deleteDialog.isDeleting ? lifecycle.deleteDialog.sessions : null,
  });

  if (sessionId === null) {
    return <SessionWindowEmpty windowId={windowId} />;
  }
  // The delete this window confirmed is still in flight. Keep the last resolved session on
  // screen instead of swapping to skeleton/notice/empty until the request settles.
  if (retainingSession && session) {
    return (
      <SessionWindowBody>
        <SessionWindowView
          windowId={windowId}
          name={agentName ?? ""}
          id={sessionId}
          workspaceId={workspaceId ?? ""}
          session={session}
          focused={focused}
          liveTailEnabled={effectiveLiveTailEnabled}
          isLoading={false}
          error={null}
        />
      </SessionWindowBody>
    );
  }
  if (deletedLocally) {
    return <SessionWindowEmpty windowId={windowId} />;
  }
  if (agentName === null) {
    return <SessionWindowEmpty windowId={windowId} />;
  }
  if (crossesWorkspace) {
    return <SessionWindowNotice title="This session is outside the current project" />;
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
      <SessionWindowBody>
        <SessionWindowView
          windowId={windowId}
          name={agentName}
          id={sessionId}
          workspaceId={workspaceId ?? ""}
          session={undefined}
          focused={focused}
          liveTailEnabled={liveTailEnabled}
          isLoading
          error={null}
        />
      </SessionWindowBody>
    );
  }
  if (foreign.status === "error") {
    return (
      <SessionWindowNotice title="Couldn't open this session" detail={foreign.error.message} />
    );
  }
  if (workspaceId === null) {
    return <SessionWindowNotice title={PROJECT_UNAVAILABLE} detail={error?.message} />;
  }

  return (
    <SessionWindowBody>
      <SessionWindowView
        windowId={windowId}
        name={agentName}
        id={sessionId}
        workspaceId={workspaceId}
        session={session}
        focused={focused}
        liveTailEnabled={effectiveLiveTailEnabled}
        isLoading={isLoading}
        error={error}
      />
    </SessionWindowBody>
  );
}

/** The frame every session view fills, loading or loaded, so the two share one geometry. */
function SessionWindowBody({ children }: { children: React.ReactNode }) {
  return <div className="flex min-h-full min-w-0 flex-col">{children}</div>;
}
