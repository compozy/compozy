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

import { useAttentionJump } from "../hooks/use-attention-jump";
import { OsSessionsDeriveHost } from "./os-sessions-derive-host";
import {
  type SessionLifecycleActionHandlers,
  SessionList,
  type SessionPayload,
  useSessionSidebarState,
  type SessionListViewModel,
  SessionDeleteDialog,
  SessionRenameDialog,
  type useSessionLifecycleActions,
} from "@/systems/session";

export interface OsSessionsModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  dismissalBlocked?: boolean;
  sessions: readonly SessionPayload[];
  disconnected: boolean;
  /** Breadth, order, and widened workspace groups — owned by the shell, not fetched here. */
  view: SessionListViewModel;
  currentWorkspaceId?: string | null;
  onNewSession: () => void;
  sessionActions: SessionLifecycleActionHandlers;
}

/** Keeps the catalog mounted beneath its derive and lifecycle confirmation dialogs. */
export function OsSessionsDialogHost({
  open,
  onOpenChange,
  disconnected,
  view,
  currentWorkspaceId,
  onNewSession,
  lifecycle,
}: Pick<
  OsSessionsModalProps,
  "open" | "onOpenChange" | "disconnected" | "view" | "currentWorkspaceId" | "onNewSession"
> & {
  lifecycle: ReturnType<typeof useSessionLifecycleActions>;
}) {
  return (
    <>
      <OsSessionsDeriveHost
        workspaceId={currentWorkspaceId ?? null}
        onLanded={() => onOpenChange(false)}
      >
        {({ deriveOpen }) => (
          <OsSessionsModal
            open={open}
            onOpenChange={onOpenChange}
            dismissalBlocked={
              lifecycle.deleteDialog.open || lifecycle.renameDialog.open || deriveOpen
            }
            sessions={view.catalog?.sessions ?? []}
            disconnected={disconnected}
            view={view}
            currentWorkspaceId={currentWorkspaceId}
            onNewSession={onNewSession}
            sessionActions={lifecycle.actions}
          />
        )}
      </OsSessionsDeriveHost>
      {lifecycle.deleteDialog.session ? (
        <SessionDeleteDialog
          open={lifecycle.deleteDialog.open}
          onOpenChange={lifecycle.deleteDialog.onOpenChange}
          session={lifecycle.deleteDialog.session}
          sessions={lifecycle.deleteDialog.sessions}
          results={lifecycle.deleteDialog.results}
          onRetry={lifecycle.deleteDialog.onRetry}
          isDeleting={lifecycle.deleteDialog.isDeleting}
          onConfirm={lifecycle.deleteDialog.onConfirm}
        />
      ) : null}
      {lifecycle.renameDialog.session ? (
        <SessionRenameDialog
          open={lifecycle.renameDialog.open}
          onOpenChange={lifecycle.renameDialog.onOpenChange}
          session={lifecycle.renameDialog.session}
          isRenaming={lifecycle.renameDialog.isRenaming}
          onConfirm={lifecycle.renameDialog.onConfirm}
        />
      ) : null}
    </>
  );
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
  disconnected,
  view,
  currentWorkspaceId,
  onNewSession,
  sessionActions,
}: OsSessionsModalProps) {
  const jumpToSession = useAttentionJump();
  const { collapsedThreadIds, toggleThread } = useSessionSidebarState();

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen && dismissalBlocked) return;
    onOpenChange(nextOpen);
  };

  const selectSession = (session: SessionPayload) => {
    onOpenChange(false);
    jumpToSession({
      sessionId: session.id,
      agentName: session.agent_name,
      workspaceId: session.workspace_id ?? currentWorkspaceId ?? "",
    });
  };
  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent
        unframed
        showCloseButton={false}
        data-testid="os-sessions-modal"
        className="top-[9vh] flex h-[min(var(--height-modal-md),70vh)] max-h-[70vh] w-full translate-y-0 flex-col overflow-hidden shell-wide:top-[16vh] sm:w-detail-inspector-inline sm:max-w-none"
      >
        <DialogTitle className="sr-only">Sessions</DialogTitle>
        <DialogDescription className="sr-only">
          Filter sessions and open one in the current workspace.
        </DialogDescription>
        <SessionList
          view={view}
          sessions={sessions}
          disconnected={disconnected}
          collapsedThreadIds={collapsedThreadIds}
          onToggleThread={toggleThread}
          onSelectSession={selectSession}
          onNewSession={onNewSession}
          sessionActions={sessionActions}
          testIdPrefix="os-sessions-modal"
          header={visibleCount => (
            <div className="flex items-center gap-2 px-3 pt-3 pb-1.5">
              <Eyebrow className="min-w-0 flex-1 text-subtle">
                Sessions <span className="ml-1 text-faint">{visibleCount}</span>
              </Eyebrow>
              <Button
                type="button"
                variant="quiet"
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
