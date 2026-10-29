import { definePrototype, type PrototypeProps } from "#gallery/contract";

import type { SessionsBoardView } from "../../hooks/use-sessions-board";
import {
  SESSIONS_BOARD_COLUMN_IDS,
  boardRuntimeFixture,
  sessionsBoardViewFixture,
} from "../../mocks/sessions-board-view-fixtures";
import type { SessionBoardCardProps } from "./session-board-card";
import { SessionBoardCardSlot } from "./session-board-card-slot";
import { SessionBoardCardView } from "./session-board-card-view";
import { SessionsBoardSkeleton } from "./sessions-board-skeleton";
import { SessionsBoardView as SessionsBoardLayout } from "./sessions-board-view";

export const meta = definePrototype({
  title: "Sessions board",
  spec: "sessions-kanban",
  surface: "S4",
  route: "/sessions?view=board",
  states: [
    { id: "populated", label: "Populated" },
    { id: "open-session", label: "Open session highlighted" },
    { id: "empty", label: "Empty" },
    { id: "loading", label: "Loading columns" },
    { id: "skeleton", label: "Board skeleton" },
    { id: "column-error", label: "Column failed to load" },
    { id: "archive-error", label: "Archive failed to load" },
  ],
});

/** The card a live board would render, from fixture data (no runtime query, no preload). */
function PrototypeBoardCard({ actions: _actions, ...props }: SessionBoardCardProps) {
  return <SessionBoardCardView {...props} runtime={boardRuntimeFixture(props.session)} />;
}

const loadingColumn = { loading: true, sessions: [], count: undefined };
const emptyColumn = { sessions: [], count: 0, hasMore: false };

function boardOf(state: string): SessionsBoardView {
  switch (state) {
    case "empty":
      return sessionsBoardViewFixture(
        Object.fromEntries(SESSIONS_BOARD_COLUMN_IDS.map(id => [id, emptyColumn])),
        { count: 0, sessions: [] }
      );
    case "loading":
      return sessionsBoardViewFixture(
        Object.fromEntries(SESSIONS_BOARD_COLUMN_IDS.map(id => [id, loadingColumn])),
        { loading: true, count: undefined, sessions: [] }
      );
    case "column-error":
      return sessionsBoardViewFixture({ in_review: { failure: "initial", sessions: [] } });
    case "archive-error":
      return sessionsBoardViewFixture({}, { count: undefined, sessions: [], failure: "initial" });
    default:
      return sessionsBoardViewFixture();
  }
}

export default function SessionsBoardPrototype({ state }: PrototypeProps) {
  if (state === "skeleton") return <SessionsBoardSkeleton />;
  return (
    <SessionBoardCardSlot value={PrototypeBoardCard}>
      <SessionsBoardLayout
        board={boardOf(state)}
        currentSessionId={state === "open-session" ? "s4" : null}
        onOpen={() => undefined}
      />
    </SessionBoardCardSlot>
  );
}
