import { MessagesSquare, Plus } from "@compozy/ui/icons";

import { Button, Empty, useTopbarSlot } from "@compozy/ui";

import { useSessionWindowLifecycle } from "./hooks/use-session-window-lifecycle";
import { useSessionWindowSidebar } from "./use-session-window-sidebar";
import {
  SessionPanelToggle,
  SessionSidebar,
  SessionSidebarLayout,
  SessionViewToggle,
  useSessionWindowViewState,
} from "@/systems/session";

/**
 * Keep catalog actions available without an active session. The delete/process/later/rename
 * confirmations stay with the window's lifecycle host, above the content/empty branch: an
 * empty body must not take an in-flight confirmation down with it.
 */
export function SessionWindowEmpty({ windowId }: { windowId: string }) {
  const lifecycle = useSessionWindowLifecycle();
  const sidebar = useSessionWindowSidebar({ windowId, lifecycle: lifecycle ?? undefined });
  const view = useSessionWindowViewState();

  useTopbarSlot({
    crumb: <span className="text-muted-foreground">Sessions</span>,
    overflow: <SessionViewToggle onValueChange={view.setView} value={view.view} />,
    start: <SessionPanelToggle panel="sidebar" open={sidebar.open} onToggle={sidebar.toggle} />,
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
    </div>
  );
}
