import { ListTree, Plus, SquareKanban } from "lucide-react";
import { lazy, Suspense, type ReactNode } from "react";

import { Button, useTopbarSlot } from "@compozy/ui";

import {
  SESSION_BOARD_SORTS,
  SessionGroupByMenu,
  SessionsBoard,
  SessionSortMenu,
  SessionViewToggle,
  sessionBoardSort,
} from "@/systems/session";
import { useDataScope } from "@/systems/workspace";

import { OsSessionsDeriveHost } from "../../components/os-sessions-derive-host";
import { useSessionBoardWindow } from "./hooks/use-session-board-window";
import { useSessionWindowRuntime } from "./hooks/use-session-window-runtime";
import { SessionBoardDrawer } from "./session-board-drawer";

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

/**
 * The session window in Kanban mode (US-011): the body becomes the board for
 * the workspace / All projects scope and publishes its own head — "Sessions",
 * the view toggle, Group by, Sort by, "Hide child sessions" and "New session".
 * A card opens its session in the drawer beside the board (#1052): the window keeps
 * that session as its own, so List, a reload and a restore all show the same one. Its
 * context menu and `…` run the same verbs as a session list row.
 */
export function SessionBoardWindow({
  windowId,
  session,
}: {
  windowId: string;
  /** The window's session view, shown in the drawer while the route asks for it. */
  session: ReactNode;
}) {
  const scope = useDataScope();
  const runtime = useSessionWindowRuntime(windowId);
  const { sessionId } = runtime;
  const drawerOpen = runtime.drawerOpen && sessionId !== null;
  const {
    view,
    setView,
    order,
    board,
    lifecycle,
    actions,
    openForAgent,
    onOpen,
    openInDrawer,
    drawer,
  } = useSessionBoardWindow(windowId, sessionId, drawerOpen);

  useTopbarSlot({
    glyph: <SquareKanban />,
    crumb: "Sessions",
    overflow: <SessionViewToggle onValueChange={setView} value={view.view} />,
    actions: (
      <>
        <SessionGroupByMenu
          onValueChange={order.setGroupBy}
          options={board.groupByOptions}
          testId="sessions-board-group-by"
          value={board.groupBy}
        />
        <SessionSortMenu
          onValueChange={order.setSort}
          options={SESSION_BOARD_SORTS}
          testId="sessions-board-sort"
          value={sessionBoardSort(order.sort)}
        />
        <Button
          aria-pressed={view.hideChildren}
          data-testid="sessions-board-hide-children"
          onClick={() => view.setHideChildren(!view.hideChildren)}
          size="sm"
          type="button"
          variant="quiet"
        >
          <ListTree aria-hidden="true" data-icon="inline-start" />
          Hide child sessions
        </Button>
        <Button
          data-testid="sessions-board-new-session"
          onClick={() => openForAgent("")}
          size="sm"
          type="button"
          variant="secondary"
        >
          <Plus aria-hidden="true" data-icon="inline-start" />
          New session
        </Button>
      </>
    ),
  });

  const { deleteDialog, renameDialog } = lifecycle;
  return (
    <OsSessionsDeriveHost workspaceId={scope?.kind === "project" ? scope.workspaceId : null}>
      {() => (
        <>
          <SessionBoardDrawer
            actions={actions}
            onClose={drawer.close}
            onExpand={() => setView("list")}
            onOpenSession={openInDrawer}
            onStep={drawer.step}
            onWidthChange={view.setDrawerWidth}
            open={drawerOpen}
            returnFocus={drawer.returnFocus}
            session={session}
            sessionId={sessionId}
            width={view.drawerWidth}
          >
            <div
              className="flex min-h-0 min-w-0 flex-1 flex-col"
              ref={drawer.boardRef}
              {...drawer.boardPointerHandlers}
            >
              <SessionsBoard
                actions={actions}
                board={board}
                currentSessionId={sessionId}
                onOpen={onOpen}
              />
            </div>
          </SessionBoardDrawer>
          {deleteDialog.open && deleteDialog.session ? (
            <Suspense fallback={null}>
              <SessionDeleteDialog
                isDeleting={deleteDialog.isDeleting}
                onConfirm={deleteDialog.onConfirm}
                onOpenChange={deleteDialog.onOpenChange}
                onRetry={deleteDialog.onRetry}
                open
                results={deleteDialog.results}
                session={deleteDialog.session}
                sessions={deleteDialog.sessions}
              />
            </Suspense>
          ) : null}
          {renameDialog.open && renameDialog.session ? (
            <Suspense fallback={null}>
              <SessionRenameDialog
                isRenaming={renameDialog.isRenaming}
                onConfirm={renameDialog.onConfirm}
                onOpenChange={renameDialog.onOpenChange}
                open
                session={renameDialog.session}
              />
            </Suspense>
          ) : null}
        </>
      )}
    </OsSessionsDeriveHost>
  );
}
