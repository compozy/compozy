import { SessionMenuOpenContext } from "../../contexts/session-menu-open-context-value";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { useSessionBoardColumnFolds } from "../../hooks/use-session-window-view-state";
import type { SessionsBoardView as SessionsBoardModel } from "../../hooks/use-sessions-board";
import type { SessionBoardPayload } from "../../lib/session-board-contract";
import type { SessionListTarget } from "../../types";
import { AlwaysOnListTop } from "../always-on/always-on-list-top";
import { SessionBoardCard } from "./session-board-card";
import { SessionBoardCardSlot } from "./session-board-card-slot";
import { SessionsBoardSwimlane } from "./sessions-board-swimlane";
import { SessionsBoardView } from "./sessions-board-view";

export interface SessionsBoardProps {
  board: SessionsBoardModel;
  currentSessionId?: string | null;
  onOpen: (session: SessionBoardPayload) => void;
  /** The session list's lifecycle verbs; every card then offers them. */
  actions?: SessionLifecycleActionHandlers;
}

/**
 * The Sessions board (`_uiux.md` S4): the derived columns in a horizontally
 * scrolling row on the canvas, plus the Archive bar. Column folds persist with the window. Grouped by project or
 * profile, each group becomes a swimlane holding its own row of the same
 * columns. Placement is the daemon's (`board_column`); there is no drag and drop.
 * This container binds the live cards and lanes to `SessionsBoardView`.
 */
export function SessionsBoard(props: SessionsBoardProps) {
  const folds = useSessionBoardColumnFolds();
  // A card's menu opens a rolled-up item's child session in the drawer, like a card click.
  const openTarget = (session: SessionListTarget) => props.onOpen(session as SessionBoardPayload);
  return (
    <SessionMenuOpenContext value={openTarget}>
      <SessionBoardCardSlot value={SessionBoardCard}>
        <SessionsBoardView
          {...props}
          collapsedColumns={folds.collapsed}
          onColumnCollapsedChange={folds.setCollapsed}
          header={
            <AlwaysOnListTop
              currentSessionId={props.currentSessionId}
              groupBy={props.board.groupBy}
              onOpenSession={openTarget}
              surface="ring-canvas"
              workspaceId={props.board.focusedWorkspaceId}
            />
          }
          renderLane={(lane, row) => (
            <SessionsBoardSwimlane {...row} key={lane.group.key} lane={lane} />
          )}
        />
      </SessionBoardCardSlot>
    </SessionMenuOpenContext>
  );
}
