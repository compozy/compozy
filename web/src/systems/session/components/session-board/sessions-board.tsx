import { useState } from "react";

import type { SessionsBoardView } from "../../hooks/use-sessions-board";
import type { SessionBoardPayload } from "../../lib/session-board-contract";
import { SessionsBoardArchiveBar } from "./sessions-board-archive-bar";
import { SessionsBoardColumn } from "./sessions-board-column";

export interface SessionsBoardProps {
  board: SessionsBoardView;
  currentSessionId?: string | null;
  onOpen: (session: SessionBoardPayload) => void;
}

/**
 * The Sessions board (`_uiux.md` S4): four derived columns in a horizontally
 * scrolling row on the canvas, plus the Archive bar. Placement is the daemon's
 * (`board_column`); there is no drag and drop.
 */
export function SessionsBoard({ board, currentSessionId = null, onOpen }: SessionsBoardProps) {
  // Done starts expanded (Open Question 1); the fold is local to this view.
  const [doneCollapsed, setDoneCollapsed] = useState(false);
  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col bg-canvas" data-testid="sessions-board">
      <div className="min-h-0 flex-1 overflow-auto p-4">
        <div className="flex min-h-full w-max items-stretch gap-3">
          {board.columns.map(column => (
            <SessionsBoardColumn
              collapsible={
                column.id === "done"
                  ? { collapsed: doneCollapsed, onToggle: () => setDoneCollapsed(value => !value) }
                  : undefined
              }
              column={column}
              currentSessionId={currentSessionId}
              key={column.id}
              onOpen={onOpen}
              workspaceNameOf={board.workspaceNameOf}
            />
          ))}
        </div>
      </div>
      <SessionsBoardArchiveBar
        archive={board.archive}
        onOpen={onOpen}
        workspaceNameOf={board.workspaceNameOf}
      />
    </div>
  );
}
