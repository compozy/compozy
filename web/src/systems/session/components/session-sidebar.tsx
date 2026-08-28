import { cn } from "@/lib/utils";

import type { SessionPayload } from "../types";
import { SessionList } from "./session-list/session-list";
import type { SessionListViewModel } from "../hooks/use-session-list-view";
import type { SessionLifecycleActionHandlers } from "../hooks/use-session-lifecycle-actions";

export interface SessionSidebarProps {
  open: boolean;
  sessions: readonly SessionPayload[];
  disconnected: boolean;
  collapsedThreadIds: readonly string[];
  currentSessionId?: string;
  view: SessionListViewModel;
  onToggleThread: (sessionId: string) => void;
  onSelectSession: (session: SessionPayload) => void;
  onNewSession: () => void;
  sessionActions: SessionLifecycleActionHandlers;
}

/**
 * In-window sessions rail: the same catalog body the dock's sessions surface
 * hosts, docked left of the transcript so the operator can switch sessions —
 * including spawned threads — without leaving the window.
 */
export function SessionSidebar({
  open,
  sessions,
  disconnected,
  collapsedThreadIds,
  currentSessionId,
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
      className={cn(
        "flex shrink-0 flex-col overflow-hidden bg-rail transition-[width] duration-shell-slow motion-reduce:transition-none",
        open ? "w-66 border-r border-line" : "w-0"
      )}
    >
      <div className="flex h-full w-66 min-w-0 flex-col pt-1.5">
        <SessionList
          view={view}
          sessions={sessions}
          disconnected={disconnected}
          collapsedThreadIds={collapsedThreadIds}
          currentSessionId={currentSessionId}
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
