import { X } from "lucide-react";

import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
  Eyebrow,
  Icon,
} from "@compozy/ui";

import {
  SessionList,
  useSessionSidebarState,
  type SessionLifecycleActionHandlers,
  type SessionPayload,
} from "@/systems/session";

import { useOsSessionsModal } from "../hooks/use-os-sessions-modal";

export interface OsSessionsModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  dismissalBlocked?: boolean;
  sessions: readonly SessionPayload[];
  archivedSessions: readonly SessionPayload[];
  archivedTotal?: number;
  disconnected: boolean;
  sessionActions: SessionLifecycleActionHandlers;
}

/**
 * Global sessions catalog (shell-level Dialog). Shares the session-list body —
 * including provenance threads — with the in-window sessions sidebar; chrome
 * matches ⌘K (portal, scrim, top offset).
 */
export function OsSessionsModal({
  open,
  onOpenChange,
  dismissalBlocked = false,
  sessions,
  archivedSessions,
  archivedTotal,
  disconnected,
  sessionActions,
}: OsSessionsModalProps) {
  const { coordinator, manager, collapsedAgentIds } = useOsSessionsModal();
  const { collapsedThreadIds, toggleThread } = useSessionSidebarState();

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen && dismissalBlocked) return;
    onOpenChange(nextOpen);
  };

  const selectSession = (session: SessionPayload) => {
    onOpenChange(false);
    void coordinator.userOpen({
      app: "session",
      instanceKey: session.id,
      route: {
        pathname: `/agents/${encodeURIComponent(session.agent_name)}/sessions/${encodeURIComponent(session.id)}`,
        search: {},
      },
    });
  };
  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent
        unframed
        showCloseButton={false}
        data-testid="os-sessions-modal"
        className="top-[9vh] flex h-[min(var(--height-modal-md),70vh)] max-h-[70vh] w-full translate-y-0 flex-col overflow-hidden min-[960px]:top-[16vh] sm:w-detail-inspector-inline sm:max-w-none"
      >
        <DialogTitle className="sr-only">Sessions</DialogTitle>
        <DialogDescription className="sr-only">
          Filter sessions and open one in the current workspace.
        </DialogDescription>
        <SessionList
          sessions={sessions}
          archivedSessions={archivedSessions}
          archivedTotal={archivedTotal}
          disconnected={disconnected}
          collapsedAgentIds={collapsedAgentIds}
          collapsedThreadIds={collapsedThreadIds}
          onToggleGroup={agentName => manager.toggleRailGroup(agentName)}
          onToggleThread={toggleThread}
          onSelectSession={selectSession}
          sessionActions={sessionActions}
          testIdPrefix="os-sessions-modal"
          header={visibleCount => (
            <div className="flex items-center gap-2 px-3 pt-3 pb-1.5">
              <Eyebrow className="min-w-0 flex-1 text-subtle">
                Sessions <span className="ml-1 text-faint">{visibleCount}</span>
              </Eyebrow>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label="Close sessions"
                onClick={() => handleOpenChange(false)}
              >
                <Icon as={X} size="sm" />
              </Button>
            </div>
          )}
        />
      </DialogContent>
    </Dialog>
  );
}
