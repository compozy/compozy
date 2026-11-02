import { lazy, Suspense } from "react";
import { MessagesSquare, Plus } from "@compozy/ui/icons";

import { Button, Empty, useTopbarSlot } from "@compozy/ui";

import { useSessionWindowSidebar } from "./use-session-window-sidebar";
import {
  SessionPanelToggle,
  SessionSidebar,
  SessionSidebarLayout,
  SessionViewToggle,
  useSessionWindowViewState,
} from "@/systems/session";

const SessionProcessChoiceDialog = lazy(() =>
  import("@/systems/session/components/session-process-choice-dialog").then(module => ({
    default: module.SessionProcessChoiceDialog,
  }))
);
const SessionDeleteDialog = lazy(() =>
  import("@/systems/session/components/session-delete-dialog").then(module => ({
    default: module.SessionDeleteDialog,
  }))
);
const SessionLaterDialog = lazy(() =>
  import("@/systems/session/components/session-later-dialog").then(module => ({
    default: module.SessionLaterDialog,
  }))
);
const SessionRenameDialog = lazy(() =>
  import("@/systems/session/components/session-rename-dialog").then(module => ({
    default: module.SessionRenameDialog,
  }))
);

/** Keep catalog actions and their confirmation dialogs available without an active session. */
export function SessionWindowEmpty({ windowId }: { windowId: string }) {
  const sidebar = useSessionWindowSidebar({ windowId });
  const view = useSessionWindowViewState();

  useTopbarSlot({
    crumb: <span className="text-muted-foreground">Sessions</span>,
    overflow: <SessionViewToggle onValueChange={view.setView} value={view.view} />,
    actions: <SessionPanelToggle panel="sidebar" open={sidebar.open} onToggle={sidebar.toggle} />,
  });

  return (
    <div className="flex min-h-0 min-w-0 flex-1 overflow-hidden">
      <SessionSidebarLayout
        sidebar={
          <SessionSidebar
            open={sidebar.open}
            sessions={sidebar.sessions}
            disconnected={sidebar.disconnected}
            collapsedThreadIds={sidebar.collapsedThreadIds}
            subagentThreadIds={sidebar.subagentThreadIds}
            view={sidebar.view}
            onThreadBranchChange={sidebar.onThreadBranchChange}
            onSelectSession={sidebar.onSelectSession}
            onNewSession={sidebar.onNewSession}
            sessionActions={sidebar.sessionActions}
          />
        }
      >
        <div
          className="flex min-h-0 min-w-0 flex-1 items-center justify-center"
          data-testid="session-window-empty"
        >
          <Empty
            action={
              <Button
                data-testid="session-window-empty-new"
                onClick={() => sidebar.onNewSession()}
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
      </SessionSidebarLayout>
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
            childSessions={sidebar.rowDeleteDialog.childSessions}
            onConfirm={sidebar.rowDeleteDialog.onConfirm}
          />
        </Suspense>
      ) : null}
      {sidebar.rowProcessDialog.open ? (
        <Suspense fallback={null}>
          <SessionProcessChoiceDialog {...sidebar.rowProcessDialog} />
        </Suspense>
      ) : null}
      {sidebar.rowLaterDialog.open ? (
        <Suspense fallback={null}>
          <SessionLaterDialog {...sidebar.rowLaterDialog} />
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
