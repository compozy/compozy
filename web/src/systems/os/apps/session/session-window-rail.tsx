import { useState, type ReactNode } from "react";

import {
  type SessionPayload,
  SessionSidebar,
  SessionSidebarLayout,
  useSessionWindowPresentation,
} from "@/systems/session";

import { SessionWindowRowDialogs } from "./session-window-row-dialogs";
import { SessionWindowRailContext } from "./use-session-window-rail";
import {
  type SessionWindowRailTransport,
  useSessionWindowSidebar,
} from "./use-session-window-sidebar";

/** The sessions rail beside the session pane; over the board the board is the list, so no rail. */
function SessionWindowRailLayout({
  sidebar,
  children,
}: {
  sidebar: ReactNode;
  children: ReactNode;
}) {
  if (useSessionWindowPresentation() === "board-drawer") {
    return <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">{children}</div>;
  }
  return <SessionSidebarLayout sidebar={sidebar}>{children}</SessionSidebarLayout>;
}

/**
 * The session window's sessions rail and the pane it frames (#1923). The rail belongs to the
 * window, not to the session it shows: it sits outside the pane's per-session keyed boundary, so
 * picking a row swaps only the pane while the list keeps its scroll, its row state and focus,
 * through the next session's loading state as well. The pane reports its session's stream back
 * through context, since the rail is outside that session's runtime providers.
 */
export function SessionWindowRail({
  windowId,
  sessionId,
  session,
  children,
}: {
  windowId: string;
  sessionId: string;
  /** The session on screen once it has loaded; a subagent session is revealed under its parent. */
  session: SessionPayload | undefined;
  /** The session pane. */
  children: ReactNode;
}) {
  const [transport, reportTransport] = useState<SessionWindowRailTransport | null>(null);
  const sidebar = useSessionWindowSidebar({ sessionId, transport, windowId });
  // While the next session loads, the list keeps the last one revealed, so a subagent row it
  // revealed does not drop out and shift the rows under the pointer.
  const [lastSession, setLastSession] = useState(session);
  if (session !== undefined && session !== lastSession) setLastSession(session);

  return (
    <SessionWindowRailContext value={{ sidebar, reportTransport }}>
      <div className="flex min-h-0 min-w-0 flex-1 overflow-hidden">
        <SessionWindowRailLayout
          sidebar={
            <SessionSidebar
              open={sidebar.open}
              sessions={sidebar.sessions}
              disconnected={sidebar.disconnected}
              collapsedThreadIds={sidebar.collapsedThreadIds}
              subagentThreadIds={sidebar.subagentThreadIds}
              view={sidebar.view}
              currentSessionId={sessionId}
              revealedSession={session ?? lastSession}
              onThreadBranchChange={sidebar.onThreadBranchChange}
              onSelectSession={sidebar.onSelectSession}
              onNewSession={sidebar.onNewSession}
              sessionActions={sidebar.sessionActions}
            />
          }
        >
          {children}
        </SessionWindowRailLayout>
        <SessionWindowRowDialogs sidebar={sidebar} />
      </div>
    </SessionWindowRailContext>
  );
}
