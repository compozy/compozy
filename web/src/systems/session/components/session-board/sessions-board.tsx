import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import type { SessionsBoardView as SessionsBoardModel } from "../../hooks/use-sessions-board";
import type { SessionBoardPayload } from "../../lib/session-board-contract";
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
 * The Sessions board (`_uiux.md` S4): four derived columns in a horizontally
 * scrolling row on the canvas, plus the Archive bar. Grouped by project or
 * profile, each group becomes a swimlane holding its own row of the same
 * columns. Placement is the daemon's (`board_column`); there is no drag and drop.
 * This container binds the live cards and lanes to `SessionsBoardView`.
 */
export function SessionsBoard(props: SessionsBoardProps) {
  return (
    <SessionBoardCardSlot value={SessionBoardCard}>
      <SessionsBoardView
        {...props}
        renderLane={(lane, row) => (
          <SessionsBoardSwimlane {...row} key={lane.group.key} lane={lane} />
        )}
      />
    </SessionBoardCardSlot>
  );
}
