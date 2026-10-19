import type { SessionPayload } from "../types";
import { SESSION_SIDEBAR_MIN_WIDTH } from "../lib/session-sidebar-width";
import { SessionList } from "./session-list/session-list";
import type { SessionListViewModel } from "../hooks/use-session-list-view";
import type { SessionLifecycleActionHandlers } from "../hooks/use-session-lifecycle-actions";

export interface SessionSidebarProps {
  open: boolean;
  sessions: readonly SessionPayload[];
  disconnected: boolean;
  collapsedThreadIds: readonly string[];
  currentSessionId?: string;
  /** The session on screen; a subagent session is revealed under its parent row. */
  revealedSession?: SessionPayload | null;
  view: SessionListViewModel;
  onToggleThread: (sessionId: string) => void;
  onSelectSession: (session: SessionPayload) => void;
  onNewSession: (options?: { folderId?: string }) => void;
  sessionActions: SessionLifecycleActionHandlers;
}

/**
 * In-window sessions rail: the same catalog body the dock's sessions surface
 * hosts, docked left of the transcript so the operator can switch sessions —
 * including spawned threads — without leaving the window. It fills its host:
 * `SessionSidebarLayout` owns the rail's width, resize and collapse.
 */
export function SessionSidebar({
  open,
  sessions,
  disconnected,
  collapsedThreadIds,
  currentSessionId,
  revealedSession,
  view,
  onToggleThread,
  onSelectSession,
  onNewSession,
  sessionActions,
}: SessionSidebarProps) {
  return (
    <aside
      aria-label="Sessions"
      data-state={open ? "open" : "closed"}
      data-testid="session-sidebar"
      inert={!open}
      className="flex h-full w-full flex-col overflow-hidden bg-sunken"
    >
      {/* Rows lay out at the rail's floor while it animates closed, so they clip instead of reflowing. */}
      <div
        className="flex h-full w-full flex-col pt-1.5"
        style={{ minWidth: SESSION_SIDEBAR_MIN_WIDTH }}
      >
        <SessionList
          view={view}
          sessions={sessions}
          disconnected={disconnected}
          collapsedThreadIds={collapsedThreadIds}
          currentSessionId={currentSessionId}
          revealedSession={revealedSession}
          onToggleThread={onToggleThread}
          onSelectSession={onSelectSession}
          onNewSession={onNewSession}
          sessionActions={sessionActions}
          testIdPrefix="session-sidebar"
        />
      </div>
    </aside>
  );
}
