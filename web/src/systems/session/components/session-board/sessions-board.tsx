import { useState } from "react";

import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import type { SessionsBoardView } from "../../hooks/use-sessions-board";
import type { SessionBoardPayload } from "../../lib/session-board-contract";
import { SessionsBoardArchiveBar } from "./sessions-board-archive-bar";
import {
  SESSIONS_BOARD_FRAME,
  SESSIONS_BOARD_LANES,
  SESSIONS_BOARD_SCROLLER,
} from "./sessions-board-frame";
import { SessionsBoardLanesSkeleton } from "./sessions-board-skeleton";
import {
  SessionsBoardColumnsRow,
  SessionsBoardLanesFailure,
  SessionsBoardSwimlane,
} from "./sessions-board-swimlane";

export interface SessionsBoardProps {
  board: SessionsBoardView;
  currentSessionId?: string | null;
  onOpen: (session: SessionBoardPayload) => void;
  /** The session list's lifecycle verbs; every card then offers them. */
  actions?: SessionLifecycleActionHandlers;
}

/**
 * The Sessions board (`_uiux.md` S4): four derived columns in a horizontally
 * scrolling row on the canvas, plus the Archive bar. Grouped by project or
 * profile, each group becomes a swimlane holding its own row of the same
 * columns. Placement is the daemon's (`board_column`); there is no drag and drop.
 */
export function SessionsBoard({
  board,
  currentSessionId = null,
  onOpen,
  actions,
}: SessionsBoardProps) {
  // Done starts expanded (Open Question 1); the fold is local to this view.
  const [doneCollapsed, setDoneCollapsed] = useState(false);
  const row = {
    actions,
    currentSessionId,
    doneCollapsed,
    onDoneToggle: () => setDoneCollapsed(value => !value),
    onOpen,
    workspaceNameOf: board.workspaceNameOf,
  };
  return (
    <div className={SESSIONS_BOARD_FRAME} data-testid="sessions-board">
      <div className={SESSIONS_BOARD_SCROLLER}>
        {board.lanes === null ? (
          <SessionsBoardColumnsRow {...row} columns={board.columns} layout="fill" />
        ) : (
          <SessionsBoardLanes board={board} row={row} />
        )}
      </div>
      <SessionsBoardArchiveBar
        actions={actions}
        archive={board.archive}
        onOpen={onOpen}
        workspaceNameOf={board.workspaceNameOf}
      />
    </div>
  );
}

function SessionsBoardLanes({
  board,
  row,
}: {
  board: SessionsBoardView;
  row: Omit<React.ComponentProps<typeof SessionsBoardSwimlane>, "lane">;
}) {
  if (board.lanesError) return <SessionsBoardLanesFailure onRetry={board.lanesError.retry} />;
  if (board.lanesLoading) return <SessionsBoardLanesSkeleton />;
  if (board.lanes === null || board.lanes.length === 0) {
    return (
      <p
        className="px-3 py-8 text-center text-small-body text-muted"
        data-testid="sessions-board-lanes-empty"
      >
        No sessions yet.
      </p>
    );
  }
  return (
    <div className={SESSIONS_BOARD_LANES} data-testid="sessions-board-lanes">
      {board.lanes.map(lane => (
        <SessionsBoardSwimlane {...row} key={lane.group.key} lane={lane} />
      ))}
    </div>
  );
}
