import { ListTree, Plus, SquareKanban } from "lucide-react";
import { lazy, Suspense } from "react";

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
const SessionRenameDialog = lazy(() =>
  import("@/systems/session/components/session-rename-dialog").then(module => ({
    default: module.SessionRenameDialog,
  }))
);

/**
 * The session window in Kanban mode (US-011): the body becomes the board for
 * the workspace / All projects scope and publishes its own head — "Sessions",
 * the view toggle, Group by, Sort by, "Hide child sessions" and "New session".
 * A card switches the window back to List with that session open; its context
 * menu and `…` run the same verbs as a session list row.
 */
export function SessionBoardWindow({
  windowId,
  sessionId,
}: {
  windowId: string;
  /** The session this window shows in List mode; its card reads as current. */
  sessionId: string | null;
}) {
  const scope = useDataScope();
  const { view, order, board, lifecycle, openForAgent, onOpen } = useSessionBoardWindow(
    windowId,
    sessionId
  );

  useTopbarSlot({
    glyph: <SquareKanban />,
    crumb: "Sessions",
    overflow: <SessionViewToggle onValueChange={view.setView} value={view.view} />,
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

  const { deleteDialog, processDialog, renameDialog } = lifecycle;
  return (
    <OsSessionsDeriveHost workspaceId={scope?.kind === "project" ? scope.workspaceId : null}>
      {() => (
        <>
          <SessionsBoard
            actions={lifecycle.actions}
            board={board}
            currentSessionId={sessionId}
            onOpen={onOpen}
          />
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
          {processDialog.open ? (
            <Suspense fallback={null}>
              <SessionProcessChoiceDialog {...processDialog} />
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
