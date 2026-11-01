import { TriangleAlert } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import { Alert, AlertActions, AlertDescription, Button } from "@compozy/ui";

import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import type { SessionsBoardView as SessionsBoardModel } from "../../hooks/use-sessions-board";
import type { SessionBoardPayload } from "../../lib/session-board-contract";
import { SessionsBoardShelfBar } from "./sessions-board-shelf-bar";
import {
  SessionsBoardColumnsRow,
  SessionsBoardLanesFailure,
  type SessionsBoardColumnsRowProps,
} from "./sessions-board-columns-row";
import {
  SESSIONS_BOARD_FRAME,
  SESSIONS_BOARD_LANES,
  SESSIONS_BOARD_SCROLLER,
} from "./sessions-board-frame";
import { SessionsBoardLanesSkeleton } from "./sessions-board-skeleton";

/** What every columns row of the board shares: the column folds, the card verbs and the opener. */
export type SessionsBoardRow = Omit<SessionsBoardColumnsRowProps, "columns" | "layout">;

export interface SessionsBoardViewProps {
  board: SessionsBoardModel;
  currentSessionId?: string | null;
  onOpen: (session: SessionBoardPayload) => void;
  /** The session list's lifecycle verbs; every card then offers them. */
  actions?: SessionLifecycleActionHandlers;
  /** Folded columns by id: one fold covers the column in every swimlane. */
  collapsedColumns: SessionsBoardRow["collapsedColumns"];
  onColumnCollapsedChange: SessionsBoardRow["onColumnCollapsedChange"];
  /** One swimlane per Group-by group; each lane reads its own columns. */
  renderLane?: (
    lane: NonNullable<SessionsBoardModel["lanes"]>[number],
    row: SessionsBoardRow
  ) => ReactNode;
}

/**
 * The Sessions board's presentation (`_uiux.md` S4): the derived columns in a horizontally
 * scrolling row on the canvas, plus the Later and Archive bars, or one swimlane per group through
 * `renderLane`. Cards come from `SessionBoardCardSlot`, so this view reads no query itself.
 */
export function SessionsBoardView({
  board,
  currentSessionId = null,
  onOpen,
  actions,
  collapsedColumns,
  onColumnCollapsedChange,
  renderLane,
}: SessionsBoardViewProps) {
  const row: SessionsBoardRow = {
    actions,
    currentSessionId,
    collapsedColumns,
    onColumnCollapsedChange,
    onOpen,
    workspaceNameOf: board.workspaceNameOf,
  };
  return (
    <div className={SESSIONS_BOARD_FRAME} data-testid="sessions-board">
      <div className={SESSIONS_BOARD_SCROLLER}>
        {board.lanes === null ? (
          <SessionsBoardColumnsRow
            {...row}
            columns={board.columns}
            layout="fill"
            selectionRow={{ key: "board", rank: 0 }}
          />
        ) : (
          <SessionsBoardLanes board={board} renderLane={renderLane} row={row} />
        )}
      </div>
      <SessionsBoardShelfBar
        actions={actions}
        onOpen={onOpen}
        page={board.later}
        shelf="later"
        workspaceNameOf={board.workspaceNameOf}
      />
      <SessionsBoardShelfBar
        actions={actions}
        onOpen={onOpen}
        page={board.archive}
        shelf="archive"
        workspaceNameOf={board.workspaceNameOf}
      />
    </div>
  );
}

/** The saved project order failed to load: the retry shows with or without lanes. */
function SessionsBoardLayoutWarning({ error }: { error: SessionsBoardModel["layoutError"] }) {
  if (!error) return null;
  return (
    <Alert className="w-auto px-2.5 py-2" role="status" variant="warning">
      <TriangleAlert aria-hidden="true" />
      <AlertDescription>Couldn&rsquo;t load folders.</AlertDescription>
      <AlertActions>
        <Button onClick={error.retry} size="xs" type="button" variant="secondary">
          Retry
        </Button>
      </AlertActions>
    </Alert>
  );
}

function SessionsBoardLanes({
  board,
  row,
  renderLane,
}: {
  board: SessionsBoardModel;
  row: SessionsBoardRow;
  renderLane: SessionsBoardViewProps["renderLane"];
}) {
  if (board.lanesError) return <SessionsBoardLanesFailure onRetry={board.lanesError.retry} />;
  if (board.lanesLoading) return <SessionsBoardLanesSkeleton />;
  if (board.lanes === null || board.lanes.length === 0) {
    return (
      <>
        <SessionsBoardLayoutWarning error={board.layoutError} />
        <p
          className="px-3 py-8 text-center text-small-body text-muted-foreground"
          data-testid="sessions-board-lanes-empty"
        >
          No sessions yet.
        </p>
      </>
    );
  }
  return (
    <div className={SESSIONS_BOARD_LANES} data-testid="sessions-board-lanes">
      <SessionsBoardLayoutWarning error={board.layoutError} />
      {board.lanes.map((lane, index) =>
        renderLane?.(lane, {
          ...row,
          selectionRow: { key: `lane:${lane.group.key}`, rank: index },
        })
      )}
    </div>
  );
}
