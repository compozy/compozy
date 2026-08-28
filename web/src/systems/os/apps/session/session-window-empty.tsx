import { lazy, Suspense } from "react";
import { List, MessagesSquare } from "lucide-react";

import { Button, cn, Empty, useTopbarSlot } from "@compozy/ui";

import { useSessionWindowSidebar } from "./use-session-window-sidebar";
import { SessionSidebar } from "@/systems/session";

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
    actions: (
      <Button
        type="button"
        variant="ghost"
        size="icon-sm"
        aria-label={sidebar.open ? "Close sessions sidebar" : "Open sessions sidebar"}
        aria-pressed={sidebar.open}
        className={cn(
          "size-11 focus-visible:shadow-focus-inset",
          sidebar.open ? "bg-elevated text-fg" : null
        )}
        data-state={sidebar.open ? "open" : "closed"}
        data-testid="session-sidebar-toggle"
        onClick={sidebar.toggle}
      >
        <List aria-hidden="true" className="size-3" />
      </Button>
    ),
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
        <Empty icon={MessagesSquare} title="No session selected" />
      </div>
      {sidebar.rowDeleteDialog.open && sidebar.rowDeleteDialog.session ? (
        <Suspense fallback={null}>
          <SessionDeleteDialog
            open
            onOpenChange={sidebar.rowDeleteDialog.onOpenChange}
            session={sidebar.rowDeleteDialog.session}
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
