import { Plus } from "lucide-react";

import { Icon } from "@compozy/ui";

import { cn } from "@/lib/utils";

import type { SessionPayload } from "../types";
import { SessionList } from "./session-list/session-list";
import type { SessionListViewModel } from "../hooks/use-session-list-view";
import type { SessionLifecycleActionHandlers } from "../hooks/use-session-lifecycle-actions";

export interface SessionSidebarProps {
  open: boolean;
  sessions: readonly SessionPayload[];
  disconnected: boolean;
  collapsedAgentIds: readonly string[];
  collapsedThreadIds: readonly string[];
  currentSessionId: string;
  view: SessionListViewModel;
  onToggleGroup: (agentName: string) => void;
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
  collapsedAgentIds,
  collapsedThreadIds,
  currentSessionId,
  view,
  onToggleGroup,
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
          collapsedAgentIds={collapsedAgentIds}
          collapsedThreadIds={collapsedThreadIds}
          currentSessionId={currentSessionId}
          onToggleGroup={onToggleGroup}
          onToggleThread={onToggleThread}
          onSelectSession={onSelectSession}
          sessionActions={sessionActions}
          testIdPrefix="session-sidebar"
          footer={
            <button
              type="button"
              className="flex w-full items-center gap-2 border-t border-line-soft px-3 py-2 text-small-body font-medium text-subtle transition-colors hover:bg-row-hover hover:text-fg focus-visible:shadow-focus-ring focus-visible:outline-none"
              data-testid="session-sidebar-new-session"
              onClick={onNewSession}
            >
              <Icon as={Plus} size="sm" />
              New session
            </button>
          }
        />
      </div>
    </aside>
  );
}
