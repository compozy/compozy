import { lazy, Suspense } from "react";
import { MessagesSquare, Plus } from "lucide-react";

import { Button, Empty, useTopbarSlot } from "@compozy/ui";

import { useSessionWindowSidebar } from "./use-session-window-sidebar";
import { SessionPanelToggle, SessionSidebar } from "@/systems/session";

const SessionDeleteDialog = lazy(() =>
  import("@/systems/session/components/session-delete-dialog").then(module => ({
    default: module.SessionDeleteDialog,
  }))
);
const SessionRenameDialog = lazy(() =>
  import("@/systems/session/components/session-rename-dialog").then(module => ({
    default: module.SessionRenameDialog,
  }))
);

/** Keep catalog actions and their confirmation dialogs available without an active session. */
export function SessionWindowEmpty({
  windowId,
  workspaceId,
}: {
  windowId: string;
  workspaceId: string;
}) {
  const sidebar = useSessionWindowSidebar({
    windowId,
    workspaceId,
  });

  useTopbarSlot({
    crumb: <span className="text-muted">Sessions</span>,
    actions: <SessionPanelToggle panel="sidebar" open={sidebar.open} onToggle={sidebar.toggle} />,
  });

  return (
    <div className="flex min-h-0 min-w-0 flex-1 overflow-hidden">
      <SessionSidebar
        open={sidebar.open}
        sessions={sidebar.sessions}
        disconnected={sidebar.disconnected}
        collapsedThreadIds={sidebar.collapsedThreadIds}
        view={sidebar.view}
        onToggleThread={sidebar.onToggleThread}
        onSelectSession={sidebar.onSelectSession}
        onNewSession={sidebar.onNewSession}
        sessionActions={sidebar.sessionActions}
      />
      <div
        className="flex min-h-0 min-w-0 flex-1 items-center justify-center"
        data-testid="session-window-empty"
      >
        <Empty
          action={
            <Button
              data-testid="session-window-empty-new"
              onClick={sidebar.onNewSession}
              type="button"
              variant="primary"
            >
              <Plus aria-hidden="true" data-icon="inline-start" />
              New session
            </Button>
          }
          description="Pick one from the list, or start a new one."
          icon={MessagesSquare}
          title="No session selected"
        />
      </div>
      {sidebar.rowDeleteDialog.open && sidebar.rowDeleteDialog.session ? (
        <Suspense fallback={null}>
          <SessionDeleteDialog
            open
            onOpenChange={sidebar.rowDeleteDialog.onOpenChange}
            session={sidebar.rowDeleteDialog.session}
            sessions={sidebar.rowDeleteDialog.sessions}
            results={sidebar.rowDeleteDialog.results}
            onRetry={sidebar.rowDeleteDialog.onRetry}
            isDeleting={sidebar.rowDeleteDialog.isDeleting}
            onConfirm={sidebar.rowDeleteDialog.onConfirm}
          />
        </Suspense>
      ) : null}
      {sidebar.rowRenameDialog.open && sidebar.rowRenameDialog.session ? (
        <Suspense fallback={null}>
          <SessionRenameDialog
            open
            onOpenChange={sidebar.rowRenameDialog.onOpenChange}
            session={sidebar.rowRenameDialog.session}
            isRenaming={sidebar.rowRenameDialog.isRenaming}
            onConfirm={sidebar.rowRenameDialog.onConfirm}
          />
        </Suspense>
      ) : null}
    </div>
  );
}
